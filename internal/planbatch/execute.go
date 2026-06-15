package planbatch

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/plan"
	"github.com/applauselab/bachkator/internal/planexecute"
	"github.com/applauselab/bachkator/internal/planstatus"
)

func (s Service) Execute(
	ctx context.Context,
	project *model.RunProject,
	client planstatus.LedgerClient,
	opts Options,
) (Result, error) {
	if len(opts.Paths) == 0 {
		return Result{}, planstatus.ErrNoPlanPaths
	}
	if opts.StopOn == "" {
		opts.StopOn = StopOnFailure
	}
	if opts.Parallelism <= 0 {
		opts.Parallelism = 1
	}

	status, err := planstatus.Status(ctx, project, client, planstatus.Options{Paths: opts.Paths})
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Waves:     status.Selection.Waves,
		Plans:     make([]PlanResult, 0, len(status.Records)),
		StartedAt: s.now(),
	}

	external, err := loadExternalLedgers(ctx, client, status.Selection)
	if err != nil {
		return Result{}, err
	}

	state := newBatchState(status, external, opts)
	stopped := false

	for waveIndex, wave := range status.Selection.Waves {
		select {
		case <-ctx.Done():
			result.Plans = append(result.Plans, state.flushRemaining(ctx.Err())...)
			return result, ctx.Err()
		default:
		}

		ready := state.readyPlans(wave)
		if stopped {
			for _, item := range ready {
				state.skip(item.record.Document.ID, "batch stopped after earlier failure")
			}
			continue
		}

		if err := s.executeWave(ctx, ready, opts); err != nil {
			result.Plans = append(result.Plans, state.flushRemaining(err)...)
			return result, err
		}

		if opts.StopOn == StopOnFailure && state.waveHadFailure(waveIndex) {
			stopped = true
		}
	}

	result.Plans = state.sortedResults()
	result.EndedAt = s.now()
	return result, nil
}

func (s Service) executeWave(
	ctx context.Context,
	ready []readyPlan,
	opts Options,
) error {
	if len(ready) == 0 {
		return nil
	}

	work := make(chan readyPlan, len(ready))
	for _, item := range ready {
		work <- item
	}
	close(work)

	var wg sync.WaitGroup
	errs := make(chan error, len(ready))

	workers := opts.Parallelism
	if workers > len(ready) {
		workers = len(ready)
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				select {
				case <-ctx.Done():
					errs <- fmt.Errorf("wave execution cancelled: %w", ctx.Err())
					return
				default:
				}
				s.runOne(ctx, item, opts)
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (s Service) runOne(ctx context.Context, item readyPlan, opts Options) {
	execOpts := planexecute.Options{
		Path:        item.record.Document.Path,
		DryRun:      opts.DryRun,
		Force:       opts.Force,
		Yes:         opts.Yes,
		EnvFile:     opts.EnvFile,
		LogOnly:     opts.LogOnly,
		Verbose:     opts.Verbose,
		Parallelism: 1,
		Template:    opts.Template,
	}
	execResult, err := s.Implement(ctx, execOpts)
	item.state.complete(item.record.Document.ID, execResult, err)
}

func loadExternalLedgers(
	ctx context.Context,
	client planstatus.LedgerClient,
	selection plan.Selection,
) (map[string]bool, error) {
	selected := make(map[string]bool, len(selection.Documents))
	for _, doc := range selection.Documents {
		selected[doc.ID] = true
	}

	external := make(map[string]bool)
	for _, doc := range selection.Documents {
		for _, dep := range doc.DependsOn {
			if selected[dep] {
				continue
			}
			if _, ok := external[dep]; ok {
				continue
			}
			ledger, _, err := client.Get(ctx, dep)
			if err != nil {
				return nil, err
			}
			external[dep] = ledger.Status == plan.StatusImplemented
		}
	}
	return external, nil
}

type readyPlan struct {
	record plan.StatusRecord
	state  *batchState
}

type batchState struct {
	opts           Options
	recordsByID    map[string]plan.StatusRecord
	selection      plan.Selection
	results        map[string]PlanResult
	externalStatus map[string]bool
	mu             sync.Mutex
}

func newBatchState(
	status planstatus.Result,
	externalStatus map[string]bool,
	opts Options,
) *batchState {
	recordsByID := make(map[string]plan.StatusRecord, len(status.Records))
	for _, record := range status.Records {
		recordsByID[record.Document.ID] = record
	}
	return &batchState{
		opts:           opts,
		recordsByID:    recordsByID,
		selection:      status.Selection,
		results:        make(map[string]PlanResult, len(status.Records)),
		externalStatus: externalStatus,
	}
}

func (b *batchState) readyPlans(wave []string) []readyPlan {
	ready := make([]readyPlan, 0, len(wave))
	for _, id := range wave {
		record, ok := b.recordsByID[id]
		if !ok {
			continue
		}
		if result, done := b.classify(record); done {
			b.results[id] = result
			continue
		}
		ready = append(ready, readyPlan{record: record, state: b})
	}
	return ready
}

func (b *batchState) classify(record plan.StatusRecord) (PlanResult, bool) {
	if len(record.Diagnostics) > 0 && hasError(record.Diagnostics) {
		return PlanResult{
			Plan:        record.Document,
			State:       StateFailed,
			Reason:      "plan has validation errors",
			Diagnostics: append([]plan.Diagnostic(nil), record.Diagnostics...),
		}, true
	}

	if record.Ledger != nil && record.Ledger.Status == plan.StatusImplemented &&
		record.Ledger.Hash == record.Document.Hash && !b.opts.Force {
		return PlanResult{
			Plan:   record.Document,
			State:  StateAlreadyImplemented,
			Reason: "latest ledger is implemented and hash matches",
			Ledger: cloneLedger(record.Ledger),
		}, true
	}

	for _, dep := range record.Document.DependsOn {
		if !b.dependencyImplemented(dep) {
			return PlanResult{
				Plan:   record.Document,
				State:  StateBlocked,
				Reason: "dependency " + dep + " is not implemented",
			}, true
		}
	}

	return PlanResult{}, false
}

func (b *batchState) dependencyImplemented(dep string) bool {
	if result, ok := b.results[dep]; ok {
		return result.State == StateImplemented || result.State == StateAlreadyImplemented
	}
	return b.externalStatus[dep]
}

func (b *batchState) complete(id string, execResult planexecute.Result, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	record := b.recordsByID[id]
	result := PlanResult{
		Plan:        record.Document,
		Target:      execResult.Target,
		RunID:       execResult.RunID,
		Diagnostics: append([]plan.Diagnostic(nil), execResult.Diagnostics...),
	}
	if execResult.Ledger != nil {
		result.Ledger = cloneLedger(execResult.Ledger)
	}

	switch {
	case err != nil:
		result.State = StateFailed
		result.Reason = err.Error()
	case execResult.Result == planexecute.ResultSkipped:
		result.State = StateAlreadyImplemented
		result.Reason = "latest ledger is implemented and hash matches"
	case execResult.Result == planexecute.ResultFailed:
		result.State = StateFailed
		result.Reason = "implementation run failed"
	default:
		result.State = StateImplemented
		result.Reason = "implementation run succeeded"
	}

	b.results[id] = result
}

func (b *batchState) skip(id string, reason string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	record := b.recordsByID[id]
	b.results[id] = PlanResult{
		Plan:   record.Document,
		State:  StateSkipped,
		Reason: reason,
	}
}

func (b *batchState) waveHadFailure(waveIndex int) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, id := range b.resultsInWave(waveIndex) {
		result := b.results[id]
		if result.State == StateFailed {
			return true
		}
	}
	return false
}

func (b *batchState) resultsInWave(waveIndex int) []string {
	if waveIndex >= len(b.selection.Waves) {
		return nil
	}
	out := make([]string, 0, len(b.selection.Waves[waveIndex]))
	for _, id := range b.selection.Waves[waveIndex] {
		if _, ok := b.results[id]; ok {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func (b *batchState) flushRemaining(err error) []PlanResult {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]PlanResult, 0, len(b.recordsByID))
	for _, wave := range b.selection.Waves {
		for _, id := range wave {
			if existing, ok := b.results[id]; ok {
				out = append(out, existing)
				continue
			}
			record := b.recordsByID[id]
			out = append(out, PlanResult{
				Plan:   record.Document,
				State:  StateSkipped,
				Reason: err.Error(),
			})
		}
	}
	return out
}

func (b *batchState) sortedResults() []PlanResult {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]PlanResult, 0, len(b.recordsByID))
	for _, wave := range b.selection.Waves {
		for _, id := range wave {
			if result, ok := b.results[id]; ok {
				out = append(out, result)
			}
		}
	}
	return out
}

func hasError(diagnostics []plan.Diagnostic) bool {
	for _, d := range diagnostics {
		if d.Severity == "error" {
			return true
		}
	}
	return false
}

func cloneLedger(ledger *plan.LedgerSummary) *plan.LedgerSummary {
	if ledger == nil {
		return nil
	}
	copy := *ledger
	return &copy
}
