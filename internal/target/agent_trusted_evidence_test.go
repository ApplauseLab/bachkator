package target

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/state"
)

func TestValidateTrustedEvidenceUnchangedDetectsPolicyMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	before, err := trustedEvidenceSnapshot(root, "")
	if err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(
		root,
		".bach",
		"artifacts",
		"policies",
		"run-1",
		"agent-example.json",
	)
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`{"verdict":"passed"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := validateTrustedEvidenceUnchanged(root, "", before); err == nil {
		t.Fatal("validateTrustedEvidenceUnchanged() error = nil, want policy mutation error")
	}
}

func TestValidateTrustedEvidenceUnchangedDetectsPolicyArtifactMTimeMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	policyPath := filepath.Join(
		root,
		".bach",
		"artifacts",
		"policies",
		"run-1",
		"agent-example.json",
	)
	if err := os.MkdirAll(filepath.Dir(policyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(policyPath, []byte(`{"verdict":"passed"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := trustedEvidenceSnapshot(root, "")
	if err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(policyPath, later, later); err != nil {
		t.Fatal(err)
	}

	if err := validateTrustedEvidenceUnchanged(root, "", before); err == nil {
		t.Fatal("validateTrustedEvidenceUnchanged() error = nil, want mtime mutation error")
	}
}

func TestValidateTrustedEvidenceUnchangedIgnoresStateDBMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	before, err := trustedEvidenceSnapshot(root, "")
	if err != nil {
		t.Fatal(err)
	}
	writeRunState(t, root, "run-1", "shell/build")

	if err := validateTrustedEvidenceUnchanged(root, "", before); err != nil {
		t.Fatalf("validateTrustedEvidenceUnchanged() error = %v, want nil", err)
	}
}

func TestValidateTrustedEvidenceUnchangedDetectsTargetStateMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	before, err := trustedEvidenceSnapshot(root, "")
	if err != nil {
		t.Fatal(err)
	}
	writeTargetState(t, root, "shell/release", "forged-fingerprint")

	if err := validateTrustedEvidenceUnchanged(root, "", before); err == nil {
		t.Fatal("validateTrustedEvidenceUnchanged() error = nil, want target state mutation error")
	}
}

func TestValidateTrustedEvidenceUnchangedDetectsFactoryApprovalMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	before, err := trustedEvidenceSnapshot(root, "")
	if err != nil {
		t.Fatal(err)
	}
	writeFactoryApprovalEvidence(t, root)

	if err := validateTrustedEvidenceUnchanged(root, "", before); err == nil {
		t.Fatal("validateTrustedEvidenceUnchanged() error = nil, want approval mutation error")
	}
}

func TestValidateTrustedEvidenceUnchangedDetectsPolicyStateMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	before, err := trustedEvidenceSnapshot(root, "")
	if err != nil {
		t.Fatal(err)
	}
	writeRunState(t, root, "run-1", "policy/accept@agent.example")

	if err := validateTrustedEvidenceUnchanged(root, "", before); err == nil {
		t.Fatal("validateTrustedEvidenceUnchanged() error = nil, want policy state mutation error")
	}
}

func TestValidateTrustedEvidenceUnchangedDetectsContainingRunStateMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRunStateWithStatus(
		t,
		root,
		"run-1",
		"group/gate",
		model.RunStatusSuccess,
		map[string]model.RunStatus{"policy/accept@agent.example": model.RunStatusSuccess},
	)
	before, err := trustedEvidenceSnapshot(root, "")
	if err != nil {
		t.Fatal(err)
	}
	writeRunStateWithStatus(
		t,
		root,
		"run-1",
		"group/gate",
		model.RunStatusFailed,
		map[string]model.RunStatus{"policy/accept@agent.example": model.RunStatusSuccess},
	)

	if err := validateTrustedEvidenceUnchanged(root, "", before); err == nil {
		t.Fatal(
			"validateTrustedEvidenceUnchanged() error = nil, want containing run mutation error",
		)
	}
}

func TestValidateTrustedEvidenceUnchangedDetectsPolicyRunMetadataMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeRunStateWithLogDir(t, root, "run-1", "policy/accept@agent.example", "log-a")
	before, err := trustedEvidenceSnapshot(root, "")
	if err != nil {
		t.Fatal(err)
	}
	writeRunStateWithLogDir(t, root, "run-1", "policy/accept@agent.example", "log-b")

	if err := validateTrustedEvidenceUnchanged(root, "", before); err == nil {
		t.Fatal(
			"validateTrustedEvidenceUnchanged() error = nil, want policy metadata mutation error",
		)
	}
}

func TestValidateTrustedEvidenceUnchangedDetectsPlanLedgerMutation(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	before, err := trustedEvidenceSnapshot(root, "")
	if err != nil {
		t.Fatal(err)
	}
	writePlanLedgerEvidence(t, root, "implemented")

	if err := validateTrustedEvidenceUnchanged(root, "", before); err == nil {
		t.Fatal("validateTrustedEvidenceUnchanged() error = nil, want plan ledger mutation error")
	}
}

func TestValidateTrustedEvidenceUsesConfiguredStatePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	statePath := filepath.Join(root, "custom", "state.db")
	before, err := trustedEvidenceSnapshot(root, statePath)
	if err != nil {
		t.Fatal(err)
	}
	writePlanLedgerEvidenceAt(t, statePath, "implemented")

	if err := validateTrustedEvidenceUnchanged(root, statePath, before); err == nil {
		t.Fatal("validateTrustedEvidenceUnchanged() error = nil, want custom state mutation error")
	}
}

func writePolicyEvidenceState(t *testing.T, root string, policyTarget string) {
	t.Helper()
	writeRunState(t, root, "run-1", policyTarget)
}

func writeRunState(t *testing.T, root string, runID string, target string) {
	t.Helper()
	writeRunStateWithStatus(
		t,
		root,
		runID,
		target,
		model.RunStatusSuccess,
		map[string]model.RunStatus{target: model.RunStatusSuccess},
	)
}

func writeRunStateWithLogDir(
	t *testing.T,
	root string,
	runID string,
	target string,
	logDir string,
) {
	t.Helper()
	writeRunStateRecord(
		t,
		root,
		state.RunRecord{
			ID:         runID,
			Target:     target,
			Status:     model.RunStatusSuccess,
			StartedAt:  time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC),
			FinishedAt: time.Date(2026, 6, 13, 0, 0, 1, 0, time.UTC),
			LogDir:     logDir,
			Targets: map[string]state.TargetRunRecord{
				target: {
					Status:     model.RunStatusSuccess,
					StartedAt:  time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC),
					FinishedAt: time.Date(2026, 6, 13, 0, 0, 1, 0, time.UTC),
					LogPath:    logDir + "/target.log",
					Operation:  "policy fixture",
				},
			},
			Artifacts: []state.ArtifactRecord{{
				RunID:     runID,
				Target:    target,
				Kind:      "policy",
				Path:      logDir + "/policy.json",
				CreatedAt: time.Date(2026, 6, 13, 0, 0, 1, 0, time.UTC),
			}},
		},
	)
}

func writeRunStateWithStatus(
	t *testing.T,
	root string,
	runID string,
	target string,
	runStatus model.RunStatus,
	targetStatuses map[string]model.RunStatus,
) {
	t.Helper()
	startedAt := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	targetRuns := make(map[string]state.TargetRunRecord, len(targetStatuses))
	for targetName, targetStatus := range targetStatuses {
		targetRuns[targetName] = state.TargetRunRecord{
			Status:     targetStatus,
			StartedAt:  startedAt,
			FinishedAt: startedAt,
		}
	}
	run := state.RunRecord{
		ID:         runID,
		Target:     target,
		Status:     runStatus,
		StartedAt:  startedAt,
		FinishedAt: startedAt,
		Targets:    targetRuns,
	}
	writeRunStateRecord(t, root, run)
}

func writeRunStateRecord(t *testing.T, root string, run state.RunRecord) {
	t.Helper()
	store, err := state.NewStore(filepath.Join(root, ".bach", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if err := store.RecordRunCompletion(nil, run); err != nil {
		t.Fatal(err)
	}
}

func writePlanLedgerEvidence(t *testing.T, root string, status string) {
	t.Helper()
	writePlanLedgerEvidenceAt(t, filepath.Join(root, ".bach", "state.db"), status)
}

func writePlanLedgerEvidenceAt(t *testing.T, statePath string, status string) {
	t.Helper()
	store, err := state.NewStore(statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if err := store.RecordPlanLedger(state.PlanLedger{
		SchemaVersion: "bach.plan_ledger.v1",
		LedgerID:      "ledger-1",
		PlanID:        "plan-1",
		Status:        status,
		Hash:          "sha256:plan",
		RunID:         "run-1",
		Commit:        "abc123",
		RecordedAt:    time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC),
		ImplementedAt: time.Date(2026, 6, 13, 0, 0, 1, 0, time.UTC),
		Evidence: []state.PlanEvidence{{
			ID:   "evidence-1",
			Kind: "agent-report",
			Hash: "sha256:evidence",
			Content: map[string]any{
				"status": status,
			},
			Metadata: map[string]string{
				"source": "test",
			},
		}},
	}); err != nil {
		t.Fatal(err)
	}
}

func writeTargetState(t *testing.T, root string, target string, fingerprint string) {
	t.Helper()
	store, err := state.NewStore(filepath.Join(root, ".bach", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	if err := store.SaveTargetFingerprints(map[string]state.Record{
		target: {
			Fingerprint: fingerprint,
			FingerprintParts: map[string]string{
				"inputs": "forged-inputs",
			},
			CompletedAt: time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC),
		},
	}); err != nil {
		t.Fatal(err)
	}
}

func writeFactoryApprovalEvidence(t *testing.T, root string) {
	t.Helper()
	store, err := state.NewStore(filepath.Join(root, ".bach", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(root, ".bach", "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.Exec(`
		INSERT INTO factory_work_items (
			id, factory, workflow, lifecycle, current_phase, title, body_hash,
			priority, source_type, created_at, updated_at
		) VALUES (
			'item-1', 'factory', 'workflow', 'waiting_approval', 'plan', 'Title',
			'sha256:body', 'normal', 'manual', '2026-06-13T00:00:00Z',
			'2026-06-13T00:00:00Z'
		);
		INSERT INTO factory_work_item_attempts (
			id, work_item_id, attempt_number, status, start_phase, created_at, updated_at
		) VALUES (
			'attempt-1', 'item-1', 1, 'waiting_approval', 'plan',
			'2026-06-13T00:00:00Z', '2026-06-13T00:00:00Z'
		);
		INSERT INTO factory_work_item_approvals (
			id, factory, workflow, work_item_id, attempt_id, phase, plan_path,
			plan_hash, approved_at, approver, approver_source, reason
		) VALUES (
			'approval-1', 'factory', 'workflow', 'item-1', 'attempt-1', 'plan',
			'plans/factory/item-1.md', 'sha256:plan', '2026-06-13T00:00:01Z',
			'test', 'test', 'approved'
		);
	`); err != nil {
		t.Fatal(err)
	}
}
