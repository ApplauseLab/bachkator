package state

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestPlanLedgerPersistsLatestAndDetectsConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bach", "state.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	now := time.Now().UTC()
	ledger := PlanLedger{
		SchemaVersion: "bach.plan_ledger.v1",
		LedgerID:      "019ec300-0000-7000-8000-000000000001",
		PlanID:        "phase-4-plan-foundation",
		Status:        "implemented",
		Hash:          "sha256:one",
		RecordedAt:    now,
		ImplementedAt: now,
		Evidence: []PlanEvidence{{
			ID:       "019ec300-0000-7000-8000-000000000002",
			Kind:     "plan.implementation",
			Hash:     "sha256:evidence",
			Content:  map[string]any{"summary": "done"},
			Metadata: map[string]string{"agent_target": "agent/phase-4"},
		}},
	}
	if err := store.RecordPlanLedger(ledger); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordPlanLedger(ledger); err != nil {
		t.Fatalf("idempotent record failed: %v", err)
	}
	got, ok, err := store.GetLatestPlanLedger(ledger.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.LedgerID != ledger.LedgerID || len(got.Evidence) != 1 ||
		got.Evidence[0].Metadata["agent_target"] == "" {
		t.Fatalf("ledger = %#v ok=%v", got, ok)
	}
	changed := ledger
	changed.Hash = "sha256:two"
	if err := store.RecordPlanLedger(changed); !errors.Is(err, ErrPlanLedgerConflict) {
		t.Fatalf("conflict error = %v", err)
	}

	later := ledger
	later.LedgerID = "019ec300-0000-7000-8000-000000000003"
	later.Hash = "sha256:later"
	later.RecordedAt = now.Add(time.Second)
	later.Evidence = nil
	if err := store.RecordPlanLedger(later); err != nil {
		t.Fatal(err)
	}
	got, ok, err = store.GetLatestPlanLedger(ledger.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.LedgerID != later.LedgerID {
		t.Fatalf("latest ledger = %#v ok=%v", got, ok)
	}
}
