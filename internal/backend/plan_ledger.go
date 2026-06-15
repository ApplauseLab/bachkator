package backend

import (
	"context"
	"time"

	statestore "github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

type PlanLedger = statestore.PlanLedger
type PlanEvidence = statestore.PlanEvidence

func (c PlanLedgerClient) Record(ctx context.Context, ledger PlanLedger) error {
	if !c.client.provider {
		_, err := withStore(ctx, c.client.path, func(store *statestore.Store) (struct{}, error) {
			return struct{}{}, store.RecordPlanLedger(ledger)
		})
		return err
	}
	return c.client.callProvider(ctx, "plans.recordLedger", planLedgerToProtocol(ledger))
}

func (c PlanLedgerClient) Get(ctx context.Context, planID string) (PlanLedger, bool, error) {
	if !c.client.provider {
		store, err := statestore.NewStore(c.client.path)
		if err != nil {
			return PlanLedger{}, false, err
		}
		defer func() { _ = store.Close() }()
		return store.GetLatestPlanLedger(planID)
	}
	var result backendprotocol.PlanLedgerResult
	err := c.client.callProviderResult(
		ctx,
		"plans.getLedger",
		backendprotocol.PlanLedgerQuery{PlanID: planID},
		&result,
	)
	if isBackendNotFound(err) {
		return PlanLedger{}, false, nil
	}
	if err != nil {
		return PlanLedger{}, false, err
	}
	ledger, err := planLedgerFromProtocol(result.Ledger)
	return ledger, true, err
}

func planLedgerToProtocol(ledger PlanLedger) backendprotocol.PlanLedger {
	result := backendprotocol.PlanLedger{
		SchemaVersion: ledger.SchemaVersion,
		LedgerID:      ledger.LedgerID,
		PlanID:        ledger.PlanID,
		Status:        ledger.Status,
		Hash:          ledger.Hash,
		RunID:         ledger.RunID,
		Commit:        ledger.Commit,
		RecordedAt:    formatBackendTime(ledger.RecordedAt),
		Evidence:      make([]backendprotocol.PlanEvidence, 0, len(ledger.Evidence)),
		ImplementedAt: formatBackendTime(ledger.ImplementedAt),
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

func planLedgerFromProtocol(ledger backendprotocol.PlanLedger) (PlanLedger, error) {
	recordedAt, err := parseFactoryTime(ledger.RecordedAt, "recorded_at")
	if err != nil {
		return PlanLedger{}, err
	}
	implementedAt, err := parseFactoryTime(ledger.ImplementedAt, "implemented_at")
	if err != nil {
		return PlanLedger{}, err
	}
	result := PlanLedger{
		SchemaVersion: ledger.SchemaVersion,
		LedgerID:      ledger.LedgerID,
		PlanID:        ledger.PlanID,
		Status:        ledger.Status,
		Hash:          ledger.Hash,
		RunID:         ledger.RunID,
		Commit:        ledger.Commit,
		RecordedAt:    recordedAt,
		ImplementedAt: implementedAt,
		Evidence:      make([]PlanEvidence, 0, len(ledger.Evidence)),
	}
	for _, evidence := range ledger.Evidence {
		result.Evidence = append(result.Evidence, PlanEvidence{
			ID:       evidence.ID,
			Kind:     evidence.Kind,
			Hash:     evidence.Hash,
			Content:  cloneAnyMap(evidence.Content),
			Metadata: cloneStringMap(evidence.Metadata),
		})
	}
	return result, nil
}

func formatBackendTime(value time.Time) string {
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
