package githubissuetrigger

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/applauselab/bachkator/pkg/triggerprotocol"
)

const (
	ProviderName              = "bach-github-issue-trigger"
	providerVersion           = "v1"
	defaultAPIURL             = "https://api.github.com"
	defaultTokenEnv           = "GITHUB_TOKEN"
	defaultState              = "open"
	defaultPerPage            = 100
	defaultMaxPages           = 5
	defaultPriorityLabelPrefx = "priority:"
	maxErrorBodyBytes         = 4096
)

type Provider struct {
	client *http.Client
	config providerConfig
}

type providerConfig struct {
	Repo                string
	APIURL              string
	TokenEnv            string
	State               string
	Labels              string
	PerPage             int
	MaxPages            int
	Since               string
	PriorityLabelPrefix string
}

type githubIssue struct {
	Number      int           `json:"number"`
	HTMLURL     string        `json:"html_url"`
	Title       string        `json:"title"`
	Body        *string       `json:"body"`
	State       string        `json:"state"`
	Labels      []githubLabel `json:"labels"`
	User        githubUser    `json:"user"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	PullRequest *struct{}     `json:"pull_request"`
}

type githubLabel struct {
	Name string `json:"name"`
}

type githubUser struct {
	Login string `json:"login"`
}

func New(client *http.Client) *Provider {
	if client == nil {
		client = http.DefaultClient
	}
	return &Provider{client: client}
}

func (p *Provider) Handshake(
	_ context.Context,
	params triggerprotocol.HandshakeParams,
) (triggerprotocol.HandshakeResult, error) {
	if params.Protocol != triggerprotocol.ProtocolVersion {
		return triggerprotocol.HandshakeResult{}, triggerprotocol.NewError(
			triggerprotocol.ErrorUnsupportedProtocol,
			"unsupported protocol "+params.Protocol,
		)
	}
	cfg, err := parseConfig(params.Config)
	if err != nil {
		return triggerprotocol.HandshakeResult{}, triggerError(err)
	}
	p.config = cfg
	return triggerprotocol.HandshakeResult{
		Protocol: triggerprotocol.ProtocolVersion,
		Provider: ProviderName,
		Version:  providerVersion,
		Capabilities: []triggerprotocol.Capability{
			triggerprotocol.CapabilityPoll,
			triggerprotocol.CapabilityAck,
			triggerprotocol.CapabilityNack,
		},
	}, nil
}

func (p *Provider) Poll(
	ctx context.Context,
	params triggerprotocol.PollParams,
) (triggerprotocol.PollResult, error) {
	cfg := p.config
	if len(params.Config) > 0 {
		parsed, err := parseConfig(params.Config)
		if err != nil {
			return triggerprotocol.PollResult{}, triggerError(err)
		}
		cfg = parsed
		p.config = parsed
	}
	if cfg.Repo == "" {
		return triggerprotocol.PollResult{}, triggerError(fmt.Errorf("repo is required"))
	}
	cursor := strings.TrimSpace(params.Cursor)
	if cursor == "" {
		cursor = cfg.Since
	}
	if cursor != "" {
		if _, err := parseTime(cursor); err != nil {
			return triggerprotocol.PollResult{}, triggerError(
				fmt.Errorf("cursor is not RFC3339: %w", err),
			)
		}
	}
	issues, nextCursor, err := p.fetchIssues(ctx, cfg, cursor)
	if err != nil {
		return triggerprotocol.PollResult{}, triggerprotocol.NewError(
			triggerprotocol.ErrorInternal,
			err.Error(),
		)
	}
	items := make([]triggerprotocol.PollItem, 0, len(issues))
	for _, issue := range issues {
		if issue.PullRequest != nil {
			continue
		}
		items = append(items, mapIssue(cfg, issue))
	}
	return triggerprotocol.PollResult{Cursor: nextCursor, Items: items}, nil
}

func (p *Provider) Ack(_ context.Context, _ triggerprotocol.AckParams) error {
	return nil
}

func (p *Provider) Nack(_ context.Context, _ triggerprotocol.NackParams) error {
	return nil
}

func (p *Provider) fetchIssues(
	ctx context.Context,
	cfg providerConfig,
	cursor string,
) ([]githubIssue, string, error) {
	issues := []githubIssue{}
	nextCursor := cursor
	for page := 1; page <= cfg.MaxPages; page++ {
		batch, err := p.fetchIssuePage(ctx, cfg, cursor, page)
		if err != nil {
			return nil, "", err
		}
		for _, issue := range batch {
			issues = append(issues, issue)
			if issue.UpdatedAt.IsZero() {
				continue
			}
			updated := issue.UpdatedAt.UTC().Format(time.RFC3339Nano)
			if nextCursor == "" || compareTimes(updated, nextCursor) > 0 {
				nextCursor = updated
			}
		}
		if len(batch) < cfg.PerPage {
			break
		}
	}
	return issues, nextCursor, nil
}

func (p *Provider) fetchIssuePage(
	ctx context.Context,
	cfg providerConfig,
	cursor string,
	page int,
) ([]githubIssue, error) {
	requestURL, err := issueListURL(cfg, cursor, page)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", ProviderName)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token := strings.TrimSpace(os.Getenv(cfg.TokenEnv)); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github issues request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes))
		return nil, fmt.Errorf(
			"github issues request failed with %s: %s",
			resp.Status,
			strings.TrimSpace(string(body)),
		)
	}
	var issues []githubIssue
	if err := json.NewDecoder(resp.Body).Decode(&issues); err != nil {
		return nil, fmt.Errorf("decode github issues: %w", err)
	}
	return issues, nil
}

func parseConfig(values map[string]string) (providerConfig, error) {
	cfg := providerConfig{
		APIURL:              defaultAPIURL,
		TokenEnv:            defaultTokenEnv,
		State:               defaultState,
		PerPage:             defaultPerPage,
		MaxPages:            defaultMaxPages,
		PriorityLabelPrefix: defaultPriorityLabelPrefx,
	}
	if values == nil {
		values = map[string]string{}
	}
	cfg.Repo = strings.TrimSpace(values["repo"])
	if cfg.Repo == "" {
		return providerConfig{}, fmt.Errorf("repo is required")
	}
	if strings.Count(cfg.Repo, "/") != 1 {
		return providerConfig{}, fmt.Errorf("repo must be owner/name")
	}
	if value := strings.TrimSpace(values["api_url"]); value != "" {
		cfg.APIURL = strings.TrimRight(value, "/")
	}
	if _, err := url.ParseRequestURI(cfg.APIURL); err != nil {
		return providerConfig{}, fmt.Errorf("api_url is invalid: %w", err)
	}
	if value := strings.TrimSpace(values["token_env"]); value != "" {
		cfg.TokenEnv = value
	}
	if value := strings.TrimSpace(values["state"]); value != "" {
		cfg.State = value
	}
	if cfg.State != "open" && cfg.State != "closed" && cfg.State != "all" {
		return providerConfig{}, fmt.Errorf("state must be open, closed, or all")
	}
	cfg.Labels = strings.TrimSpace(values["labels"])
	if value := strings.TrimSpace(values["per_page"]); value != "" {
		perPage, err := strconv.Atoi(value)
		if err != nil || perPage < 1 || perPage > 100 {
			return providerConfig{}, fmt.Errorf("per_page must be an integer from 1 to 100")
		}
		cfg.PerPage = perPage
	}
	if value := strings.TrimSpace(values["max_pages"]); value != "" {
		maxPages, err := strconv.Atoi(value)
		if err != nil || maxPages < 1 {
			return providerConfig{}, fmt.Errorf("max_pages must be a positive integer")
		}
		cfg.MaxPages = maxPages
	}
	if value := strings.TrimSpace(values["since"]); value != "" {
		if _, err := parseTime(value); err != nil {
			return providerConfig{}, fmt.Errorf("since is not RFC3339: %w", err)
		}
		cfg.Since = value
	}
	if value := values["priority_label_prefix"]; value != "" {
		cfg.PriorityLabelPrefix = value
	}
	return cfg, nil
}

func issueListURL(cfg providerConfig, cursor string, page int) (string, error) {
	parts := strings.Split(cfg.Repo, "/")
	base, err := url.Parse(strings.TrimRight(cfg.APIURL, "/") + "/")
	if err != nil {
		return "", err
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/repos/" + url.PathEscape(parts[0]) + "/" +
		url.PathEscape(parts[1]) + "/issues"
	query := base.Query()
	query.Set("state", cfg.State)
	query.Set("sort", "updated")
	query.Set("direction", "asc")
	query.Set("per_page", strconv.Itoa(cfg.PerPage))
	query.Set("page", strconv.Itoa(page))
	if cursor != "" {
		query.Set("since", cursor)
	}
	if cfg.Labels != "" {
		query.Set("labels", cfg.Labels)
	}
	base.RawQuery = query.Encode()
	return base.String(), nil
}

func mapIssue(cfg providerConfig, issue githubIssue) triggerprotocol.PollItem {
	labels := issueLabels(issue.Labels)
	body := ""
	if issue.Body != nil {
		body = *issue.Body
	}
	metadata := map[string]string{
		"github_repo":       cfg.Repo,
		"github_number":     strconv.Itoa(issue.Number),
		"github_state":      issue.State,
		"github_author":     issue.User.Login,
		"github_created_at": issue.CreatedAt.UTC().Format(time.RFC3339Nano),
		"github_updated_at": issue.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	return triggerprotocol.PollItem{
		Source: triggerprotocol.ItemSource{
			Type:     "github_issue",
			ID:       cfg.Repo + "#" + strconv.Itoa(issue.Number),
			URL:      issue.HTMLURL,
			Revision: issue.UpdatedAt.UTC().Format(time.RFC3339Nano),
		},
		Title:    issue.Title,
		Body:     body,
		Labels:   labels,
		Priority: priorityFromLabels(labels, cfg.PriorityLabelPrefix),
		Metadata: metadata,
	}
}

func issueLabels(labels []githubLabel) []string {
	result := make([]string, 0, len(labels))
	for _, label := range labels {
		name := strings.TrimSpace(label.Name)
		if name != "" {
			result = append(result, name)
		}
	}
	return result
}

func priorityFromLabels(labels []string, prefix string) string {
	valid := map[string]struct{}{
		"critical": {},
		"urgent":   {},
		"high":     {},
		"normal":   {},
		"low":      {},
	}
	for _, label := range labels {
		candidate := strings.ToLower(strings.TrimSpace(label))
		if prefix != "" && strings.HasPrefix(candidate, strings.ToLower(prefix)) {
			candidate = strings.TrimSpace(strings.TrimPrefix(candidate, strings.ToLower(prefix)))
		}
		if _, ok := valid[candidate]; ok {
			return candidate
		}
	}
	return ""
}

func triggerError(err error) error {
	return triggerprotocol.NewError(triggerprotocol.ErrorValidationFailed, err.Error())
}

func parseTime(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func compareTimes(left, right string) int {
	leftTime, leftErr := parseTime(left)
	rightTime, rightErr := parseTime(right)
	if leftErr != nil || rightErr != nil {
		return strings.Compare(left, right)
	}
	if leftTime.After(rightTime) {
		return 1
	}
	if leftTime.Before(rightTime) {
		return -1
	}
	return 0
}
