package state

import (
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"time"
)

var ErrPlanLedgerConflict = errors.New("plan ledger conflict")

type PlanLedger struct {
	SchemaVersion string
	LedgerID      string
	PlanID        string
	Status        string
	Hash          string
	RunID         string
	Commit        string
	RecordedAt    time.Time
	Evidence      []PlanEvidence
	ImplementedAt time.Time
}

type PlanEvidence struct {
	ID       string
	Kind     string
	Hash     string
	Content  map[string]any
	Metadata map[string]string
}

func (s *Store) RecordPlanLedger(ledger PlanLedger) error {
	if s.db == nil {
		return errReadOnlyStore()
	}
	if err := validatePlanLedgerForWrite(ledger); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	existing, ok, err := getPlanLedgerTx(tx, ledger.LedgerID)
	if err != nil {
		return err
	}
	if ok {
		if planLedgersEqual(existing, ledger) {
			return tx.Commit()
		}
		return ErrPlanLedgerConflict
	}
	if err := insertPlanLedger(tx, ledger); err != nil {
		return err
	}
	for _, evidence := range ledger.Evidence {
		if err := insertPlanEvidence(tx, ledger.LedgerID, evidence); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) GetLatestPlanLedger(planID string) (PlanLedger, bool, error) {
	if s.db == nil {
		return PlanLedger{}, false, nil
	}
	var ledgerID string
	err := s.db.QueryRow(`
		SELECT ledger_id
		FROM plan_ledgers
		WHERE plan_id = ?
		ORDER BY recorded_at DESC, ledger_id DESC
		LIMIT 1
	`, planID).Scan(&ledgerID)
	if errors.Is(err, sql.ErrNoRows) {
		return PlanLedger{}, false, nil
	}
	if err != nil {
		return PlanLedger{}, false, err
	}
	ledger, ok, err := getPlanLedgerDB(s.db, ledgerID)
	return ledger, ok, err
}

func (s *Store) ListPlanLedgerEvidence() ([]PlanLedger, error) {
	if s.db == nil {
		return []PlanLedger{}, nil
	}
	rows, err := s.db.Query(`
		SELECT ledger_id
		FROM plan_ledgers
		ORDER BY plan_id ASC, recorded_at ASC, ledger_id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	ledgerIDs := []string{}
	for rows.Next() {
		var ledgerID string
		if err := rows.Scan(&ledgerID); err != nil {
			return nil, err
		}
		ledgerIDs = append(ledgerIDs, ledgerID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ledgers := make([]PlanLedger, 0, len(ledgerIDs))
	for _, ledgerID := range ledgerIDs {
		ledger, ok, err := getPlanLedgerDB(s.db, ledgerID)
		if err != nil {
			return nil, err
		}
		if ok {
			ledgers = append(ledgers, ledger)
		}
	}
	return ledgers, nil
}

func insertPlanLedger(tx *sql.Tx, ledger PlanLedger) error {
	_, err := tx.Exec(`
		INSERT INTO plan_ledgers (
			ledger_id, plan_id, status, hash, run_id, commit_sha, recorded_at, implemented_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, ledger.LedgerID, ledger.PlanID, ledger.Status, ledger.Hash, ledger.RunID, ledger.Commit,
		formatTime(ledger.RecordedAt), formatTime(ledger.ImplementedAt))
	return err
}

func insertPlanEvidence(tx *sql.Tx, ledgerID string, evidence PlanEvidence) error {
	content, err := marshalJSONString(evidence.Content, map[string]any{})
	if err != nil {
		return err
	}
	metadata, err := marshalJSONString(evidence.Metadata, map[string]string{})
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO plan_evidence (id, ledger_id, kind, hash, content, metadata)
		VALUES (?, ?, ?, ?, ?, ?)
	`, evidence.ID, ledgerID, evidence.Kind, evidence.Hash, content, metadata)
	return err
}

func getPlanLedgerDB(db *sql.DB, ledgerID string) (PlanLedger, bool, error) {
	ledger, err := scanPlanLedger(db.QueryRow(`
		SELECT ledger_id, plan_id, status, hash, run_id, commit_sha, recorded_at, implemented_at
		FROM plan_ledgers
		WHERE ledger_id = ?
	`, ledgerID))
	if errors.Is(err, sql.ErrNoRows) {
		return PlanLedger{}, false, nil
	}
	if err != nil {
		return PlanLedger{}, false, err
	}
	evidence, err := listPlanEvidence(db, ledgerID)
	if err != nil {
		return PlanLedger{}, false, err
	}
	ledger.Evidence = evidence
	return ledger, true, nil
}

func getPlanLedgerTx(tx *sql.Tx, ledgerID string) (PlanLedger, bool, error) {
	ledger, err := scanPlanLedger(tx.QueryRow(`
		SELECT ledger_id, plan_id, status, hash, run_id, commit_sha, recorded_at, implemented_at
		FROM plan_ledgers
		WHERE ledger_id = ?
	`, ledgerID))
	if errors.Is(err, sql.ErrNoRows) {
		return PlanLedger{}, false, nil
	}
	if err != nil {
		return PlanLedger{}, false, err
	}
	evidence, err := listPlanEvidence(tx, ledgerID)
	if err != nil {
		return PlanLedger{}, false, err
	}
	ledger.Evidence = evidence
	return ledger, true, nil
}

func scanPlanLedger(row rowScanner) (PlanLedger, error) {
	var ledger PlanLedger
	var recordedAt string
	var implementedAt string
	if err := row.Scan(
		&ledger.LedgerID,
		&ledger.PlanID,
		&ledger.Status,
		&ledger.Hash,
		&ledger.RunID,
		&ledger.Commit,
		&recordedAt,
		&implementedAt,
	); err != nil {
		return PlanLedger{}, err
	}
	ledger.SchemaVersion = "bach.plan_ledger.v1"
	ledger.RecordedAt = parseTime(recordedAt)
	ledger.ImplementedAt = parseTime(implementedAt)
	return ledger, nil
}

func listPlanEvidence(db queryer, ledgerID string) ([]PlanEvidence, error) {
	rows, err := db.Query(`
		SELECT id, kind, hash, content, metadata
		FROM plan_evidence
		WHERE ledger_id = ?
		ORDER BY id ASC
	`, ledgerID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	evidence := []PlanEvidence{}
	for rows.Next() {
		var item PlanEvidence
		var content string
		var metadata string
		if err := rows.Scan(&item.ID, &item.Kind, &item.Hash, &content, &metadata); err != nil {
			return nil, err
		}
		if err := unmarshalJSONString(content, &item.Content); err != nil {
			return nil, err
		}
		if err := unmarshalJSONString(metadata, &item.Metadata); err != nil {
			return nil, err
		}
		evidence = append(evidence, item)
	}
	return evidence, rows.Err()
}

func planLedgersEqual(a, b PlanLedger) bool {
	normalizePlanLedger(&a)
	normalizePlanLedger(&b)
	if a.SchemaVersion != b.SchemaVersion ||
		a.LedgerID != b.LedgerID ||
		a.PlanID != b.PlanID ||
		a.Status != b.Status ||
		a.Hash != b.Hash ||
		a.RunID != b.RunID ||
		a.Commit != b.Commit ||
		!a.RecordedAt.Equal(b.RecordedAt) ||
		!a.ImplementedAt.Equal(b.ImplementedAt) ||
		len(a.Evidence) != len(b.Evidence) {
		return false
	}
	for i := range a.Evidence {
		if a.Evidence[i].ID != b.Evidence[i].ID ||
			a.Evidence[i].Kind != b.Evidence[i].Kind ||
			a.Evidence[i].Hash != b.Evidence[i].Hash ||
			!maps.Equal(a.Evidence[i].Content, b.Evidence[i].Content) ||
			!maps.Equal(a.Evidence[i].Metadata, b.Evidence[i].Metadata) {
			return false
		}
	}
	return true
}

func normalizePlanLedger(ledger *PlanLedger) {
	if ledger.SchemaVersion == "" {
		ledger.SchemaVersion = "bach.plan_ledger.v1"
	}
	if ledger.Evidence == nil {
		ledger.Evidence = []PlanEvidence{}
	}
	for i := range ledger.Evidence {
		if ledger.Evidence[i].Content == nil {
			ledger.Evidence[i].Content = map[string]any{}
		}
		if ledger.Evidence[i].Metadata == nil {
			ledger.Evidence[i].Metadata = map[string]string{}
		}
	}
}

func validatePlanLedgerForWrite(ledger PlanLedger) error {
	if ledger.LedgerID == "" || ledger.PlanID == "" {
		return fmt.Errorf("ledger_id and plan_id are required")
	}
	if ledger.SchemaVersion == "" || ledger.Status == "" || ledger.Hash == "" ||
		ledger.RecordedAt.IsZero() {
		return fmt.Errorf("schema_version, status, hash, and recorded_at are required")
	}
	return nil
}
