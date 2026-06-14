// Command github-approvals is an example Bach Approval Provider that maps
// GitHub issue labels to Factory approvals.
//
// An open issue labeled `approved/plan` approves the `plan` phase of every
// waiting Work Item whose intake source is github_issue/<issue-number>.
// `approved/deploy.production` works the same way for deploy gates. The
// approver is resolved from the most recent labeled event on the issue.
//
// Requires the `gh` CLI authenticated for the target repository.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/applauselab/bachkator/pkg/approvalprotocol"
)

const sourceTypeGitHubIssue = "github_issue"

type provider struct {
	repo        string
	labelPrefix string
}

func main() {
	if err := approvalprotocol.Serve(
		context.Background(),
		os.Stdin,
		os.Stdout,
		newProvider(),
	); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newProvider() *provider {
	return &provider{
		labelPrefix: "approved/",
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
	p.repo = params.Config["repo"]
	p.labelPrefix = params.Config["label_prefix"]
	if p.labelPrefix == "" {
		p.labelPrefix = "approved/"
	}
	return approvalprotocol.HandshakeResult{
		Protocol:     approvalprotocol.ProtocolVersion,
		Provider:     "github-approvals-example",
		Version:      "v1",
		Capabilities: []approvalprotocol.Capability{approvalprotocol.CapabilityPoll},
	}, nil
}

func (p *provider) Poll(
	_ context.Context,
	_ approvalprotocol.PollParams,
) (approvalprotocol.PollResult, error) {
	if p.repo == "" {
		return approvalprotocol.PollResult{}, approvalprotocol.NewError(
			approvalprotocol.ErrorValidationFailed,
			"config.repo is required",
		)
	}
	issues, err := p.openIssues()
	if err != nil {
		return approvalprotocol.PollResult{}, err
	}
	var records []approvalprotocol.ApprovalRecord
	for _, issue := range issues {
		for _, label := range issue.Labels {
			if !strings.HasPrefix(label.Name, p.labelPrefix) {
				continue
			}
			record := approvalprotocol.ApprovalRecord{
				SourceType: sourceTypeGitHubIssue,
				SourceID:   fmt.Sprintf("%d", issue.Number),
				Phase:      strings.TrimPrefix(label.Name, p.labelPrefix),
				Metadata: map[string]string{
					"issue_url": issue.HTMLURL,
					"title":     issue.Title,
				},
			}
			approver := p.lastLabelActor(issue.Number, label.Name)
			if approver != "" {
				record.Approver = approver
			}
			records = append(records, record)
		}
	}
	return approvalprotocol.PollResult{Approvals: records}, nil
}

type ghIssue struct {
	Number  int       `json:"number"`
	Title   string    `json:"title"`
	HTMLURL string    `json:"html_url"`
	Labels  []ghLabel `json:"labels"`
	Pull    *bool     `json:"pull_request"`
}

type ghLabel struct {
	Name string `json:"name"`
}

func (p *provider) openIssues() ([]ghIssue, error) {
	output, err := gh(
		"api",
		fmt.Sprintf("repos/%s/issues", p.repo),
		"-X", "GET",
		"-f", "state=open",
		"-F", "per_page=100",
	)
	if err != nil {
		return nil, approvalprotocol.NewError(
			approvalprotocol.ErrorInternal,
			"list issues: "+err.Error(),
		)
	}
	var issues []ghIssue
	if err := json.Unmarshal(output, &issues); err != nil {
		return nil, approvalprotocol.NewError(
			approvalprotocol.ErrorInternal,
			"decode issues: "+err.Error(),
		)
	}
	filtered := make([]ghIssue, 0, len(issues))
	for _, issue := range issues {
		if issue.Pull == nil {
			filtered = append(filtered, issue)
		}
	}
	return filtered, nil
}

type ghEvent struct {
	Event string   `json:"event"`
	Actor *ghUser  `json:"actor"`
	Label *ghLabel `json:"label"`
}

type ghUser struct {
	Login string `json:"login"`
}

// lastLabelActor returns the GitHub login that most recently applied the
// given label to the issue, or "" when no labeled event exists.
func (p *provider) lastLabelActor(issueNumber int, label string) string {
	output, err := gh(
		"api",
		fmt.Sprintf("repos/%s/issues/%d/events", p.repo, issueNumber),
		"-F", "per_page=100",
	)
	if err != nil {
		return ""
	}
	var events []ghEvent
	if err := json.Unmarshal(output, &events); err != nil {
		return ""
	}
	actor := ""
	for _, event := range events {
		if event.Event != "labeled" || event.Label == nil || event.Actor == nil {
			continue
		}
		if event.Label.Name == label && event.Actor.Login != "" {
			actor = event.Actor.Login
		}
	}
	return actor
}

func gh(args ...string) ([]byte, error) {
	command := exec.Command("gh", args...)
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("gh %s: %w", args[0], err)
	}
	return output, nil
}
