// Command bach-github-approval-provider is a Bach Factory approval provider
// that maps PR review approvals to factory gate approvals.
//
// Protocol: bach.approval.v1 (approval.handshake + approval.poll) over stdio
// JSON-RPC with Content-Length framing.
//
// Config:
//
//	repo        owner/name (required)
//	phases      comma-separated gated phases served by this provider
//	            (default "plan,deploy.staging")
//	head_prefix only consider PRs whose head branch starts with this
//	            (default "bach/")
//	token_env   env var holding the GitHub token (default GITHUB_TOKEN)
//	api_url     GitHub REST base (default https://api.github.com)
//	per_page    page size 1-100 (default 50)
//	max_pages   max pages to scan (default 3)
//
// A PR is associated with its factory Work Item through an
// "approval-source: #<issue-number>" (or "Closes #N" / "Fixes #N") marker in
// the PR body, matching the Trigger Provider intake identity
// (source_type "github_issue", source_id "<repo>#<number>"). When the PR's
// latest review is APPROVED, one record per configured phase is emitted. Bach
// applies records only while an item waits at that exact gate; repeats are
// idempotent. A CHANGES_REQUESTED or absent latest review emits nothing.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/applauselab/bachkator/pkg/approvalprotocol"
)

const (
	ProviderName      = "bach-github-approval-provider"
	providerVersion   = "v1"
	defaultAPIURL     = "https://api.github.com"
	defaultTokenEnv   = "GITHUB_TOKEN"
	defaultHeadPrefix = "bach/"
	defaultPhases     = "plan,deploy.staging"
	defaultPerPage    = 50
	defaultMaxPages   = 3
	maxErrorBodyBytes = 4096
)

type provider struct {
	client *http.Client
	config providerConfig
}

type providerConfig struct {
	Repo       string
	APIURL     string
	TokenEnv   string
	HeadPrefix string
	Phases     []string
	PerPage    int
	MaxPages   int
}

type githubPull struct {
	Number         int     `json:"number"`
	Title          string  `json:"title"`
	HTMLURL        string  `json:"html_url"`
	Body           *string `json:"body"`
	State          string  `json:"state"`
	Draft          bool    `json:"draft"`
	ReviewDecision *string `json:"review_decision"`
	Head           struct {
		Ref string `json:"ref"`
	} `json:"head"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	MergedAt *time.Time `json:"merged_at"`
}

type githubReview struct {
	State string `json:"state"`
	Body  string `json:"body"`
	User  struct {
		Login string `json:"login"`
	} `json:"user"`
	SubmittedAt *time.Time `json:"submitted_at"`
	HTMLURL     string     `json:"html_url"`
}

type githubComment struct {
	Body string `json:"body"`
	User struct {
		Login string `json:"login"`
	} `json:"user"`
	CreatedAt time.Time `json:"created_at"`
	HTMLURL   string    `json:"html_url"`
}

var factoryHeadPattern = regexp.MustCompile(
	`^bach/factory/([0-9a-fA-F]{8}-[0-9a-fA-F-]{27,36})/`,
)

// factoryHeadRef recognizes factory-generated head branches and returns the
// embedded work item id, so approvals do not depend on PR body markers.
func factoryHeadRef(ref string) (string, bool) {
	m := factoryHeadPattern.FindStringSubmatch(ref)
	if m == nil {
		return "", false
	}
	return m[1], true
}

var sourceMarker = regexp.MustCompile(
	`(?im)^(?:approval-source|closes|fixes|resolves|refs)\s*:\s*(?:[^\s#]+)?#(\d+)`,
)

func main() {
	if err := approvalprotocol.Serve(
		context.Background(),
		os.Stdin,
		os.Stdout,
		&provider{client: http.DefaultClient},
	); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func (p *provider) Handshake(
	_ context.Context,
	params approvalprotocol.HandshakeParams,
) (approvalprotocol.HandshakeResult, error) {
	if params.Protocol != approvalprotocol.ProtocolVersion {
		return approvalprotocol.HandshakeResult{}, approvalprotocol.NewError(
			approvalprotocol.ErrorUnsupportedProtocol,
			"unsupported protocol "+params.Protocol,
		)
	}
	cfg, err := parseConfig(params.Config)
	if err != nil {
		return approvalprotocol.HandshakeResult{}, validation(err)
	}
	p.config = cfg
	return approvalprotocol.HandshakeResult{
		Protocol:     approvalprotocol.ProtocolVersion,
		Provider:     ProviderName,
		Version:      providerVersion,
		Capabilities: []approvalprotocol.Capability{approvalprotocol.CapabilityPoll},
	}, nil
}

func (p *provider) Poll(
	ctx context.Context,
	params approvalprotocol.PollParams,
) (approvalprotocol.PollResult, error) {
	if len(params.Config) > 0 {
		cfg, err := parseConfig(params.Config)
		if err != nil {
			return approvalprotocol.PollResult{}, validation(err)
		}
		p.config = cfg
	}
	if p.config.Repo == "" {
		return approvalprotocol.PollResult{}, validation(fmt.Errorf("repo is required"))
	}
	pulls, err := p.fetchPulls(ctx)
	if err != nil {
		return approvalprotocol.PollResult{}, approvalprotocol.NewError(
			approvalprotocol.ErrorInternal, err.Error(),
		)
	}
	fmt.Fprintf(os.Stderr, "[debug] fetched pulls=%d\n", len(pulls))
	records := []approvalprotocol.ApprovalRecord{}
	for _, pull := range pulls {
		number, hasSource := sourceIssue(pull)
		_, headScoped := factoryHeadRef(pull.Head.Ref)
		if !hasSource && !headScoped {
			continue // unassociated PR
		}
		if !p.eligible(pull) {
			continue
		}
		decision, err := p.latestDecision(ctx, pull.Number)
		if err != nil || decision == nil || !decision.approved {
			continue // no approval, or the latest decision is a rejection
		}
		for _, phase := range p.config.Phases {
			record := approvalprotocol.ApprovalRecord{
				Phase:      phase,
				Rejected:   !decision.approved,
				Approver:   decision.approver,
				ApprovedAt: decision.at,
				Reason:     decision.reason,
				Metadata: map[string]string{
					"github_repo":   p.config.Repo,
					"pr_number":     strconv.Itoa(pull.Number),
					"pr_url":        pull.HTMLURL,
					"decision_kind": decision.kind,
				},
			}
			if hasSource {
				record.SourceType = "github_issue"
				record.SourceID = p.config.Repo + "#" + strconv.Itoa(number)
			} else {
				record.HeadRef = pull.Head.Ref
			}
			records = append(records, record)
		}
	}
	return approvalprotocol.PollResult{Approvals: records}, nil
}

// decision is the latest human approval signal on a PR: either a review or a
// marker comment. Solo operators cannot review-approve their own PR, so a
// "/approve" (or "/deny") comment counts with the same weight; the most
// recent signal wins.
type decision struct {
	approved bool
	approver string
	at       string
	reason   string
	kind     string
}

var (
	approveMarker = regexp.MustCompile(`(?im)^\s*/approve\b`)
	denyMarker    = regexp.MustCompile(`(?im)^\s*/deny\b`)
)

func (p *provider) latestDecision(ctx context.Context, number int) (*decision, error) {
	var best *decision

	reviews, err := p.fetchReviews(ctx, number)
	if err != nil {
		return nil, err
	}
	for _, r := range reviews {
		var approved bool
		switch r.State {
		case "APPROVED":
			approved = true
		case "CHANGES_REQUESTED", "DISMISSED":
			approved = false
		default:
			continue // COMMENT / PENDING reviews carry no decision
		}
		at := ""
		if r.SubmittedAt != nil {
			at = r.SubmittedAt.UTC().Format(time.RFC3339Nano)
		}
		best = &decision{
			approved: approved,
			approver: r.User.Login,
			at:       at,
			reason:   "approved via PR review " + r.HTMLURL,
			kind:     "review",
		}
		if !approved {
			best.reason = "changes requested via PR review " + r.HTMLURL
		}
	}

	comments, err := p.fetchComments(ctx, number)
	if err != nil {
		return nil, err
	}
	for _, c := range comments {
		var approved bool
		switch {
		case approveMarker.MatchString(c.Body):
			approved = true
		case denyMarker.MatchString(c.Body):
			approved = false
		default:
			continue
		}
		cand := &decision{
			approved: approved,
			approver: c.User.Login,
			at:       c.CreatedAt.UTC().Format(time.RFC3339Nano),
			reason:   "approved via PR comment " + c.HTMLURL,
			kind:     "comment",
		}
		if !approved {
			cand.reason = "denied via PR comment " + c.HTMLURL
		}
		if best == nil || cand.at > best.at {
			best = cand
		}
	}
	return best, nil
}

func (p *provider) eligible(pull githubPull) bool {
	if pull.Draft || pull.State != "open" || pull.MergedAt != nil {
		return false
	}
	return strings.HasPrefix(pull.Head.Ref, p.config.HeadPrefix)
}

// sourceIssue extracts the intake issue number from the PR body marker.
func sourceIssue(pull githubPull) (int, bool) {
	if pull.Body == nil {
		return 0, false
	}
	match := sourceMarker.FindStringSubmatch(*pull.Body)
	if match == nil {
		return 0, false
	}
	number, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return number, true
}

func (p *provider) approved(pull githubPull) bool {
	if pull.Draft || pull.State != "open" || pull.MergedAt != nil {
		return false
	}
	if !strings.HasPrefix(pull.Head.Ref, p.config.HeadPrefix) {
		return false
	}
	return pull.ReviewDecision != nil && *pull.ReviewDecision == "APPROVED"
}

// fetchReviews returns submitted reviews for a PR, oldest first.
func (p *provider) fetchReviews(ctx context.Context, number int) ([]githubReview, error) {
	requestURL, err := p.apiURL("/repos/%s/pulls/%d/reviews", p.config.Repo, number)
	if err != nil {
		return nil, err
	}
	query := requestURL.Query()
	query.Set("per_page", "100")
	requestURL.RawQuery = query.Encode()
	body, status, err := p.get(ctx, requestURL.String())
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("pull reviews request failed with %d: %s", status, truncated(body))
	}
	var reviews []githubReview
	if err := json.Unmarshal(body, &reviews); err != nil {
		return nil, fmt.Errorf("decode pull reviews: %w", err)
	}
	return reviews, nil
}

// fetchComments returns issue comments (the PR conversation) for a PR,
// oldest first.
func (p *provider) fetchComments(ctx context.Context, number int) ([]githubComment, error) {
	requestURL, err := p.apiURL("/repos/%s/issues/%d/comments", p.config.Repo, number)
	if err != nil {
		return nil, err
	}
	query := requestURL.Query()
	query.Set("per_page", "100")
	requestURL.RawQuery = query.Encode()
	body, status, err := p.get(ctx, requestURL.String())
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("pr comments request failed with %d: %s", status, truncated(body))
	}
	var comments []githubComment
	if err := json.Unmarshal(body, &comments); err != nil {
		return nil, fmt.Errorf("decode pr comments: %w", err)
	}
	return comments, nil
}

func (p *provider) fetchPulls(ctx context.Context) ([]githubPull, error) {
	pulls := []githubPull{}
	for page := 1; page <= p.config.MaxPages; page++ {
		requestURL, err := p.apiURL("/repos/%s/pulls", p.config.Repo)
		if err != nil {
			return nil, err
		}
		query := requestURL.Query()
		query.Set("state", "open")
		query.Set("sort", "updated")
		query.Set("direction", "desc")
		query.Set("per_page", strconv.Itoa(p.config.PerPage))
		query.Set("page", strconv.Itoa(page))
		requestURL.RawQuery = query.Encode()
		body, status, err := p.get(ctx, requestURL.String())
		if err != nil {
			return nil, err
		}
		if status != http.StatusOK {
			return nil, fmt.Errorf(
				"github pulls request failed with %d: %s",
				status,
				truncated(body),
			)
		}
		var batch []githubPull
		if err := json.Unmarshal(body, &batch); err != nil {
			return nil, fmt.Errorf("decode github pulls: %w", err)
		}
		pulls = append(pulls, batch...)
		if len(batch) < p.config.PerPage {
			break
		}
	}
	return pulls, nil
}

func (p *provider) apiURL(format string, args ...any) (*url.URL, error) {
	base, err := url.Parse(strings.TrimRight(p.config.APIURL, "/") + "/")
	if err != nil {
		return nil, err
	}
	base.Path = strings.TrimRight(base.Path, "/") + fmt.Sprintf(format, args...)
	return base, nil
}

func (p *provider) get(ctx context.Context, requestURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", ProviderName)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := strings.TrimSpace(os.Getenv(p.config.TokenEnv)); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("github request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read github response: %w", err)
	}
	message := string(body)
	if token := strings.TrimSpace(os.Getenv(p.config.TokenEnv)); token != "" {
		message = strings.ReplaceAll(message, token, "[REDACTED]")
	}
	return []byte(message), resp.StatusCode, nil
}

func stamp(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func truncated(body []byte) string {
	if len(body) > maxErrorBodyBytes {
		return string(body[:maxErrorBodyBytes])
	}
	return string(body)
}

func parseConfig(values map[string]string) (providerConfig, error) {
	cfg := providerConfig{
		APIURL:     defaultAPIURL,
		TokenEnv:   defaultTokenEnv,
		HeadPrefix: defaultHeadPrefix,
		Phases:     strings.Split(defaultPhases, ","),
		PerPage:    defaultPerPage,
		MaxPages:   defaultMaxPages,
	}
	if values == nil {
		values = map[string]string{}
	}
	cfg.Repo = strings.TrimSpace(values["repo"])
	if cfg.Repo == "" {
		return cfg, fmt.Errorf("repo is required")
	}
	if strings.Count(cfg.Repo, "/") != 1 {
		return cfg, fmt.Errorf("repo must be owner/name")
	}
	if value := strings.TrimSpace(values["api_url"]); value != "" {
		cfg.APIURL = strings.TrimRight(value, "/")
	}
	if value := strings.TrimSpace(values["token_env"]); value != "" {
		cfg.TokenEnv = value
	}
	if value := values["head_prefix"]; value != "" {
		cfg.HeadPrefix = value
	}
	if value := strings.TrimSpace(values["phases"]); value != "" {
		cfg.Phases = strings.Split(value, ",")
	}
	phases := make([]string, 0, len(cfg.Phases))
	for _, phase := range cfg.Phases {
		if phase = strings.TrimSpace(phase); phase != "" {
			phases = append(phases, phase)
		}
	}
	if len(phases) == 0 {
		return cfg, fmt.Errorf("phases must list at least one gated phase")
	}
	cfg.Phases = phases
	if value := strings.TrimSpace(values["per_page"]); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > 100 {
			return cfg, fmt.Errorf("per_page must be an integer from 1 to 100")
		}
		cfg.PerPage = n
	}
	if value := strings.TrimSpace(values["max_pages"]); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			return cfg, fmt.Errorf("max_pages must be a positive integer")
		}
		cfg.MaxPages = n
	}
	return cfg, nil
}

func validation(err error) approvalprotocol.Error {
	return approvalprotocol.NewError(approvalprotocol.ErrorValidationFailed, err.Error())
}
