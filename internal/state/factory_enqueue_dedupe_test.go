package state

import (
	"path/filepath"
	"testing"
	"time"
)

// A gated item (waiting_approval) must still match provider dedupe keys;
// otherwise every external-source touch spawns a duplicate work item while
// the item waits at an approval gate.
func TestEnqueueFactoryWorkItemDedupesWaitingApproval(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bach", "state.db")
	store, err := NewStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()

	now := time.Now().UTC()
	item := FactoryWorkItem{
		ID:                 "019ec200-0000-7000-8000-000000000011",
		Factory:            "sldc",
		Workflow:           "ship",
		Lifecycle:          "pending",
		CurrentPhase:       "plan",
		Title:              "Add billing webhook",
		Body:               "request body",
		BodyHash:           "sha256:body",
		Priority:           "high",
		SourceType:         "github_issue",
		DedupeKey:          "sldc|github|ship|github_issue|sldc/repo#42",
		IntakeEvidenceID:   "019ec200-0000-7000-8000-000000000012",
		IntakeEvidenceURI:  ".bach/artifacts/factory/019ec200-0000-7000-8000-000000000011/intake.json",
		IntakeEvidenceHash: "sha256:intake",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	attempt := FactoryWorkItemAttempt{
		ID:            "019ec200-0000-7000-8000-000000000013",
		WorkItemID:    item.ID,
		AttemptNumber: 1,
		Status:        "pending",
		StartPhase:    "plan",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	event := FactoryWorkItemEvent{
		ID:         "019ec200-0000-7000-8000-000000000014",
		WorkItemID: item.ID,
		AttemptID:  attempt.ID,
		Type:       "submitted",
		CreatedAt:  now,
	}

	_, wasCreated, err := store.EnqueueFactoryWorkItem(item, attempt, event, FactoryWorkItemEvent{})
	if err != nil || !wasCreated {
		t.Fatalf("initial enqueue: created=%v err=%v", wasCreated, err)
	}

	if _, err := store.db.Exec(
		`UPDATE factory_work_items SET lifecycle = 'waiting_approval' WHERE id = ?`,
		item.ID,
	); err != nil {
		t.Fatal(err)
	}

	duplicate := item
	duplicate.ID = "019ec200-0000-7000-8000-000000000015"
	duplicate.CreatedAt = now.Add(time.Minute)
	duplicate.UpdatedAt = now.Add(time.Minute)
	dupAttempt := attempt
	dupAttempt.ID = "019ec200-0000-7000-8000-000000000016"
	dupAttempt.WorkItemID = duplicate.ID
	dupEvent := event
	dupEvent.ID = "019ec200-0000-7000-8000-000000000017"
	dupEvent.WorkItemID = duplicate.ID
	dupEvent.AttemptID = dupAttempt.ID

	got, wasCreated, err := store.EnqueueFactoryWorkItem(
		duplicate, dupAttempt, dupEvent,
		FactoryWorkItemEvent{
			ID:        "019ec200-0000-7000-8000-000000000018",
			Type:      "deduped",
			CreatedAt: now.Add(time.Minute),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if wasCreated {
		t.Fatal("waiting_approval item was not deduped")
	}
	if got.ID != item.ID {
		t.Fatalf("dedupe returned %q, want %q", got.ID, item.ID)
	}
	if got.Lifecycle != "waiting_approval" {
		t.Fatalf("lifecycle = %q, want waiting_approval", got.Lifecycle)
	}
}
