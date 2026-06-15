package backend

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/applauselab/bachkator/internal/model"
)

func TestNewClientLoadsEmptyState(t *testing.T) {
	client := NewClient(filepath.Join(t.TempDir(), ".bach", "state.db"))
	state, err := client.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if state == nil || state.Targets == nil {
		t.Fatalf("state = %#v", state)
	}
}

func TestProjectClientWritesRunThroughStdioProvider(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, ".bach", "state.db")
	client := NewProjectClient(root, path, model.Backend{
		Type:    "stdio",
		Command: []string{"bach", "backend", "sqlite"},
		Config:  map[string]string{"path": ".bach/state.db"},
	})
	run := RunRecord{
		ID:        "019ec166-0000-7000-8000-000000000001",
		Target:    "shell/test",
		Status:    "running",
		StartedAt: time.Now().UTC(),
		Targets:   map[string]TargetRunRecord{},
	}
	if err := client.Runs.Create(ctx, run); err != nil {
		t.Fatal(err)
	}
	state, err := client.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Runs) != 1 || state.Runs[0].ID != run.ID {
		t.Fatalf("runs = %#v", state.Runs)
	}
}

func TestProjectClientWritesFactoryQueueThroughStdioProvider(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, ".bach", "state.db")
	client := NewProjectClient(root, path, model.Backend{
		Type:    "stdio",
		Command: []string{"bach", "backend", "sqlite"},
		Config:  map[string]string{"path": ".bach/state.db"},
	})
	now := time.Now().UTC()
	item := FactoryWorkItem{
		ID:                 "019ec202-0000-7000-8000-000000000001",
		Factory:            "sldc",
		Workflow:           "ship",
		Lifecycle:          "pending",
		CurrentPhase:       "plan",
		Title:              "Ship billing webhook",
		Body:               "body",
		BodyHash:           "sha256:body",
		Priority:           "normal",
		Labels:             []string{"billing"},
		SourceType:         "manual",
		DedupeKey:          "billing-webhook",
		IntakeEvidenceID:   "019ec202-0000-7000-8000-000000000002",
		IntakeEvidenceURI:  ".bach/artifacts/factory/019ec202-0000-7000-8000-000000000001/intake.json",
		IntakeEvidenceHash: "sha256:intake",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	attempt := FactoryWorkItemAttempt{
		ID:            "019ec202-0000-7000-8000-000000000003",
		WorkItemID:    item.ID,
		AttemptNumber: 1,
		Status:        "pending",
		StartPhase:    "plan",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	event := FactoryWorkItemEvent{
		ID:         "019ec202-0000-7000-8000-000000000004",
		WorkItemID: item.ID,
		AttemptID:  attempt.ID,
		Type:       "submitted",
		CreatedAt:  now,
	}
	created, ok, err := client.Factory.Enqueue(ctx, item, attempt, event, FactoryWorkItemEvent{})
	if err != nil {
		t.Fatal(err)
	}
	if !ok || created.ID != item.ID || len(created.Attempts) != 1 {
		t.Fatalf("created = %#v ok=%v", created, ok)
	}
	duplicate := item
	duplicate.ID = "019ec202-0000-7000-8000-000000000005"
	deduped, ok, err := client.Factory.Enqueue(ctx, duplicate, attempt, event, FactoryWorkItemEvent{
		ID:        "019ec202-0000-7000-8000-000000000006",
		Type:      "deduped",
		CreatedAt: now.Add(time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok || deduped.ID != item.ID || len(deduped.Events) != 2 {
		t.Fatalf("deduped = %#v ok=%v", deduped, ok)
	}
	items, err := client.Factory.List(ctx, FactoryWorkItemQuery{Factory: "sldc", Status: "pending"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != item.ID {
		t.Fatalf("items = %#v", items)
	}
	found, ok, err := client.Factory.Get(ctx, "sldc", item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || found.ID != item.ID {
		t.Fatalf("found = %#v ok=%v", found, ok)
	}
	cancelled, ok, err := client.Factory.Cancel(
		ctx,
		"sldc",
		item.ID,
		"duplicate",
		now,
		FactoryWorkItemEvent{
			ID:        "019ec202-0000-7000-8000-000000000007",
			Type:      "cancelled",
			CreatedAt: now,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || cancelled.Lifecycle != "cancelled" || cancelled.CancelReason != "duplicate" {
		t.Fatalf("cancelled = %#v ok=%v", cancelled, ok)
	}
}

func TestProjectClientReadsPlanLedgerThroughStdioProvider(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	path := filepath.Join(root, ".bach", "state.db")
	client := NewProjectClient(root, path, model.Backend{
		Type:    "stdio",
		Command: []string{"bach", "backend", "sqlite"},
		Config:  map[string]string{"path": ".bach/state.db"},
	})
	now := time.Now().UTC()
	ledger := PlanLedger{
		SchemaVersion: "bach.plan_ledger.v1",
		LedgerID:      "019ec301-0000-7000-8000-000000000001",
		PlanID:        "phase-4-plan-foundation",
		Status:        "implemented",
		Hash:          "sha256:plan",
		RecordedAt:    now,
		ImplementedAt: now,
		Evidence: []PlanEvidence{{
			ID:   "019ec301-0000-7000-8000-000000000002",
			Kind: "plan.implementation",
		}},
	}
	if err := client.Plans.Record(ctx, ledger); err != nil {
		t.Fatal(err)
	}
	got, ok, err := client.Plans.Get(ctx, ledger.PlanID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || got.LedgerID != ledger.LedgerID || len(got.Evidence) != 1 {
		t.Fatalf("ledger = %#v ok=%v", got, ok)
	}
	_, ok, err = client.Plans.Get(ctx, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("missing ledger reported as found")
	}
}
