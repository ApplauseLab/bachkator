package cli

import (
	"context"
	"io"
	"time"

	"github.com/applauselab/bachkator/internal/plan"
	"github.com/applauselab/bachkator/internal/planbatch"
	"github.com/applauselab/bachkator/internal/planexecute"
)

type PlanStatusFunc func(context.Context, *Project, []string) (PlanStatusResult, error)

type PlanImplementFunc func(context.Context, *Project, PlanImplementOptions) (planexecute.Result, error)

type PlanBatchFunc func(context.Context, *Project, PlanBatchOptions) (planbatch.Result, error)

type PlanReviewFunc func(context.Context, *Project, PlanReviewOptions) (PlanReviewResult, error)

type PlanImplementOptions struct {
	Path        string
	DryRun      bool
	Force       bool
	Yes         bool
	EnvFile     string
	LogOnly     bool
	Verbose     bool
	Parallelism int
	Stdout      io.Writer
	Stderr      io.Writer
}

type PlanBatchOptions struct {
	Paths       []string
	Parallelism int
	StopOn      planbatch.StopMode
	Force       bool
	DryRun      bool
	Yes         bool
	EnvFile     string
	LogOnly     bool
	Verbose     bool
	Template    string
	Stdout      io.Writer
	Stderr      io.Writer
}

type PlanReviewOptions struct {
	Paths []string
}

type PlanStatusResult struct {
	Records     []plan.StatusRecord
	Waves       [][]string
	Diagnostics []plan.Diagnostic
}

type PlanReviewResult struct {
	Queue       planbatch.ReviewQueue
	Diagnostics []plan.Diagnostic
}

type planStatusJSON struct {
	SchemaVersion string            `json:"schema_version"`
	Plans         []planStatusView  `json:"plans"`
	Waves         [][]string        `json:"waves"`
	Diagnostics   []plan.Diagnostic `json:"diagnostics,omitempty"`
}

type planStatusView struct {
	File        string            `json:"file"`
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Status      string            `json:"status"`
	Hash        string            `json:"hash"`
	DependsOn   []string          `json:"depends_on,omitempty"`
	Ledger      *planLedgerView   `json:"ledger,omitempty"`
	Diagnostics []plan.Diagnostic `json:"diagnostics,omitempty"`
}

type planImplementJSON struct {
	SchemaVersion string            `json:"schema_version"`
	Plan          planStatusView    `json:"plan"`
	Result        string            `json:"result"`
	Target        string            `json:"target"`
	Template      string            `json:"template,omitempty"`
	RunID         string            `json:"run_id,omitempty"`
	Ledger        *planLedgerView   `json:"ledger,omitempty"`
	Written       []planLedgerView  `json:"written_ledgers,omitempty"`
	Diagnostics   []plan.Diagnostic `json:"diagnostics,omitempty"`
}

type planBatchJSON struct {
	SchemaVersion string           `json:"schema_version"`
	Plans         []planResultView `json:"plans"`
	Waves         [][]string       `json:"waves"`
	StartedAt     string           `json:"started_at"`
	EndedAt       string           `json:"ended_at"`
}

type planResultView struct {
	File        string            `json:"file"`
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	State       string            `json:"state"`
	Reason      string            `json:"reason"`
	RunID       string            `json:"run_id,omitempty"`
	Target      string            `json:"target,omitempty"`
	LedgerID    string            `json:"ledger_id,omitempty"`
	Diagnostics []plan.Diagnostic `json:"diagnostics,omitempty"`
}

type planReviewJSON struct {
	SchemaVersion string            `json:"schema_version"`
	Implemented   []reviewItemView  `json:"implemented"`
	NeedsReview   []reviewItemView  `json:"needs_review"`
	Failed        []reviewItemView  `json:"failed"`
	Blocked       []reviewItemView  `json:"blocked"`
	Skipped       []reviewItemView  `json:"skipped"`
	Diagnostics   []plan.Diagnostic `json:"diagnostics,omitempty"`
}

type reviewItemView struct {
	PlanID      string            `json:"plan_id"`
	PlanPath    string            `json:"plan_path"`
	Title       string            `json:"title"`
	State       string            `json:"state"`
	Reason      string            `json:"reason"`
	RunID       string            `json:"run_id,omitempty"`
	Target      string            `json:"target,omitempty"`
	LedgerID    string            `json:"ledger_id,omitempty"`
	Diagnostics []plan.Diagnostic `json:"diagnostics,omitempty"`
}

type planLedgerView struct {
	LedgerID   string `json:"ledger_id"`
	Status     string `json:"status"`
	Hash       string `json:"hash"`
	RecordedAt string `json:"recorded_at"`
}

func planStatusViewFor(record plan.StatusRecord) planStatusView {
	view := planStatusView{
		File:        record.Document.Path,
		ID:          record.Document.ID,
		Title:       record.Document.Title,
		Status:      record.Status,
		Hash:        record.Document.Hash,
		DependsOn:   append([]string(nil), record.Document.DependsOn...),
		Diagnostics: append([]plan.Diagnostic(nil), record.Diagnostics...),
	}
	if record.Ledger != nil {
		view.Ledger = &planLedgerView{
			LedgerID:   record.Ledger.LedgerID,
			Status:     record.Ledger.Status,
			Hash:       record.Ledger.Hash,
			RecordedAt: formatPlanTime(record.Ledger.RecordedAt),
		}
	}
	return view
}

func planLedgerViewFor(summary plan.LedgerSummary) planLedgerView {
	return planLedgerView{
		LedgerID:   summary.LedgerID,
		Status:     summary.Status,
		Hash:       summary.Hash,
		RecordedAt: formatPlanTime(summary.RecordedAt),
	}
}

func formatPlanTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func planResultViewFor(result planbatch.PlanResult) planResultView {
	view := planResultView{
		File:        result.Plan.Path,
		ID:          result.Plan.ID,
		Title:       result.Plan.Title,
		State:       result.State,
		Reason:      result.Reason,
		RunID:       result.RunID,
		Target:      result.Target,
		Diagnostics: append([]plan.Diagnostic(nil), result.Diagnostics...),
	}
	if result.Ledger != nil {
		view.LedgerID = result.Ledger.LedgerID
	}
	return view
}

func reviewItemViewFor(item planbatch.ReviewItem) reviewItemView {
	return reviewItemView{
		PlanID:      item.PlanID,
		PlanPath:    item.PlanPath,
		Title:       item.Title,
		State:       item.State,
		Reason:      item.Reason,
		RunID:       item.RunID,
		Target:      item.Target,
		LedgerID:    item.LedgerID,
		Diagnostics: append([]plan.Diagnostic(nil), item.Diagnostics...),
	}
}
