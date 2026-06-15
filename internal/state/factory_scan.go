package state

import (
	"database/sql"
	"encoding/json"
	"errors"
)

func getFactoryWorkItemDB(db *sql.DB, factory string, id string) (FactoryWorkItem, error) {
	item, err := scanFactoryWorkItem(db.QueryRow(`
		SELECT id, factory, workflow, lifecycle, current_phase, title, body, body_hash,
			priority, labels, source_type, dedupe_key, submitted_plan_path,
			submitted_plan_hash, intake_evidence_id, intake_evidence_uri,
			intake_evidence_hash, metadata, created_at, updated_at, cancelled_at,
			cancel_reason, claimed_by_daemon_id, claimed_at, claim_expires_at,
			completed_at, failed_at, failure_phase, failure_message
		FROM factory_work_items
		WHERE factory = ? AND id = ?
	`, factory, id))
	if err != nil {
		return FactoryWorkItem{}, err
	}
	return hydrateFactoryWorkItem(db, item)
}

func getFactoryWorkItemTx(tx *sql.Tx, factory string, id string) (FactoryWorkItem, error) {
	item, err := scanFactoryWorkItem(tx.QueryRow(`
		SELECT id, factory, workflow, lifecycle, current_phase, title, body, body_hash,
			priority, labels, source_type, dedupe_key, submitted_plan_path,
			submitted_plan_hash, intake_evidence_id, intake_evidence_uri,
			intake_evidence_hash, metadata, created_at, updated_at, cancelled_at,
			cancel_reason, claimed_by_daemon_id, claimed_at, claim_expires_at,
			completed_at, failed_at, failure_phase, failure_message
		FROM factory_work_items
		WHERE factory = ? AND id = ?
	`, factory, id))
	if err != nil {
		return FactoryWorkItem{}, err
	}
	return hydrateFactoryWorkItem(tx, item)
}

func getFactoryWorkItemTxForUpdate(
	tx *sql.Tx,
	factory string,
	id string,
) (FactoryWorkItem, bool, error) {
	item, err := scanFactoryWorkItem(tx.QueryRow(`
		SELECT id, factory, workflow, lifecycle, current_phase, title, body, body_hash,
			priority, labels, source_type, dedupe_key, submitted_plan_path,
			submitted_plan_hash, intake_evidence_id, intake_evidence_uri,
			intake_evidence_hash, metadata, created_at, updated_at, cancelled_at,
			cancel_reason, claimed_by_daemon_id, claimed_at, claim_expires_at,
			completed_at, failed_at, failure_phase, failure_message
		FROM factory_work_items
		WHERE factory = ? AND id = ?
	`, factory, id))
	if errors.Is(err, sql.ErrNoRows) {
		return FactoryWorkItem{}, false, nil
	}
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	hydrated, err := hydrateFactoryWorkItem(tx, item)
	return hydrated, true, err
}

type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanFactoryWorkItem(row rowScanner) (FactoryWorkItem, error) {
	var item FactoryWorkItem
	var labels string
	var metadata string
	var createdAt string
	var updatedAt string
	var cancelledAt string
	var claimedAt string
	var claimExpiresAt string
	var completedAt string
	var failedAt string
	if err := row.Scan(
		&item.ID,
		&item.Factory,
		&item.Workflow,
		&item.Lifecycle,
		&item.CurrentPhase,
		&item.Title,
		&item.Body,
		&item.BodyHash,
		&item.Priority,
		&labels,
		&item.SourceType,
		&item.DedupeKey,
		&item.SubmittedPlanPath,
		&item.SubmittedPlanHash,
		&item.IntakeEvidenceID,
		&item.IntakeEvidenceURI,
		&item.IntakeEvidenceHash,
		&metadata,
		&createdAt,
		&updatedAt,
		&cancelledAt,
		&item.CancelReason,
		&item.ClaimedByDaemonID,
		&claimedAt,
		&claimExpiresAt,
		&completedAt,
		&failedAt,
		&item.FailurePhase,
		&item.FailureMessage,
	); err != nil {
		return FactoryWorkItem{}, err
	}
	if labels != "" {
		if err := json.Unmarshal([]byte(labels), &item.Labels); err != nil {
			return FactoryWorkItem{}, err
		}
	}
	if metadata != "" {
		if err := json.Unmarshal([]byte(metadata), &item.Metadata); err != nil {
			return FactoryWorkItem{}, err
		}
	}
	item.CreatedAt = parseTime(createdAt)
	item.UpdatedAt = parseTime(updatedAt)
	item.CancelledAt = parseTime(cancelledAt)
	item.ClaimedAt = parseTime(claimedAt)
	item.ClaimExpiresAt = parseTime(claimExpiresAt)
	item.CompletedAt = parseTime(completedAt)
	item.FailedAt = parseTime(failedAt)
	return item, nil
}

func hydrateFactoryWorkItem(db queryer, item FactoryWorkItem) (FactoryWorkItem, error) {
	attempts, err := listFactoryWorkItemAttempts(db, item.ID)
	if err != nil {
		return FactoryWorkItem{}, err
	}
	events, err := listFactoryWorkItemEvents(db, item.ID)
	if err != nil {
		return FactoryWorkItem{}, err
	}
	item.Attempts = attempts
	item.Events = events
	return item, nil
}

func listFactoryWorkItemAttempts(db queryer, workItemID string) ([]FactoryWorkItemAttempt, error) {
	rows, err := db.Query(`
		SELECT id, work_item_id, attempt_number, status, start_phase,
			submitted_plan_path, submitted_plan_hash, created_at, updated_at, finished_at
		FROM factory_work_item_attempts
		WHERE work_item_id = ?
		ORDER BY attempt_number ASC
	`, workItemID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	attempts := []FactoryWorkItemAttempt{}
	for rows.Next() {
		var attempt FactoryWorkItemAttempt
		var createdAt string
		var updatedAt string
		var finishedAt string
		if err := rows.Scan(
			&attempt.ID,
			&attempt.WorkItemID,
			&attempt.AttemptNumber,
			&attempt.Status,
			&attempt.StartPhase,
			&attempt.SubmittedPlanPath,
			&attempt.SubmittedPlanHash,
			&createdAt,
			&updatedAt,
			&finishedAt,
		); err != nil {
			return nil, err
		}
		attempt.CreatedAt = parseTime(createdAt)
		attempt.UpdatedAt = parseTime(updatedAt)
		attempt.FinishedAt = parseTime(finishedAt)
		attempts = append(attempts, attempt)
	}
	return attempts, rows.Err()
}

func listFactoryWorkItemEvents(db queryer, workItemID string) ([]FactoryWorkItemEvent, error) {
	rows, err := db.Query(`
		SELECT id, work_item_id, attempt_id, type, message, metadata, created_at
		FROM factory_work_item_events
		WHERE work_item_id = ?
		ORDER BY created_at ASC, id ASC
	`, workItemID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	events := []FactoryWorkItemEvent{}
	for rows.Next() {
		var event FactoryWorkItemEvent
		var metadata string
		var createdAt string
		if err := rows.Scan(
			&event.ID,
			&event.WorkItemID,
			&event.AttemptID,
			&event.Type,
			&event.Message,
			&metadata,
			&createdAt,
		); err != nil {
			return nil, err
		}
		if metadata != "" {
			if err := json.Unmarshal([]byte(metadata), &event.Metadata); err != nil {
				return nil, err
			}
		}
		event.CreatedAt = parseTime(createdAt)
		events = append(events, event)
	}
	return events, rows.Err()
}

func marshalJSONString[T any](value T, fallback T) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		data, err = json.Marshal(fallback)
	}
	return string(data), err
}

func unmarshalJSONString[T any](value string, out *T) error {
	if value == "" {
		value = "{}"
	}
	return json.Unmarshal([]byte(value), out)
}
