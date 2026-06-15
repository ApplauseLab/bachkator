package sqlite

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func (p *Provider) recordPlanLedger(raw json.RawMessage) (map[string]bool, error) {
	if err := p.requireInitialized(); err != nil {
		return nil, err
	}
	var params backendprotocol.PlanLedger
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, backendprotocol.NewError(backendprotocol.ErrorInvalidRequest, err.Error())
	}
	ledger, err := planLedgerFromProtocol(params)
	if err != nil {
		return nil, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	if err := store.RecordPlanLedger(ledger); err != nil {
		if errors.Is(err, state.ErrPlanLedgerConflict) {
			return nil, backendprotocol.NewError(
				backendprotocol.ErrorConflict,
				"plan ledger already exists with different payload",
			)
		}
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (p *Provider) getPlanLedger(
	raw json.RawMessage,
) (backendprotocol.PlanLedgerResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.PlanLedgerResult{}, err
	}
	var query backendprotocol.PlanLedgerQuery
	if err := json.Unmarshal(raw, &query); err != nil {
		return backendprotocol.PlanLedgerResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	if query.PlanID == "" {
		return backendprotocol.PlanLedgerResult{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			"plan_id is required",
		)
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.PlanLedgerResult{}, err
	}
	defer func() { _ = store.Close() }()
	ledger, ok, err := store.GetLatestPlanLedger(query.PlanID)
	if err != nil {
		return backendprotocol.PlanLedgerResult{}, err
	}
	if !ok {
		return backendprotocol.PlanLedgerResult{}, backendprotocol.NewError(
			backendprotocol.ErrorNotFound,
			"plan ledger not found",
		)
	}
	return backendprotocol.PlanLedgerResult{Ledger: planLedgerToProtocol(ledger)}, nil
}

func planLedgerFromProtocol(ledger backendprotocol.PlanLedger) (state.PlanLedger, error) {
	recordedAt, err := parseOptionalTime(ledger.RecordedAt, "recorded_at")
	if err != nil {
		return state.PlanLedger{}, err
	}
	implementedAt, err := parseOptionalTime(ledger.ImplementedAt, "implemented_at")
	if err != nil {
		return state.PlanLedger{}, err
	}
	result := state.PlanLedger{
		SchemaVersion: ledger.SchemaVersion,
		LedgerID:      ledger.LedgerID,
		PlanID:        ledger.PlanID,
		Status:        ledger.Status,
		Hash:          ledger.Hash,
		RunID:         ledger.RunID,
		Commit:        ledger.Commit,
		RecordedAt:    recordedAt,
		ImplementedAt: implementedAt,
		Evidence:      make([]state.PlanEvidence, 0, len(ledger.Evidence)),
	}
	for _, evidence := range ledger.Evidence {
		result.Evidence = append(result.Evidence, state.PlanEvidence{
			ID:       evidence.ID,
			Kind:     evidence.Kind,
			Hash:     evidence.Hash,
			Content:  cloneAnyMap(evidence.Content),
			Metadata: cloneStringMap(evidence.Metadata),
		})
	}
	return result, nil
}

func planLedgerToProtocol(ledger state.PlanLedger) backendprotocol.PlanLedger {
	result := backendprotocol.PlanLedger{
		SchemaVersion: ledger.SchemaVersion,
		LedgerID:      ledger.LedgerID,
		PlanID:        ledger.PlanID,
		Status:        ledger.Status,
		Hash:          ledger.Hash,
		RunID:         ledger.RunID,
		Commit:        ledger.Commit,
		RecordedAt:    formatSQLiteTime(ledger.RecordedAt),
		ImplementedAt: formatSQLiteTime(ledger.ImplementedAt),
		Evidence:      make([]backendprotocol.PlanEvidence, 0, len(ledger.Evidence)),
	}
	if result.SchemaVersion == "" {
		result.SchemaVersion = "bach.plan_ledger.v1"
	}
	for _, evidence := range ledger.Evidence {
		result.Evidence = append(result.Evidence, backendprotocol.PlanEvidence{
			ID:       evidence.ID,
			Kind:     evidence.Kind,
			Hash:     evidence.Hash,
			Content:  cloneAnyMap(evidence.Content),
			Metadata: cloneStringMap(evidence.Metadata),
		})
	}
	return result
}

func formatSQLiteTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
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
