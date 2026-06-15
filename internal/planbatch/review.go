package planbatch

import (
	"github.com/applauselab/bachkator/internal/plan"
)

const (
	ReviewImplemented = "implemented"
	ReviewNeedsReview = "needs_review"
	ReviewFailed      = "failed"
	ReviewBlocked     = "blocked"
	ReviewSkipped     = "skipped"
)

type ReviewQueue struct {
	Implemented []ReviewItem
	NeedsReview []ReviewItem
	Failed      []ReviewItem
	Blocked     []ReviewItem
	Skipped     []ReviewItem
}

type ReviewItem struct {
	PlanID      string
	PlanPath    string
	Title       string
	State       string
	Reason      string
	RunID       string
	Target      string
	LedgerID    string
	Diagnostics []plan.Diagnostic
}

func Review(result Result) ReviewQueue {
	queue := ReviewQueue{}
	for _, planResult := range result.Plans {
		item := reviewItemForResult(planResult)
		switch reviewState(planResult) {
		case ReviewImplemented:
			queue.Implemented = append(queue.Implemented, item)
		case ReviewNeedsReview:
			queue.NeedsReview = append(queue.NeedsReview, item)
		case ReviewFailed:
			queue.Failed = append(queue.Failed, item)
		case ReviewBlocked:
			queue.Blocked = append(queue.Blocked, item)
		case ReviewSkipped:
			queue.Skipped = append(queue.Skipped, item)
		}
	}
	return queue
}

func ReviewStatus(records []plan.StatusRecord) ReviewQueue {
	queue := ReviewQueue{}
	for _, record := range records {
		item := reviewItemForRecord(record)
		switch reviewStateForRecord(record) {
		case ReviewImplemented:
			queue.Implemented = append(queue.Implemented, item)
		case ReviewNeedsReview:
			queue.NeedsReview = append(queue.NeedsReview, item)
		case ReviewFailed:
			queue.Failed = append(queue.Failed, item)
		case ReviewBlocked:
			queue.Blocked = append(queue.Blocked, item)
		case ReviewSkipped:
			queue.Skipped = append(queue.Skipped, item)
		}
	}
	return queue
}

func reviewItemForResult(result PlanResult) ReviewItem {
	item := ReviewItem{
		PlanID:      result.Plan.ID,
		PlanPath:    result.Plan.Path,
		Title:       result.Plan.Title,
		State:       result.State,
		Reason:      result.Reason,
		RunID:       result.RunID,
		Target:      result.Target,
		Diagnostics: append([]plan.Diagnostic(nil), result.Diagnostics...),
	}
	if result.Ledger != nil {
		item.LedgerID = result.Ledger.LedgerID
	}
	return item
}

func reviewItemForRecord(record plan.StatusRecord) ReviewItem {
	item := ReviewItem{
		PlanID:      record.Document.ID,
		PlanPath:    record.Document.Path,
		Title:       record.Document.Title,
		State:       record.Status,
		Diagnostics: append([]plan.Diagnostic(nil), record.Diagnostics...),
	}
	if record.Ledger != nil {
		item.LedgerID = record.Ledger.LedgerID
	}
	return item
}

func reviewState(result PlanResult) string {
	switch result.State {
	case StateImplemented:
		if len(result.Diagnostics) > 0 {
			return ReviewNeedsReview
		}
		return ReviewImplemented
	case StateAlreadyImplemented:
		return ReviewSkipped
	case StateFailed:
		return ReviewFailed
	case StateBlocked:
		return ReviewBlocked
	case StateSkipped:
		return ReviewSkipped
	}
	return ReviewNeedsReview
}

func reviewStateForRecord(record plan.StatusRecord) string {
	switch record.Status {
	case plan.StatusImplemented:
		if len(record.Diagnostics) > 0 {
			return ReviewNeedsReview
		}
		return ReviewImplemented
	case plan.StatusFailed:
		return ReviewFailed
	case plan.StatusBlocked, plan.StatusStale, plan.StatusInvalidLedger:
		return ReviewBlocked
	case plan.StatusReady, plan.StatusPlanned, plan.StatusPending, plan.StatusInProgress:
		return ReviewBlocked
	}
	return ReviewNeedsReview
}
