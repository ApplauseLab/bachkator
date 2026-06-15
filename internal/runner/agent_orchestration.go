package runner

import (
	"context"
	"fmt"
	"path/filepath"

	targetpkg "github.com/applauselab/bachkator/internal/target"
)

func (r *Runner) runAgentPolicyTarget(
	ctx context.Context,
	s *Session,
	req targetpkg.PolicyTargetRequest,
) error {
	if req.Target == "" {
		return fmt.Errorf("policy target is empty")
	}
	policyRunner := Runner{
		Force:       true,
		Yes:         r.Yes,
		EnvFile:     r.EnvFile,
		LogOnly:     r.LogOnly,
		Verbose:     r.Verbose,
		Parallelism: r.Parallelism,
		Stdout:      r.Stdout,
		Stderr:      r.Stderr,
		Targets:     s.targets,
		Parsers:     r.reportParsers(),
		Gates:       r.gateEvaluators(),
	}
	return policyRunner.RunTargets(ctx, s.project, []string{req.Target})
}

func (r *Runner) runAgentRequiredTargets(
	ctx context.Context,
	s *Session,
	req targetpkg.RequiredTargetsRequest,
) error {
	subjectProject := *s.project
	subjectProject.Root = req.WorkDir
	subjectProject.StatePath = filepath.Join(req.WorkDir, ".bach", "state.db")
	subjectProject.Env = append(
		append([]string(nil), subjectProject.Env...),
		"BACH_POLICY_SUBJECT="+req.Subject,
		"BACH_POLICY_SUBJECT_COMMIT="+req.SubjectCommit,
		"BACH_POLICY_NODE="+req.PolicyNode,
	)
	policyRunner := Runner{
		Force:       r.Force,
		Yes:         r.Yes,
		EnvFile:     r.EnvFile,
		LogOnly:     r.LogOnly,
		Verbose:     r.Verbose,
		Parallelism: r.Parallelism,
		Stdout:      r.Stdout,
		Stderr:      r.Stderr,
		Targets:     s.targets,
		Parsers:     r.reportParsers(),
		Gates:       r.gateEvaluators(),
	}
	return policyRunner.RunTargets(ctx, &subjectProject, req.Targets)
}
