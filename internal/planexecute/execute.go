package planexecute

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/id"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/plan"
	"github.com/applauselab/bachkator/internal/planstatus"
	"github.com/applauselab/bachkator/internal/quality"
	"github.com/applauselab/bachkator/internal/runner"
	"github.com/applauselab/bachkator/internal/target"
)

const ResultImplemented = "implemented"
const ResultSkipped = "skipped"
const ResultFailed = "failed"

type Options struct {
	Path        string
	DryRun      bool
	Force       bool
	Yes         bool
	EnvFile     string
	LogOnly     bool
	Verbose     bool
	Parallelism int
	Template    string
}

type Result struct {
	Plan        plan.Document
	Result      string
	Target      string
	Template    string
	RunID       string
	Ledger      *plan.LedgerSummary
	Written     []plan.LedgerSummary
	Diagnostics []plan.Diagnostic
}

type Service struct {
	Project *model.RunProject
	Backend *backend.Client
	Targets runner.TargetHandlers
	Parsers quality.ReportParsers
	Gates   quality.GateEvaluators
	Stdout  interface{ Write([]byte) (int, error) }
	Stderr  interface{ Write([]byte) (int, error) }
	Now     clock.NowFunc
}

func (s Service) Implement(ctx context.Context, opts Options) (Result, error) {
	status, err := planstatus.Status(
		ctx,
		s.Project,
		s.Backend.Plans,
		planstatus.Options{Paths: []string{opts.Path}},
	)
	if err != nil {
		return Result{}, err
	}
	if len(status.Records) != 1 {
		return Result{}, fmt.Errorf("expected exactly one Plan")
	}
	record := status.Records[0]
	result := Result{
		Plan:        record.Document,
		Target:      generatedTargetName(record.Document),
		Diagnostics: status.Diagnostics,
	}
	if record.Ledger != nil {
		result.Ledger = record.Ledger
	}
	if len(record.Document.DependsOn) > 0 {
		result.Diagnostics = filterExternalDependencyDiagnostics(
			result.Diagnostics,
			record.Document,
		)
		depDiagnostics, err := s.externalDependencyDiagnostics(ctx, record.Document)
		if err != nil {
			return result, err
		}
		result.Diagnostics = append(result.Diagnostics, depDiagnostics...)
	}
	if hasErrorDiagnostics(result.Diagnostics) {
		return result, fmt.Errorf("plan has validation errors")
	}
	if record.Status == plan.StatusImplemented && !opts.Force {
		result.Result = ResultSkipped
		return result, nil
	}
	if record.Document.AgentTemplate == "" {
		if opts.Template != "" {
			record.Document.AgentTemplate = opts.Template
			result.Plan = record.Document
		} else {
			result.Diagnostics = append(
				result.Diagnostics,
				diagnostic(
					record.Document,
					"missing-agent-template",
					"Plan execution requires agent_template metadata",
				),
			)
			return result, fmt.Errorf("plan execution requires agent_template metadata")
		}
	} else if opts.Template != "" {
		record.Document.AgentTemplate = opts.Template
		result.Plan = record.Document
	}
	project, targetName, templateName, err := materializeProject(s.Project, record.Document)
	if err != nil {
		result.Diagnostics = append(
			result.Diagnostics,
			diagnostic(record.Document, "generated-target-error", err.Error()),
		)
		return result, err
	}
	result.Target = targetName
	result.Template = templateName
	if opts.DryRun {
		return result, runnerFor(s, opts, true).RunTargets(ctx, project, []string{targetName})
	}
	if pending, err := s.recordLedger(
		ctx,
		record.Document,
		plan.StatusPending,
		targetName,
		"",
	); err != nil {
		return result, err
	} else {
		result.Written = append(result.Written, pending)
	}
	if inProgress, err := s.recordLedger(
		ctx,
		record.Document,
		plan.StatusInProgress,
		targetName,
		"",
	); err != nil {
		return result, err
	} else {
		result.Written = append(result.Written, inProgress)
	}
	runErr := runnerFor(s, opts, false).RunTargets(ctx, project, []string{targetName})
	runID, runIDErr := latestRunID(ctx, s.Backend, targetName)
	result.RunID = runID
	terminal := plan.StatusImplemented
	result.Result = ResultImplemented
	if runErr != nil {
		terminal = plan.StatusFailed
		result.Result = ResultFailed
	} else if runIDErr != nil {
		return result, runIDErr
	}
	ledger, ledgerErr := s.recordLedger(ctx, record.Document, terminal, targetName, runID)
	if ledgerErr != nil {
		return result, ledgerErr
	}
	result.Written = append(result.Written, ledger)
	result.Ledger = &ledger
	if runErr != nil {
		return result, runErr
	}
	return result, nil
}

func runnerFor(s Service, opts Options, dryRun bool) *runner.Runner {
	r := runner.Runner{
		DryRun:      dryRun,
		Force:       opts.Force,
		Yes:         opts.Yes,
		EnvFile:     opts.EnvFile,
		LogOnly:     opts.LogOnly,
		Verbose:     opts.Verbose,
		Parallelism: opts.Parallelism,
		Stdout:      s.Stdout,
		Stderr:      s.Stderr,
		Targets:     s.Targets,
		Parsers:     s.Parsers,
		Gates:       s.Gates,
		Now:         s.Now,
	}
	return &r
}

func (s Service) recordLedger(
	ctx context.Context,
	doc plan.Document,
	status string,
	targetName string,
	runID string,
) (plan.LedgerSummary, error) {
	now := s.now()
	ledgerID, err := id.New()
	if err != nil {
		return plan.LedgerSummary{}, err
	}
	evidenceID, err := id.New()
	if err != nil {
		return plan.LedgerSummary{}, err
	}
	metadata := map[string]string{
		"plan_path":        doc.Path,
		"generated_target": targetName,
	}
	if runID != "" {
		metadata["run_id"] = runID
	}
	ledger := backend.PlanLedger{
		SchemaVersion: plan.LedgerSchemaVersion,
		LedgerID:      ledgerID,
		PlanID:        doc.ID,
		Status:        status,
		Hash:          doc.Hash,
		RunID:         runID,
		RecordedAt:    now,
		Evidence: []backend.PlanEvidence{{
			ID:       evidenceID,
			Kind:     "plan." + strings.ReplaceAll(status, "_", "-"),
			Hash:     doc.Hash,
			Metadata: metadata,
		}},
	}
	if status == plan.StatusImplemented {
		ledger.ImplementedAt = now
	}
	if err := s.Backend.Plans.Record(ctx, ledger); err != nil {
		return plan.LedgerSummary{}, err
	}
	return plan.LedgerSummary{
		LedgerID:   ledgerID,
		Status:     status,
		Hash:       doc.Hash,
		RecordedAt: now,
	}, nil
}

func latestRunID(ctx context.Context, client *backend.Client, targetName string) (string, error) {
	runs, err := client.Runs.List(ctx, backend.RunQuery{Target: targetName, Limit: 1})
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return "", fmt.Errorf("run evidence not found for %s", targetName)
	}
	return runs[0].ID, nil
}

func (s Service) now() time.Time {
	return clock.UTC(s.Now)
}

func (s Service) externalDependencyDiagnostics(
	ctx context.Context,
	doc plan.Document,
) ([]plan.Diagnostic, error) {
	diagnostics := []plan.Diagnostic{}
	for _, dep := range doc.DependsOn {
		ledger, ok, err := s.Backend.Plans.Get(ctx, dep)
		if err != nil {
			return nil, err
		}
		if !ok {
			diagnostics = append(diagnostics, diagnostic(
				doc,
				"missing-dependency-ledger",
				"Plan dependency "+dep+" has no Backend ledger",
			))
			continue
		}
		if ledger.Status != plan.StatusImplemented {
			diagnostics = append(diagnostics, diagnostic(
				doc,
				"dependency-not-implemented",
				"Plan dependency "+dep+" is "+ledger.Status,
			))
		}
	}
	return diagnostics, nil
}

func filterExternalDependencyDiagnostics(
	diagnostics []plan.Diagnostic,
	doc plan.Document,
) []plan.Diagnostic {
	out := diagnostics[:0]
	for _, diagnostic := range diagnostics {
		if diagnostic.File == doc.Path && diagnostic.Code == "missing-plan-dependency" {
			continue
		}
		out = append(out, diagnostic)
	}
	return out
}

func hasErrorDiagnostics(diagnostics []plan.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return true
		}
	}
	return false
}

func diagnostic(doc plan.Document, code string, message string) plan.Diagnostic {
	return plan.Diagnostic{Severity: "error", File: doc.Path, Code: code, Message: message}
}

func generatedTargetName(doc plan.Document) string {
	return "agent/plan." + doc.ID
}

var _ runner.TargetHandlers = target.BuiltinTargetRegistry()
