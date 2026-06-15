package target

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/applauselab/bachkator/internal/evidence"
	gitpkg "github.com/applauselab/bachkator/internal/git"
	"github.com/applauselab/bachkator/internal/model"
)

type policyHandler struct{}

func (policyHandler) Type() model.TargetType { return model.TargetTypePolicy }

func (policyHandler) Runnable(spec model.TargetSpec) bool {
	_, ok := spec.Body.(model.PolicySpec)
	return ok
}

func (policyHandler) Describe(_ context.Context, req DescribeRequest) (RunDescription, error) {
	body, ok := req.Spec.Body.(model.PolicySpec)
	if !ok {
		return RunDescription{}, fmt.Errorf(
			"target %q has %s body, want policy",
			req.Spec.Name,
			req.Spec.TargetType(),
		)
	}
	return RunDescription{
		Operation: fmt.Sprintf(
			"generated policy fan-out subject=%s required_targets=%s reviewers=%s",
			body.Subject.Target,
			strings.Join(body.Policy.RequiredTargets, ","),
			strings.Join(body.Policy.Reviewers, ","),
		),
	}, nil
}

func (policyHandler) Execute(ctx context.Context, req ExecuteRequest) error {
	body, ok := req.Spec.Body.(model.PolicySpec)
	if !ok {
		return fmt.Errorf("target %q has non-policy body", req.Spec.Name)
	}
	runDirectory := req.Env["BACH_RUN_DIRECTORY"]
	if runDirectory == "" {
		runDirectory = req.Env["RUN_DIRECTORY"]
	}
	if runDirectory == "" {
		return fmt.Errorf("target %q missing run directory", req.Spec.Name)
	}
	workspace, err := evidence.ResolveWorkspace(req.WorkDir, body.Subject.Workspace)
	if err != nil {
		return err
	}
	commit, err := gitpkg.Head(ctx, workspace)
	if err != nil {
		return fmt.Errorf("policy subject commit: %w", err)
	}
	subject := body.Subject
	subject.Workspace = workspace
	subject.Commit = commit
	agent := model.AgentSpec{Policy: body.Policy}
	if len(body.Policy.RequiredTargets) > 0 {
		if err := runAttachedPolicyRequiredTargets(
			ctx,
			req,
			agent,
			subject,
			runDirectory,
			runDirectory,
		); err != nil {
			return err
		}
	}
	if len(body.Policy.ReviewerSpecs) > 0 {
		reviewDir := filepath.Join(runDirectory, "reviews")
		if err := os.MkdirAll(reviewDir, 0o755); err != nil {
			return err
		}
		return runReviewPolicy(
			ctx,
			agent,
			subject,
			req,
			workspace,
			runDirectory,
			reviewDir,
		)
	}
	if body.Policy.Name != "" {
		return writePassingPolicyReport(
			runDirectory,
			runDirectory,
			subject,
			body.Policy.Gates,
			req.now(),
		)
	}
	return nil
}

func (policyHandler) FingerprintParts(body model.TargetBody) map[string]string {
	policy, _ := body.(model.PolicySpec)
	return map[string]string{
		"policy":            policy.Policy.Name,
		"subject":           policy.Subject.Target,
		"subject_workspace": policy.Subject.Workspace,
		"required_targets":  strings.Join(policy.Policy.RequiredTargets, "\x00"),
		"reviewers":         strings.Join(policy.Policy.Reviewers, "\x00"),
	}
}

func (policyHandler) CompositeChildren(model.TargetBody) []CompositeChild { return nil }
