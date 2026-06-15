package backend

import (
	"context"
	"time"

	"github.com/applauselab/bachkator/internal/id"
	statestore "github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func (c RunsClient) Create(ctx context.Context, run RunRecord) error {
	if !c.client.provider {
		_, err := withStore(ctx, c.client.path, func(store *statestore.Store) (struct{}, error) {
			return struct{}{}, store.RecordRunStart(run)
		})
		return err
	}
	return c.client.callProvider(ctx, "runs.create", runRecordToProtocol(run))
}

func (c RunsClient) Finish(
	ctx context.Context,
	targets map[string]StateRecord,
	run RunRecord,
) error {
	if !c.client.provider {
		_, err := withStore(ctx, c.client.path, func(store *statestore.Store) (struct{}, error) {
			return struct{}{}, store.RecordRunCompletion(targets, run)
		})
		return err
	}
	return c.client.callProvider(ctx, "runs.finish", runFinishToProtocol(targets, run))
}

func (c RunsClient) List(ctx context.Context, query RunQuery) ([]RunRecord, error) {
	if !c.client.provider {
		return withStore(ctx, c.client.path, func(store *statestore.Store) ([]RunRecord, error) {
			return store.ListRuns(query)
		})
	}
	var result backendprotocol.RunListResult
	if err := c.client.callProviderResult(
		ctx,
		"runs.list",
		runQueryToProtocol(query),
		&result,
	); err != nil {
		return nil, err
	}
	runs := make([]RunRecord, 0, len(result.Runs))
	for _, protocolRun := range result.Runs {
		run, err := runRecordFromProtocol(protocolRun)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, nil
}

func runRecordToProtocol(run RunRecord) backendprotocol.RunRecord {
	result := backendprotocol.RunRecord{
		SchemaVersion: "bach.backend.run.v1",
		ID:            run.ID,
		Target:        run.Target,
		Status:        run.Status,
		StartedAt:     run.StartedAt.UTC().Format(time.RFC3339Nano),
		LogDir:        run.LogDir,
		DryRun:        run.DryRun,
		Force:         run.Force,
	}
	if !run.FinishedAt.IsZero() {
		result.FinishedAt = run.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func runQueryToProtocol(query RunQuery) backendprotocol.RunQuery {
	result := backendprotocol.RunQuery{
		Target: query.Target,
		Status: query.Status,
		Limit:  query.Limit,
	}
	if !query.Since.IsZero() {
		result.Since = query.Since.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func runRecordFromProtocol(run backendprotocol.RunRecord) (RunRecord, error) {
	startedAt, err := parseFactoryTime(run.StartedAt, "started_at")
	if err != nil {
		return RunRecord{}, err
	}
	finishedAt, err := parseFactoryTime(run.FinishedAt, "finished_at")
	if err != nil {
		return RunRecord{}, err
	}
	return RunRecord{
		ID:         run.ID,
		Target:     run.Target,
		DryRun:     run.DryRun,
		Force:      run.Force,
		Status:     run.Status,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		LogDir:     run.LogDir,
		Targets:    map[string]TargetRunRecord{},
	}, nil
}

func runFinishToProtocol(
	targets map[string]StateRecord,
	run RunRecord,
) backendprotocol.RunFinishParams {
	result := backendprotocol.RunFinishParams{
		Run:        runRecordToProtocol(run),
		Targets:    make(map[string]backendprotocol.TargetStateRecord, len(targets)),
		TargetRuns: make(map[string]backendprotocol.TargetRunRecord, len(run.Targets)),
		Evidence:   make([]backendprotocol.EvidenceRef, 0, len(run.Artifacts)),
	}
	for name, target := range targets {
		result.Targets[name] = backendprotocol.TargetStateRecord{
			Fingerprint:      target.Fingerprint,
			FingerprintParts: target.FingerprintParts,
			CompletedAt:      target.CompletedAt.UTC().Format(time.RFC3339Nano),
		}
	}
	for name, target := range run.Targets {
		result.TargetRuns[name] = targetRunToProtocol(name, target)
	}
	for _, artifact := range run.Artifacts {
		refID, err := id.New()
		if err != nil {
			continue
		}
		ref := backendprotocol.EvidenceRef{
			SchemaVersion: "bach.backend.evidence_ref.v1",
			ID:            refID,
			Kind:          artifact.Kind,
			URI:           artifact.Path,
			Hash:          artifact.Value,
			RunID:         artifact.RunID,
			Target:        artifact.Target,
		}
		if !artifact.CreatedAt.IsZero() {
			ref.CreatedAt = artifact.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
		result.Evidence = append(result.Evidence, ref)
	}
	return result
}

func targetRunToProtocol(name string, target TargetRunRecord) backendprotocol.TargetRunRecord {
	result := backendprotocol.TargetRunRecord{
		Target:    name,
		Status:    target.Status,
		StartedAt: target.StartedAt.UTC().Format(time.RFC3339Nano),
		LogPath:   target.LogPath,
		ExitCode:  target.ExitCode,
	}
	if !target.FinishedAt.IsZero() {
		result.FinishedAt = target.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}
