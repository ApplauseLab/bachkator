package sqlite

import (
	"encoding/json"
	"time"

	"github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func runRecordFromProtocol(run backendprotocol.RunRecord) (state.RunRecord, error) {
	startedAt, err := time.Parse(time.RFC3339Nano, run.StartedAt)
	if err != nil {
		return state.RunRecord{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			"started_at must be RFC3339 UTC",
		)
	}
	var finishedAt time.Time
	if run.FinishedAt != "" {
		finishedAt, err = time.Parse(time.RFC3339Nano, run.FinishedAt)
		if err != nil {
			return state.RunRecord{}, backendprotocol.NewError(
				backendprotocol.ErrorValidationFailed,
				"finished_at must be RFC3339 UTC",
			)
		}
	}
	return state.RunRecord{
		ID:         run.ID,
		Target:     run.Target,
		DryRun:     run.DryRun,
		Force:      run.Force,
		Status:     run.Status,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		LogDir:     run.LogDir,
		Targets:    map[string]state.TargetRunRecord{},
	}, nil
}

func runRecordToProtocol(run state.RunRecord) backendprotocol.RunRecord {
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

func runFinishFromProtocol(
	params backendprotocol.RunFinishParams,
	fallbackCreatedAt time.Time,
) (state.RunRecord, map[string]state.Record, error) {
	run, err := runRecordFromProtocol(params.Run)
	if err != nil {
		return state.RunRecord{}, nil, err
	}
	run.Targets = make(map[string]state.TargetRunRecord, len(params.TargetRuns))
	for name, target := range params.TargetRuns {
		record, err := targetRunRecordFromProtocol(target)
		if err != nil {
			return state.RunRecord{}, nil, err
		}
		run.Targets[name] = record
	}
	run.Artifacts = make([]state.ArtifactRecord, 0, len(params.Evidence))
	for _, evidence := range params.Evidence {
		createdAt := fallbackCreatedAt
		if evidence.CreatedAt != "" {
			parsed, err := parseOptionalTime(evidence.CreatedAt, "created_at")
			if err != nil {
				return state.RunRecord{}, nil, err
			}
			createdAt = parsed
		}
		run.Artifacts = append(run.Artifacts, state.ArtifactRecord{
			RunID:     evidence.RunID,
			Target:    evidence.Target,
			Kind:      evidence.Kind,
			Path:      evidence.URI,
			Value:     evidence.Hash,
			CreatedAt: createdAt,
		})
	}
	targets := make(map[string]state.Record, len(params.Targets))
	for name, target := range params.Targets {
		completedAt, err := parseOptionalTime(target.CompletedAt, "completed_at")
		if err != nil {
			return state.RunRecord{}, nil, err
		}
		targets[name] = state.Record{
			Fingerprint:      target.Fingerprint,
			FingerprintParts: target.FingerprintParts,
			CompletedAt:      completedAt,
		}
	}
	return run, targets, nil
}

func runQueryFromProtocol(raw json.RawMessage) (state.RunQuery, error) {
	var query backendprotocol.RunQuery
	if err := json.Unmarshal(raw, &query); err != nil {
		return state.RunQuery{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	result := state.RunQuery{Target: query.Target, Status: query.Status, Limit: query.Limit}
	if query.Since != "" {
		since, err := time.Parse(time.RFC3339Nano, query.Since)
		if err != nil {
			return state.RunQuery{}, backendprotocol.NewError(
				backendprotocol.ErrorValidationFailed,
				"since must be RFC3339 UTC",
			)
		}
		result.Since = since
	}
	return result, nil
}

func targetRunRecordFromProtocol(
	run backendprotocol.TargetRunRecord,
) (state.TargetRunRecord, error) {
	startedAt, err := parseOptionalTime(run.StartedAt, "started_at")
	if err != nil {
		return state.TargetRunRecord{}, err
	}
	finishedAt, err := parseOptionalTime(run.FinishedAt, "finished_at")
	if err != nil {
		return state.TargetRunRecord{}, err
	}
	return state.TargetRunRecord{
		Status:     run.Status,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		LogPath:    run.LogPath,
		Operation:  run.Operation,
		ExitCode:   run.ExitCode,
	}, nil
}

func qualityReportFromProtocol(report backendprotocol.QualityReport) (state.QualityReport, error) {
	createdAt, err := parseOptionalTime(report.CreatedAt, "created_at")
	if err != nil {
		return state.QualityReport{}, err
	}
	record := state.QualityReport{
		RunID:     report.RunID,
		Target:    report.Target,
		Kind:      report.Kind,
		Format:    report.Format,
		Path:      report.Path,
		Status:    report.Status,
		Message:   report.Message,
		CreatedAt: createdAt,
		Metrics:   make([]state.QualityMetric, 0, len(report.Metrics)),
		Findings:  make([]state.QualityFinding, 0, len(report.Findings)),
	}
	for _, metric := range report.Metrics {
		record.Metrics = append(record.Metrics, state.QualityMetric{
			Name:  metric.Name,
			Scope: metric.Scope,
			Value: metric.Value,
			Unit:  metric.Unit,
		})
	}
	for _, finding := range report.Findings {
		record.Findings = append(record.Findings, state.QualityFinding{
			Kind:       finding.Kind,
			File:       finding.File,
			Line:       finding.Line,
			Severity:   finding.Severity,
			Rule:       finding.Rule,
			Message:    finding.Message,
			DurationMS: finding.DurationMS,
		})
	}
	return record, nil
}

func qualityGateFromProtocol(
	gate backendprotocol.QualityGateResult,
) (state.QualityGateResult, error) {
	createdAt, err := parseOptionalTime(gate.CreatedAt, "created_at")
	if err != nil {
		return state.QualityGateResult{}, err
	}
	return state.QualityGateResult{
		RunID:     gate.RunID,
		Target:    gate.Target,
		Metric:    gate.Metric,
		Op:        gate.Op,
		Threshold: gate.Threshold,
		Actual:    gate.Actual,
		Status:    gate.Status,
		Message:   gate.Message,
		CreatedAt: createdAt,
	}, nil
}

func parseOptionalTime(value string, field string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			field+" must be RFC3339 UTC",
		)
	}
	return parsed, nil
}

func (p *Provider) findStateRun(id string) (state.RunRecord, error) {
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return state.RunRecord{}, err
	}
	defer func() { _ = store.Close() }()
	runs, err := store.ListRuns(state.RunQuery{})
	if err != nil {
		return state.RunRecord{}, err
	}
	for _, run := range runs {
		if run.ID == id {
			return run, nil
		}
	}
	return state.RunRecord{}, backendprotocol.NewError(
		backendprotocol.ErrorValidationFailed,
		"run not found",
	)
}

func findingQueryFromProtocol(raw json.RawMessage) (state.FindingQuery, error) {
	var query backendprotocol.FindingQuery
	if err := json.Unmarshal(raw, &query); err != nil {
		return state.FindingQuery{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	return state.FindingQuery{
		Fingerprint: query.Fingerprint,
		Status:      query.Status,
		Limit:       query.Limit,
	}, nil
}

func findingListToProtocol(findings []state.NormalizedFinding) backendprotocol.FindingListResult {
	result := backendprotocol.FindingListResult{
		Findings: make([]backendprotocol.FindingObservation, 0, len(findings)),
	}
	for _, finding := range findings {
		result.Findings = append(result.Findings, findingToProtocol(finding))
	}
	return result
}

func findingToProtocol(finding state.NormalizedFinding) backendprotocol.FindingObservation {
	result := backendprotocol.FindingObservation{
		SchemaVersion:        "bach.backend.finding.v1",
		ID:                   finding.ID,
		SourceType:           finding.SourceType,
		SourceID:             finding.SourceID,
		Severity:             backendprotocol.FindingSeverity(finding.Severity),
		Category:             finding.Category,
		Message:              finding.Message,
		SuggestedFingerprint: finding.SuggestedFingerprint,
		Fingerprint:          finding.Fingerprint,
		ObservedAt:           finding.ObservedAt,
		Status:               finding.Status,
		Metadata:             finding.Metadata,
	}
	if finding.Location != nil {
		result.Location = &backendprotocol.FindingLocation{
			Path:        finding.Location.Path,
			StartLine:   finding.Location.StartLine,
			StartColumn: finding.Location.StartColumn,
			EndLine:     finding.Location.EndLine,
			EndColumn:   finding.Location.EndColumn,
		}
	}
	return result
}
