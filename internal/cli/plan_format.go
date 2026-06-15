package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/applauselab/bachkator/internal/plan"
	"github.com/applauselab/bachkator/internal/planbatch"
	"github.com/applauselab/bachkator/internal/planexecute"
)

func writePlanStatusJSON(stdout io.Writer, result PlanStatusResult) error {
	views := make([]planStatusView, 0, len(result.Records))
	for _, record := range result.Records {
		views = append(views, planStatusViewFor(record))
	}
	data, err := json.MarshalIndent(planStatusJSON{
		SchemaVersion: "bach.plan_status.v1",
		Plans:         views,
		Waves:         result.Waves,
		Diagnostics:   result.Diagnostics,
	}, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s\n", data)
	return err
}

func writePlanImplementJSON(stdout io.Writer, result planexecute.Result) error {
	written := make([]planLedgerView, 0, len(result.Written))
	for _, ledger := range result.Written {
		written = append(written, planLedgerViewFor(ledger))
	}
	var ledger *planLedgerView
	if result.Ledger != nil {
		view := planLedgerViewFor(*result.Ledger)
		ledger = &view
	}
	data, err := json.MarshalIndent(planImplementJSON{
		SchemaVersion: "bach.plan_implement.v1",
		Plan: planStatusViewFor(plan.StatusRecord{
			Document:    result.Plan,
			Status:      result.Result,
			Diagnostics: result.Diagnostics,
		}),
		Result:      result.Result,
		Target:      result.Target,
		Template:    result.Template,
		RunID:       result.RunID,
		Ledger:      ledger,
		Written:     written,
		Diagnostics: result.Diagnostics,
	}, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s\n", data)
	return err
}

func writePlanBatchJSON(stdout io.Writer, result planbatch.Result) error {
	views := make([]planResultView, 0, len(result.Plans))
	for _, planResult := range result.Plans {
		views = append(views, planResultViewFor(planResult))
	}
	data, err := json.MarshalIndent(planBatchJSON{
		SchemaVersion: "bach.plan_batch.v1",
		Plans:         views,
		Waves:         result.Waves,
		StartedAt:     formatPlanTime(result.StartedAt),
		EndedAt:       formatPlanTime(result.EndedAt),
	}, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s\n", data)
	return err
}

func writePlanReviewJSON(stdout io.Writer, result PlanReviewResult) error {
	data, err := json.MarshalIndent(planReviewJSON{
		SchemaVersion: "bach.plan_review.v1",
		Implemented:   reviewItemsFor(result.Queue.Implemented),
		NeedsReview:   reviewItemsFor(result.Queue.NeedsReview),
		Failed:        reviewItemsFor(result.Queue.Failed),
		Blocked:       reviewItemsFor(result.Queue.Blocked),
		Skipped:       reviewItemsFor(result.Queue.Skipped),
		Diagnostics:   result.Diagnostics,
	}, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s\n", data)
	return err
}

func reviewItemsFor(items []planbatch.ReviewItem) []reviewItemView {
	views := make([]reviewItemView, 0, len(items))
	for _, item := range items {
		views = append(views, reviewItemViewFor(item))
	}
	return views
}

func writePlanImplementHuman(stdout io.Writer, result planexecute.Result) error {
	if result.Result == "" {
		if _, err := fmt.Fprintf(
			stdout,
			"plan %s target=%s\n",
			result.Plan.ID,
			result.Target,
		); err != nil {
			return err
		}
	} else if _, err := fmt.Fprintf(
		stdout,
		"%s %s hash=%s target=%s",
		result.Result,
		result.Plan.ID,
		shortHash(result.Plan.Hash),
		result.Target,
	); err != nil {
		return err
	}
	if result.RunID != "" {
		if _, err := fmt.Fprintf(stdout, " run=%s", result.RunID); err != nil {
			return err
		}
	}
	if result.Ledger != nil {
		if _, err := fmt.Fprintf(stdout, " ledger=%s", result.Ledger.LedgerID); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(stdout); err != nil {
		return err
	}
	for _, diagnostic := range result.Diagnostics {
		if _, err := fmt.Fprintf(
			stdout,
			"%s: %s: %s\n",
			diagnostic.File,
			diagnostic.Code,
			diagnostic.Message,
		); err != nil {
			return err
		}
	}
	return nil
}

func writePlanBatchHuman(stdout io.Writer, result planbatch.Result) error {
	counts := map[string]int{}
	for _, planResult := range result.Plans {
		counts[planResult.State]++
	}
	if _, err := fmt.Fprintf(
		stdout,
		"batch: %d plans, %d waves, implemented=%d failed=%d blocked=%d skipped=%d already_implemented=%d\n",
		len(result.Plans),
		len(result.Waves),
		counts[planbatch.StateImplemented],
		counts[planbatch.StateFailed],
		counts[planbatch.StateBlocked],
		counts[planbatch.StateSkipped],
		counts[planbatch.StateAlreadyImplemented],
	); err != nil {
		return err
	}

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "PLAN\tSTATE\tRUN\tTARGET\tREASON"); err != nil {
		return err
	}
	for _, planResult := range result.Plans {
		run := "-"
		if planResult.RunID != "" {
			run = shortHash(planResult.RunID)
		}
		target := "-"
		if planResult.Target != "" {
			target = planResult.Target
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			planResult.Plan.ID,
			planResult.State,
			run,
			target,
			planResult.Reason,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func writePlanReviewHuman(stdout io.Writer, result PlanReviewResult) error {
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "GROUP\tPLAN\tSTATE\tREASON"); err != nil {
		return err
	}
	writeGroup := func(name string, items []planbatch.ReviewItem) error {
		for _, item := range items {
			if _, err := fmt.Fprintf(
				tw,
				"%s\t%s\t%s\t%s\n",
				name,
				item.PlanID,
				item.State,
				item.Reason,
			); err != nil {
				return err
			}
		}
		return nil
	}
	if err := writeGroup("implemented", result.Queue.Implemented); err != nil {
		return err
	}
	if err := writeGroup("needs_review", result.Queue.NeedsReview); err != nil {
		return err
	}
	if err := writeGroup("failed", result.Queue.Failed); err != nil {
		return err
	}
	if err := writeGroup("blocked", result.Queue.Blocked); err != nil {
		return err
	}
	if err := writeGroup("skipped", result.Queue.Skipped); err != nil {
		return err
	}
	return tw.Flush()
}

func writePlanStatusHuman(stdout io.Writer, result PlanStatusResult) error {
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "PLAN\tSTATUS\tHASH\tDEPENDS ON\tTITLE"); err != nil {
		return err
	}
	for _, record := range result.Records {
		depends := "-"
		if len(record.Document.DependsOn) > 0 {
			depends = strings.Join(record.Document.DependsOn, ",")
		}
		if _, err := fmt.Fprintf(
			tw,
			"%s\t%s\t%s\t%s\t%s\n",
			record.Document.ID,
			record.Status,
			shortHash(record.Document.Hash),
			depends,
			record.Document.Title,
		); err != nil {
			return err
		}
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(stdout); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(stdout, "Planned waves:"); err != nil {
		return err
	}
	for i, wave := range result.Waves {
		if _, err := fmt.Fprintf(stdout, "%d. %s\n", i+1, strings.Join(wave, ", ")); err != nil {
			return err
		}
	}
	for _, diagnostic := range result.Diagnostics {
		if _, err := fmt.Fprintf(
			stdout,
			"%s: %s: %s\n",
			diagnostic.File,
			diagnostic.Code,
			diagnostic.Message,
		); err != nil {
			return err
		}
	}
	return nil
}

func shortHash(hash string) string {
	if strings.HasPrefix(hash, "sha256:") && len(hash) > len("sha256:")+8 {
		return hash[:len("sha256:")+8]
	}
	return hash
}

func hasErrorDiagnostics(diagnostics []plan.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return true
		}
	}
	return false
}
