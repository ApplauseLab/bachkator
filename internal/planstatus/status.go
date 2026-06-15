package planstatus

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/applauselab/bachkator/internal/bacherr"
	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/evidence"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/plan"
)

var ErrNoPlanPaths = errors.New("at least one plan file is required")

type LedgerClient interface {
	Get(ctx context.Context, planID string) (backend.PlanLedger, bool, error)
}

type Result struct {
	Selection   plan.Selection
	Records     []plan.StatusRecord
	Diagnostics []plan.Diagnostic
}

type Options struct {
	Paths []string
}

func Status(
	ctx context.Context,
	project *model.RunProject,
	client LedgerClient,
	opts Options,
) (Result, error) {
	if project == nil {
		return Result{}, bacherr.ValidationFailedf("project is required")
	}
	if len(opts.Paths) == 0 {
		return Result{}, ErrNoPlanPaths
	}
	documents := make([]plan.Document, 0, len(opts.Paths))
	diagnostics := []plan.Diagnostic{}
	for _, path := range opts.Paths {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		default:
		}
		doc, docDiagnostics, err := loadDocument(project.Root, path)
		if err != nil {
			diagnostics = append(diagnostics, plan.Diagnostic{
				Severity: "error",
				File:     filepath.ToSlash(path),
				Code:     "plan-file-error",
				Message:  err.Error(),
			})
			continue
		}
		diagnostics = append(diagnostics, docDiagnostics...)
		diagnostics = append(diagnostics, validateReferences(project, doc)...)
		documents = append(documents, doc)
	}
	selection := plan.BuildSelection(documents)
	diagnostics = append(diagnostics, selection.Diagnostics...)
	ledgers := map[string]plan.Ledger{}
	for _, doc := range selection.Documents {
		ledger, ok, err := client.Get(ctx, doc.ID)
		if err != nil {
			return Result{}, err
		}
		if ok {
			ledgers[doc.ID] = plan.Ledger{
				SchemaVersion: ledger.SchemaVersion,
				LedgerID:      ledger.LedgerID,
				PlanID:        ledger.PlanID,
				Status:        ledger.Status,
				Hash:          ledger.Hash,
				RunID:         ledger.RunID,
				Commit:        ledger.Commit,
				RecordedAt:    ledger.RecordedAt,
				ImplementedAt: ledger.ImplementedAt,
				Evidence:      planEvidence(ledger.Evidence),
			}
		}
	}
	records := plan.DeriveStatuses(selection, ledgers)
	for _, record := range records {
		diagnostics = append(diagnostics, record.Diagnostics...)
	}
	return Result{Selection: selection, Records: records, Diagnostics: diagnostics}, nil
}

func loadDocument(root string, path string) (plan.Document, []plan.Diagnostic, error) {
	resolved, err := evidence.ResolveProjectFile(root, path)
	if err != nil {
		return plan.Document{}, nil, err
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return plan.Document{}, nil, err
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return plan.Document{}, nil, err
	}
	rel, err := filepath.Rel(rootReal, resolved)
	if err != nil {
		return plan.Document{}, nil, err
	}
	doc, diagnostics := plan.Parse(filepath.ToSlash(rel), data)
	return doc, diagnostics, nil
}

func validateReferences(project *model.RunProject, doc plan.Document) []plan.Diagnostic {
	diagnostics := []plan.Diagnostic{}
	if doc.AgentTemplate != "" {
		key := canonicalRef(doc.AgentTemplate, "agent_template")
		if project.AgentTemplates[key] == nil {
			diagnostics = append(
				diagnostics,
				diagnostic(
					doc,
					"unknown-agent-template",
					"unknown Agent Template "+doc.AgentTemplate,
				),
			)
		}
	}
	if doc.Policy != "" {
		key := canonicalRef(doc.Policy, "policy")
		if project.Policies[key] == nil {
			diagnostics = append(
				diagnostics,
				diagnostic(doc, "unknown-policy", "unknown policy "+doc.Policy),
			)
		}
	}
	for _, target := range doc.RequiredTargets {
		canonical, _ := project.ResolveTargetName(target)
		if project.Targets[canonical] == nil {
			diagnostics = append(
				diagnostics,
				diagnostic(doc, "unknown-required-target", "unknown required target "+target),
			)
		}
	}
	return diagnostics
}

func canonicalRef(value string, prefix string) string {
	if strings.HasPrefix(value, prefix+"/") {
		return value
	}
	return prefix + "/" + strings.TrimPrefix(value, prefix+".")
}

func diagnostic(doc plan.Document, code string, message string) plan.Diagnostic {
	return plan.Diagnostic{Severity: "error", File: doc.Path, Code: code, Message: message}
}

func planEvidence(evidence []backend.PlanEvidence) []plan.Evidence {
	out := make([]plan.Evidence, 0, len(evidence))
	for _, item := range evidence {
		out = append(out, plan.Evidence{
			ID:       item.ID,
			Kind:     item.Kind,
			Hash:     item.Hash,
			Content:  cloneAnyMap(item.Content),
			Metadata: cloneStringMap(item.Metadata),
		})
	}
	return out
}

func cloneAnyMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
