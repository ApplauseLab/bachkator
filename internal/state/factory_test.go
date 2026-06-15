package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFactoryWorkItemQueuePersistsDedupeAndCancel(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bach", "state.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	now := time.Now().UTC()
	item := FactoryWorkItem{
		ID:                 "019ec200-0000-7000-8000-000000000001",
		Factory:            "sldc",
		Workflow:           "ship",
		Lifecycle:          "pending",
		CurrentPhase:       "plan",
		Title:              "Add billing webhook",
		Body:               "request body",
		BodyHash:           "sha256:body",
		Priority:           "high",
		Labels:             []string{"billing", "webhook"},
		SourceType:         "manual",
		DedupeKey:          "billing-webhook",
		SubmittedPlanPath:  "plans/billing.md",
		SubmittedPlanHash:  "sha256:plan",
		IntakeEvidenceID:   "019ec200-0000-7000-8000-000000000002",
		IntakeEvidenceURI:  ".bach/artifacts/factory/019ec200-0000-7000-8000-000000000001/intake.json",
		IntakeEvidenceHash: "sha256:intake",
		Metadata:           map[string]string{"source": "test"},
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	attempt := FactoryWorkItemAttempt{
		ID:                "019ec200-0000-7000-8000-000000000003",
		WorkItemID:        item.ID,
		AttemptNumber:     1,
		Status:            "pending",
		StartPhase:        "plan",
		SubmittedPlanPath: item.SubmittedPlanPath,
		SubmittedPlanHash: item.SubmittedPlanHash,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	event := FactoryWorkItemEvent{
		ID:         "019ec200-0000-7000-8000-000000000004",
		WorkItemID: item.ID,
		AttemptID:  attempt.ID,
		Type:       "submitted",
		Message:    "submitted manually",
		CreatedAt:  now,
	}
	created, wasCreated, err := store.EnqueueFactoryWorkItem(
		item,
		attempt,
		event,
		FactoryWorkItemEvent{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !wasCreated || created.ID != item.ID || len(created.Attempts) != 1 ||
		len(created.Events) != 1 {
		t.Fatalf("created = %#v wasCreated=%v", created, wasCreated)
	}

	dedupeEvent := FactoryWorkItemEvent{
		ID:        "019ec200-0000-7000-8000-000000000005",
		Type:      "deduped",
		Message:   "matched existing work item",
		CreatedAt: now.Add(time.Second),
	}
	duplicate := item
	duplicate.ID = "019ec200-0000-7000-8000-000000000006"
	got, wasCreated, err := store.EnqueueFactoryWorkItem(
		duplicate,
		attempt,
		event,
		dedupeEvent,
	)
	if err != nil {
		t.Fatal(err)
	}
	if wasCreated || got.ID != item.ID || len(got.Events) != 2 {
		t.Fatalf("dedupe item = %#v wasCreated=%v", got, wasCreated)
	}

	listedStore, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listedStore.Close() }()
	listed, err := listedStore.ListFactoryWorkItems(FactoryWorkItemQuery{
		Factory: "sldc",
		Status:  "pending",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].Labels[0] != "billing" ||
		listed[0].Metadata["source"] != "test" {
		t.Fatalf("listed = %#v", listed)
	}

	cancelled, ok, err := store.CancelFactoryWorkItem(
		"sldc",
		item.ID,
		"duplicate",
		now.Add(2*time.Second),
		FactoryWorkItemEvent{
			ID:        "019ec200-0000-7000-8000-000000000007",
			Type:      "cancelled",
			Message:   "cancelled manually",
			CreatedAt: now.Add(2 * time.Second),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || cancelled.Lifecycle != "cancelled" || cancelled.Attempts[0].Status != "cancelled" {
		t.Fatalf("cancelled = %#v ok=%v", cancelled, ok)
	}
	pending, err := store.ListFactoryWorkItems(
		FactoryWorkItemQuery{Factory: "sldc", Status: "pending"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("pending after cancel = %#v", pending)
	}
	cancelledAgain, ok, err := store.CancelFactoryWorkItem(
		"sldc",
		item.ID,
		"duplicate",
		now.Add(3*time.Second),
		FactoryWorkItemEvent{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || cancelledAgain.Lifecycle != "cancelled" {
		t.Fatalf("cancelledAgain = %#v ok=%v", cancelledAgain, ok)
	}
}
