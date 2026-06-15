package sqlite

import (
	"encoding/json"
	"time"

	"github.com/applauselab/bachkator/internal/id"
	"github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func (p *Provider) getRun(raw json.RawMessage) (backendprotocol.RunResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.RunResult{}, err
	}
	var query backendprotocol.RunQuery
	if err := json.Unmarshal(raw, &query); err != nil {
		return backendprotocol.RunResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	if query.ID == "" {
		return backendprotocol.RunResult{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			"id is required",
		)
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.RunResult{}, err
	}
	defer func() { _ = store.Close() }()
	runs, err := store.ListRuns(state.RunQuery{Limit: 0})
	if err != nil {
		return backendprotocol.RunResult{}, err
	}
	for _, run := range runs {
		if run.ID == query.ID {
			return backendprotocol.RunResult{Run: runRecordToProtocol(run)}, nil
		}
	}
	return backendprotocol.RunResult{}, backendprotocol.NewError(
		backendprotocol.ErrorValidationFailed,
		"run not found",
	)
}

func (p *Provider) listRuns(raw json.RawMessage) (backendprotocol.RunListResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.RunListResult{}, err
	}
	query, err := runQueryFromProtocol(raw)
	if err != nil {
		return backendprotocol.RunListResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.RunListResult{}, err
	}
	defer func() { _ = store.Close() }()
	runs, err := store.ListRuns(query)
	if err != nil {
		return backendprotocol.RunListResult{}, err
	}
	result := backendprotocol.RunListResult{Runs: make([]backendprotocol.RunRecord, 0, len(runs))}
	for _, run := range runs {
		result.Runs = append(result.Runs, runRecordToProtocol(run))
	}
	return result, nil
}

func (p *Provider) writeTargetRun(raw json.RawMessage) (map[string]bool, error) {
	if err := p.requireInitialized(); err != nil {
		return nil, err
	}
	var target backendprotocol.TargetRunWrite
	if err := json.Unmarshal(raw, &target); err != nil {
		return nil, backendprotocol.NewError(backendprotocol.ErrorInvalidRequest, err.Error())
	}
	if target.RunID == "" || target.Target == "" {
		return nil, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			"run_id and target are required",
		)
	}
	run, err := p.findStateRun(target.RunID)
	if err != nil {
		return nil, err
	}
	record, err := targetRunRecordFromProtocol(target.TargetRunRecord)
	if err != nil {
		return nil, err
	}
	if run.Targets == nil {
		run.Targets = map[string]state.TargetRunRecord{}
	}
	run.Targets[target.Target] = record
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	if err := store.RecordRunCompletion(nil, run); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (p *Provider) recordEvidenceRef(raw json.RawMessage) (map[string]bool, error) {
	if err := p.requireInitialized(); err != nil {
		return nil, err
	}
	var ref backendprotocol.EvidenceRef
	if err := json.Unmarshal(raw, &ref); err != nil {
		return nil, backendprotocol.NewError(backendprotocol.ErrorInvalidRequest, err.Error())
	}
	if ref.RunID == "" || ref.URI == "" {
		return nil, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			"run_id and uri are required",
		)
	}
	createdAt, err := evidenceCreatedAt(ref, p.now())
	if err != nil {
		return nil, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	if err := store.RecordArtifact(state.ArtifactRecord{
		RunID:     ref.RunID,
		Target:    ref.Target,
		Kind:      ref.Kind,
		Path:      ref.URI,
		Value:     ref.Hash,
		CreatedAt: createdAt,
	}); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (p *Provider) listEvidenceRefs(
	raw json.RawMessage,
) (backendprotocol.EvidenceRefListResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.EvidenceRefListResult{}, err
	}
	var query struct {
		RunID  string `json:"run_id,omitempty"`
		Target string `json:"target,omitempty"`
		Limit  int    `json:"limit,omitempty"`
	}
	if err := json.Unmarshal(raw, &query); err != nil {
		return backendprotocol.EvidenceRefListResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.EvidenceRefListResult{}, err
	}
	defer func() { _ = store.Close() }()
	artifacts, err := store.ListArtifacts(state.ArtifactQuery{
		RunID:  query.RunID,
		Target: query.Target,
		Limit:  query.Limit,
	})
	if err != nil {
		return backendprotocol.EvidenceRefListResult{}, err
	}
	result := backendprotocol.EvidenceRefListResult{
		Evidence: make([]backendprotocol.EvidenceRef, 0, len(artifacts)),
	}
	for _, artifact := range artifacts {
		refID, err := id.New()
		if err != nil {
			return backendprotocol.EvidenceRefListResult{}, err
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
	return result, nil
}

func evidenceCreatedAt(ref backendprotocol.EvidenceRef, fallback time.Time) (time.Time, error) {
	if ref.CreatedAt == "" {
		return fallback.UTC(), nil
	}
	return parseOptionalTime(ref.CreatedAt, "created_at")
}

func (p *Provider) recordQualityReport(raw json.RawMessage) (map[string]bool, error) {
	if err := p.requireInitialized(); err != nil {
		return nil, err
	}
	var report backendprotocol.QualityReport
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, backendprotocol.NewError(backendprotocol.ErrorInvalidRequest, err.Error())
	}
	record, err := qualityReportFromProtocol(report)
	if err != nil {
		return nil, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	if err := store.SaveQualityReports([]state.QualityReport{record}, nil); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (p *Provider) recordQualityReports(raw json.RawMessage) (map[string]bool, error) {
	if err := p.requireInitialized(); err != nil {
		return nil, err
	}
	var batch backendprotocol.QualityReportBatch
	if err := json.Unmarshal(raw, &batch); err != nil {
		return nil, backendprotocol.NewError(backendprotocol.ErrorInvalidRequest, err.Error())
	}
	reports := make([]state.QualityReport, 0, len(batch.Reports))
	for _, report := range batch.Reports {
		record, err := qualityReportFromProtocol(report)
		if err != nil {
			return nil, err
		}
		reports = append(reports, record)
	}
	gates := make([]state.QualityGateResult, 0, len(batch.Gates))
	for _, gate := range batch.Gates {
		record, err := qualityGateFromProtocol(gate)
		if err != nil {
			return nil, err
		}
		gates = append(gates, record)
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	if err := store.SaveQualityReports(reports, gates); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (p *Provider) createRun(raw json.RawMessage) (map[string]bool, error) {
	if err := p.requireInitialized(); err != nil {
		return nil, err
	}
	var run backendprotocol.RunRecord
	if err := json.Unmarshal(raw, &run); err != nil {
		return nil, backendprotocol.NewError(backendprotocol.ErrorInvalidRequest, err.Error())
	}
	record, err := runRecordFromProtocol(run)
	if err != nil {
		return nil, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	if err := store.RecordRunStart(record); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (p *Provider) finishRun(raw json.RawMessage) (map[string]bool, error) {
	if err := p.requireInitialized(); err != nil {
		return nil, err
	}
	var params backendprotocol.RunFinishParams
	if err := json.Unmarshal(raw, &params); err == nil && params.Run.ID != "" {
		run, targets, err := runFinishFromProtocol(params, p.now())
		if err != nil {
			return nil, err
		}
		store, err := state.NewStore(p.storePath)
		if err != nil {
			return nil, err
		}
		defer func() { _ = store.Close() }()
		if err := store.RecordRunCompletion(targets, run); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	}
	var run backendprotocol.RunRecord
	if err := json.Unmarshal(raw, &run); err != nil {
		return nil, backendprotocol.NewError(backendprotocol.ErrorInvalidRequest, err.Error())
	}
	record, err := runRecordFromProtocol(run)
	if err != nil {
		return nil, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	if err := store.RecordRunCompletion(nil, record); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (p *Provider) recordFindingObservation(raw json.RawMessage) (map[string]bool, error) {
	if err := p.requireInitialized(); err != nil {
		return nil, err
	}
	var finding backendprotocol.FindingObservation
	if err := json.Unmarshal(raw, &finding); err != nil {
		return nil, backendprotocol.NewError(backendprotocol.ErrorInvalidRequest, err.Error())
	}
	var location *state.FindingLocation
	if finding.Location != nil {
		location = &state.FindingLocation{
			Path:        finding.Location.Path,
			StartLine:   finding.Location.StartLine,
			StartColumn: finding.Location.StartColumn,
			EndLine:     finding.Location.EndLine,
			EndColumn:   finding.Location.EndColumn,
		}
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	if err := store.RecordFindingObservation(state.NormalizedFinding{
		ID:                   finding.ID,
		Fingerprint:          finding.Fingerprint,
		SourceType:           finding.SourceType,
		SourceID:             finding.SourceID,
		Severity:             string(finding.Severity),
		Category:             finding.Category,
		Message:              finding.Message,
		Location:             location,
		SuggestedFingerprint: finding.SuggestedFingerprint,
		ObservedAt:           finding.ObservedAt,
		Metadata:             finding.Metadata,
	}); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (p *Provider) getFinding(raw json.RawMessage) (backendprotocol.FindingObservation, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FindingObservation{}, err
	}
	var query backendprotocol.FindingQuery
	if err := json.Unmarshal(raw, &query); err != nil {
		return backendprotocol.FindingObservation{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	if query.Fingerprint == "" {
		return backendprotocol.FindingObservation{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			"fingerprint is required",
		)
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FindingObservation{}, err
	}
	defer func() { _ = store.Close() }()
	finding, ok, err := store.GetFinding(query.Fingerprint)
	if err != nil {
		return backendprotocol.FindingObservation{}, err
	}
	if !ok {
		return backendprotocol.FindingObservation{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			"finding not found",
		)
	}
	return findingToProtocol(finding), nil
}

func (p *Provider) listCurrentFindings(
	raw json.RawMessage,
) (backendprotocol.FindingListResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FindingListResult{}, err
	}
	query, err := findingQueryFromProtocol(raw)
	if err != nil {
		return backendprotocol.FindingListResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FindingListResult{}, err
	}
	defer func() { _ = store.Close() }()
	findings, err := store.ListCurrentFindings(query)
	if err != nil {
		return backendprotocol.FindingListResult{}, err
	}
	return findingListToProtocol(findings), nil
}

func (p *Provider) listFindingEvents(
	raw json.RawMessage,
) (backendprotocol.FindingListResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FindingListResult{}, err
	}
	query, err := findingQueryFromProtocol(raw)
	if err != nil {
		return backendprotocol.FindingListResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FindingListResult{}, err
	}
	defer func() { _ = store.Close() }()
	findings, err := store.ListFindingEvents(query)
	if err != nil {
		return backendprotocol.FindingListResult{}, err
	}
	return findingListToProtocol(findings), nil
}
