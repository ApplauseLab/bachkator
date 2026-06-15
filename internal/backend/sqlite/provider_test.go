package sqlite

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func TestProviderInitializeCreatesDatabase(t *testing.T) {
	root := t.TempDir()
	params := backendprotocol.InitializeParams{
		Protocol:    backendprotocol.ProtocolVersion,
		ProjectName: "example",
		ProjectRoot: root,
		Config:      map[string]string{"path": ".bach/state.db"},
	}
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	provider := &Provider{}
	result, err := provider.initialize(raw)
	if err != nil {
		t.Fatal(err)
	}
	if result.Protocol != backendprotocol.ProtocolVersion {
		t.Fatalf("protocol = %q", result.Protocol)
	}
	if !hasCapability(result.Capabilities, backendprotocol.CapabilityFactoryQueue) {
		t.Fatalf("capabilities = %#v, want factory queue", result.Capabilities)
	}
	if !provider.initialized {
		t.Fatal("provider not initialized")
	}
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if provider.storePath != filepath.Join(rootReal, ".bach", "state.db") {
		t.Fatalf("storePath = %q", provider.storePath)
	}
}

func TestProviderInitializeRejectsAbsolutePath(t *testing.T) {
	params := backendprotocol.InitializeParams{
		Protocol:    backendprotocol.ProtocolVersion,
		ProjectName: "example",
		ProjectRoot: t.TempDir(),
		Config:      map[string]string{"path": "/tmp/state.db"},
	}
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	_, err = (&Provider{}).initialize(raw)
	if err == nil {
		t.Fatal("expected absolute path error")
	}
}

func TestProviderInitializeRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	_, err := (&Provider{}).initialize(mustJSON(t, backendprotocol.InitializeParams{
		Protocol:    backendprotocol.ProtocolVersion,
		ProjectName: "example",
		ProjectRoot: root,
		Config:      map[string]string{"path": "link/state.db"},
	}))
	if err == nil {
		t.Fatal("expected symlink escape error")
	}
}

func TestProviderRunCreateGetList(t *testing.T) {
	provider := initializedProvider(t)
	startedAt := time.Now().UTC().Format(time.RFC3339Nano)
	run := backendprotocol.RunRecord{
		ID:        "019ec144-0000-7000-8000-000000000001",
		Target:    "shell/test",
		Status:    "running",
		StartedAt: startedAt,
		LogDir:    ".bach/runs/019ec144-0000-7000-8000-000000000001",
	}
	if _, err := provider.createRun(mustJSON(t, run)); err != nil {
		t.Fatal(err)
	}
	got, err := provider.getRun(mustJSON(t, backendprotocol.RunQuery{ID: run.ID}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Run.ID != run.ID || got.Run.Target != run.Target {
		t.Fatalf("run = %#v", got.Run)
	}
	list, err := provider.listRuns(mustJSON(t, backendprotocol.RunQuery{Target: "shell/test"}))
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Runs) != 1 || list.Runs[0].ID != run.ID {
		t.Fatalf("runs = %#v", list.Runs)
	}
}

func TestProviderFindingCurrentAndEvents(t *testing.T) {
	provider := initializedProvider(t)
	finding := backendprotocol.FindingObservation{
		ID:          "019ec144-0000-7000-8000-000000000002",
		SourceType:  "lint",
		SourceID:    "shell/lint",
		Severity:    backendprotocol.FindingWarning,
		Category:    "gofmt",
		Message:     "file is not formatted",
		Fingerprint: "lint:main.go:1",
		ObservedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		Location:    &backendprotocol.FindingLocation{Path: "main.go", StartLine: 1},
	}
	if _, err := provider.recordFindingObservation(mustJSON(t, finding)); err != nil {
		t.Fatal(err)
	}
	current, err := provider.listCurrentFindings(
		mustJSON(t, backendprotocol.FindingQuery{Status: "open"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(current.Findings) != 1 || current.Findings[0].Fingerprint != finding.Fingerprint {
		t.Fatalf("current findings = %#v", current.Findings)
	}
	events, err := provider.listFindingEvents(
		mustJSON(t, backendprotocol.FindingQuery{Fingerprint: finding.Fingerprint}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(events.Findings) != 1 || events.Findings[0].ID != finding.ID {
		t.Fatalf("finding events = %#v", events.Findings)
	}
}

func TestProviderFactoryQueueLifecycle(t *testing.T) {
	provider := initializedProvider(t)
	now := time.Now().UTC()
	item := backendprotocol.FactoryWorkItem{
		ID:                 "019ec201-0000-7000-8000-000000000001",
		Factory:            "sldc",
		Workflow:           "ship",
		Lifecycle:          "pending",
		CurrentPhase:       "plan",
		Title:              "Add billing webhook",
		Body:               "body",
		BodyHash:           "sha256:body",
		Priority:           "normal",
		Labels:             []string{"billing"},
		SourceType:         "manual",
		DedupeKey:          "billing-webhook",
		IntakeEvidenceID:   "019ec201-0000-7000-8000-000000000002",
		IntakeEvidenceURI:  ".bach/artifacts/factory/019ec201-0000-7000-8000-000000000001/intake.json",
		IntakeEvidenceHash: "sha256:intake",
		CreatedAt:          now.Format(time.RFC3339Nano),
		UpdatedAt:          now.Format(time.RFC3339Nano),
	}
	attempt := backendprotocol.FactoryWorkItemAttempt{
		ID:            "019ec201-0000-7000-8000-000000000003",
		WorkItemID:    item.ID,
		AttemptNumber: 1,
		Status:        "pending",
		StartPhase:    "plan",
		CreatedAt:     now.Format(time.RFC3339Nano),
		UpdatedAt:     now.Format(time.RFC3339Nano),
	}
	event := backendprotocol.FactoryWorkItemEvent{
		ID:         "019ec201-0000-7000-8000-000000000004",
		WorkItemID: item.ID,
		AttemptID:  attempt.ID,
		Type:       "submitted",
		CreatedAt:  now.Format(time.RFC3339Nano),
	}
	created, err := provider.enqueueFactoryWorkItem(mustJSON(
		t,
		backendprotocol.FactoryEnqueueWorkItemParams{
			Item:    item,
			Attempt: attempt,
			Event:   event,
		},
	))
	if err != nil {
		t.Fatal(err)
	}
	if !created.Created || created.Item.ID != item.ID || len(created.Item.Attempts) != 1 {
		t.Fatalf("created = %#v", created)
	}
	duplicate := item
	duplicate.ID = "019ec201-0000-7000-8000-000000000005"
	deduped, err := provider.enqueueFactoryWorkItem(mustJSON(
		t,
		backendprotocol.FactoryEnqueueWorkItemParams{
			Item:    duplicate,
			Attempt: attempt,
			Event:   event,
			DedupeEvent: backendprotocol.FactoryWorkItemEvent{
				ID:        "019ec201-0000-7000-8000-000000000006",
				Type:      "deduped",
				CreatedAt: now.Add(time.Second).Format(time.RFC3339Nano),
			},
		},
	))
	if err != nil {
		t.Fatal(err)
	}
	if deduped.Created || deduped.Item.ID != item.ID || len(deduped.Item.Events) != 2 {
		t.Fatalf("deduped = %#v", deduped)
	}
	listed, err := provider.listFactoryWorkItems(mustJSON(
		t,
		backendprotocol.FactoryWorkItemQuery{Factory: "sldc", Status: "pending"},
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].ID != item.ID {
		t.Fatalf("listed = %#v", listed.Items)
	}
	cancelled, err := provider.cancelFactoryWorkItem(mustJSON(
		t,
		backendprotocol.FactoryCancelWorkItemParams{
			Factory:     "sldc",
			ID:          item.ID,
			Reason:      "duplicate",
			CancelledAt: now.Add(2 * time.Second).Format(time.RFC3339Nano),
			Event: backendprotocol.FactoryWorkItemEvent{
				ID:        "019ec201-0000-7000-8000-000000000007",
				Type:      "cancelled",
				CreatedAt: now.Add(2 * time.Second).Format(time.RFC3339Nano),
			},
		},
	))
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Item.Lifecycle != "cancelled" || cancelled.Item.CancelReason != "duplicate" {
		t.Fatalf("cancelled = %#v", cancelled.Item)
	}
}

func TestProviderPlanLedgerRecordAndGet(t *testing.T) {
	provider := initializedProvider(t)
	now := time.Now().UTC()
	ledger := backendprotocol.PlanLedger{
		SchemaVersion: "bach.plan_ledger.v1",
		LedgerID:      "019ec302-0000-7000-8000-000000000001",
		PlanID:        "phase-4-plan-foundation",
		Status:        "implemented",
		Hash:          "sha256:plan",
		RecordedAt:    now.Format(time.RFC3339Nano),
		ImplementedAt: now.Format(time.RFC3339Nano),
		Evidence: []backendprotocol.PlanEvidence{{
			ID:       "019ec302-0000-7000-8000-000000000002",
			Kind:     "plan.implementation",
			Content:  map[string]any{"summary": "done"},
			Metadata: map[string]string{"agent_target": "agent/phase-4"},
		}},
	}
	if _, err := provider.recordPlanLedger(mustJSON(t, ledger)); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.recordPlanLedger(mustJSON(t, ledger)); err != nil {
		t.Fatalf("idempotent record failed: %v", err)
	}
	got, err := provider.getPlanLedger(mustJSON(
		t,
		backendprotocol.PlanLedgerQuery{PlanID: ledger.PlanID},
	))
	if err != nil {
		t.Fatal(err)
	}
	if got.Ledger.LedgerID != ledger.LedgerID || len(got.Ledger.Evidence) != 1 {
		t.Fatalf("ledger = %#v", got.Ledger)
	}
	changed := ledger
	changed.Hash = "sha256:changed"
	if _, err := provider.recordPlanLedger(mustJSON(t, changed)); err == nil {
		t.Fatal("expected conflict")
	}
	if _, err := provider.getPlanLedger(mustJSON(
		t,
		backendprotocol.PlanLedgerQuery{PlanID: "missing"},
	)); err == nil {
		t.Fatal("expected not found")
	}
}

func TestProviderRejectsRunBeforeInitialize(t *testing.T) {
	_, err := (&Provider{}).listRuns(mustJSON(t, backendprotocol.RunQuery{}))
	if err == nil {
		t.Fatal("expected not initialized error")
	}
}

func initializedProvider(t *testing.T) *Provider {
	t.Helper()
	provider := &Provider{}
	_, err := provider.initialize(mustJSON(t, backendprotocol.InitializeParams{
		Protocol:    backendprotocol.ProtocolVersion,
		ProjectName: "example",
		ProjectRoot: t.TempDir(),
		Config:      map[string]string{"path": ".bach/state.db"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func hasCapability(
	capabilities []backendprotocol.Capability,
	want backendprotocol.Capability,
) bool {
	for _, capability := range capabilities {
		if capability == want {
			return true
		}
	}
	return false
}

func mustJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
