package state

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/model"
)

type FactoryWorkItem struct {
	ID                 string
	Factory            string
	Workflow           string
	Lifecycle          model.Lifecycle
	CurrentPhase       string
	Title              string
	Body               string
	BodyHash           string
	Priority           model.Priority
	Labels             []string
	SourceType         string
	DedupeKey          string
	SubmittedPlanPath  string
	SubmittedPlanHash  string
	IntakeEvidenceID   string
	IntakeEvidenceURI  string
	IntakeEvidenceHash string
	Metadata           map[string]string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CancelledAt        time.Time
	CancelReason       string
	ClaimedByDaemonID  string
	ClaimedAt          time.Time
	ClaimExpiresAt     time.Time
	CompletedAt        time.Time
	FailedAt           time.Time
	FailurePhase       string
	FailureMessage     string
	Attempts           []FactoryWorkItemAttempt
	Events             []FactoryWorkItemEvent
}

type FactoryWorkItemAttempt struct {
	ID                string
	WorkItemID        string
	AttemptNumber     int
	Status            model.Lifecycle
	StartPhase        string
	SubmittedPlanPath string
	SubmittedPlanHash string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	FinishedAt        time.Time
}

type FactoryWorkItemEvent struct {
	ID         string
	WorkItemID string
	AttemptID  string
	Type       string
	Message    string
	Metadata   map[string]string
	CreatedAt  time.Time
}

type FactoryDaemonLease struct {
	DaemonID   string
	Factory    string
	Hostname   string
	PID        int
	AcquiredAt time.Time
	RenewedAt  time.Time
	ExpiresAt  time.Time
	ReleasedAt time.Time
	Status     string
}

type FactoryWorkItemPhase struct {
	WorkItemID string
	AttemptID  string
	PhaseKey   string
	Status     string
	Target     string
	RunID      string
	PlanPath   string
	LedgerID   string
	Evidence   map[string]string
	StartedAt  time.Time
	FinishedAt time.Time
	UpdatedAt  time.Time
}

type FactoryDaemonStatus struct {
	Lease           FactoryDaemonLease
	ActiveItem      FactoryWorkItem
	HasActiveItem   bool
	LifecycleCounts map[model.Lifecycle]int
}

type FactoryWorkItemQuery struct {
	Factory string
	ID      string
	Status  string
}

type FactoryApproval struct {
	ID             string
	Factory        string
	Workflow       string
	WorkItemID     string
	AttemptID      string
	Phase          string
	PlanPath       string
	PlanHash       string
	ApprovedAt     time.Time
	Approver       string
	ApproverSource string
	Reason         string
	Metadata       map[string]string
}

type FactoryTriggerCursor struct {
	Factory    string
	Trigger    string
	Cursor     string
	LastPollAt time.Time
	LastAckAt  time.Time
	LastNackAt time.Time
	LastError  string
	UpdatedAt  time.Time
	Metadata   map[string]string
}

func (s *Store) GetFactoryWorkItem(factory string, id string) (FactoryWorkItem, bool, error) {
	if s.db == nil {
		return FactoryWorkItem{}, false, nil
	}
	item, err := getFactoryWorkItemDB(s.db, factory, id)
	if errors.Is(err, sql.ErrNoRows) {
		return FactoryWorkItem{}, false, nil
	}
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	return item, true, nil
}

func (s *Store) ListFactoryWorkItems(query FactoryWorkItemQuery) ([]FactoryWorkItem, error) {
	if s.db == nil {
		return []FactoryWorkItem{}, nil
	}
	clauses := []string{"factory = ?"}
	args := []any{query.Factory}
	if query.Status != "" && query.Status != "all" {
		clauses = append(clauses, "lifecycle = ?")
		args = append(args, query.Status)
	} else if query.Status == "" {
		clauses = append(clauses, "lifecycle IN ('pending', 'waiting_approval')")
	}
	rows, err := s.db.Query(`
		SELECT id
		FROM factory_work_items
		WHERE `+strings.Join(clauses, " AND ")+`
		ORDER BY created_at DESC, id DESC`, args...)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	items := make([]FactoryWorkItem, 0, len(ids))
	for _, id := range ids {
		item, err := getFactoryWorkItemDB(s.db, query.Factory, id)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (s *Store) CancelFactoryWorkItem(
	factory string,
	id string,
	reason string,
	cancelledAt time.Time,
	event FactoryWorkItemEvent,
) (FactoryWorkItem, bool, error) {
	if s.db == nil {
		return FactoryWorkItem{}, false, errReadOnlyStore()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	item, err := getFactoryWorkItemTx(tx, factory, id)
	if errors.Is(err, sql.ErrNoRows) {
		return FactoryWorkItem{}, false, nil
	}
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	if item.Lifecycle == model.LifecycleCancelled {
		return item, true, tx.Commit()
	}
	if item.Lifecycle != model.LifecyclePending {
		return FactoryWorkItem{}, false, fmt.Errorf("work item %q is not pending", id)
	}
	if _, err := tx.Exec(`
		UPDATE factory_work_items
		SET lifecycle = 'cancelled', updated_at = ?, cancelled_at = ?, cancel_reason = ?
		WHERE id = ? AND factory = ?
	`, formatTime(cancelledAt), formatTime(cancelledAt), reason, id, factory); err != nil {
		return FactoryWorkItem{}, false, err
	}
	if _, err := tx.Exec(`
		UPDATE factory_work_item_attempts
		SET status = 'cancelled', updated_at = ?, finished_at = ?
		WHERE work_item_id = ? AND status = 'pending'
	`, formatTime(cancelledAt), formatTime(cancelledAt), id); err != nil {
		return FactoryWorkItem{}, false, err
	}
	if event.ID != "" {
		event.WorkItemID = id
		if len(item.Attempts) > 0 {
			event.AttemptID = item.Attempts[0].ID
		}
		if err := insertFactoryWorkItemEvent(tx, event); err != nil {
			return FactoryWorkItem{}, false, err
		}
	}
	loaded, err := getFactoryWorkItemTx(tx, factory, id)
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	return loaded, true, tx.Commit()
}

func insertFactoryWorkItem(tx *sql.Tx, item FactoryWorkItem) error {
	labels, err := marshalJSONString(item.Labels, []string{})
	if err != nil {
		return err
	}
	metadata, err := marshalJSONString(item.Metadata, map[string]string{})
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO factory_work_items (
			id, factory, workflow, lifecycle, current_phase, title, body, body_hash,
			priority, labels, source_type, dedupe_key, submitted_plan_path,
			submitted_plan_hash, intake_evidence_id, intake_evidence_uri,
			intake_evidence_hash, metadata, created_at, updated_at, cancelled_at,
			cancel_reason, claimed_by_daemon_id, claimed_at, claim_expires_at,
			completed_at, failed_at, failure_phase, failure_message
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		item.ID,
		item.Factory,
		item.Workflow,
		item.Lifecycle,
		item.CurrentPhase,
		item.Title,
		item.Body,
		item.BodyHash,
		item.Priority,
		labels,
		item.SourceType,
		item.DedupeKey,
		item.SubmittedPlanPath,
		item.SubmittedPlanHash,
		item.IntakeEvidenceID,
		item.IntakeEvidenceURI,
		item.IntakeEvidenceHash,
		metadata,
		formatTime(item.CreatedAt),
		formatTime(item.UpdatedAt),
		formatTime(item.CancelledAt),
		item.CancelReason,
		item.ClaimedByDaemonID,
		formatTime(item.ClaimedAt),
		formatTime(item.ClaimExpiresAt),
		formatTime(item.CompletedAt),
		formatTime(item.FailedAt),
		item.FailurePhase,
		item.FailureMessage,
	)
	return err
}

func insertFactoryWorkItemAttempt(tx *sql.Tx, attempt FactoryWorkItemAttempt) error {
	_, err := tx.Exec(`
		INSERT INTO factory_work_item_attempts (
			id, work_item_id, attempt_number, status, start_phase, submitted_plan_path,
			submitted_plan_hash, created_at, updated_at, finished_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		attempt.ID,
		attempt.WorkItemID,
		attempt.AttemptNumber,
		attempt.Status,
		attempt.StartPhase,
		attempt.SubmittedPlanPath,
		attempt.SubmittedPlanHash,
		formatTime(attempt.CreatedAt),
		formatTime(attempt.UpdatedAt),
		formatTime(attempt.FinishedAt),
	)
	return err
}

func insertFactoryWorkItemEvent(tx *sql.Tx, event FactoryWorkItemEvent) error {
	metadata, err := marshalJSONString(event.Metadata, map[string]string{})
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO factory_work_item_events (
			id, work_item_id, attempt_id, type, message, metadata, created_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		event.ID,
		event.WorkItemID,
		event.AttemptID,
		event.Type,
		event.Message,
		metadata,
		formatTime(event.CreatedAt),
	)
	return err
}

func findOpenFactoryWorkItemByDedupe(
	tx *sql.Tx,
	factory string,
	workflow string,
	dedupeKey string,
) (FactoryWorkItem, bool, error) {
	var id string
	err := tx.QueryRow(`
		SELECT id
		FROM factory_work_items
		WHERE factory = ? AND workflow = ? AND dedupe_key = ? AND lifecycle = 'pending'
		ORDER BY created_at DESC
		LIMIT 1
	`, factory, workflow, dedupeKey).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return FactoryWorkItem{}, false, nil
	}
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	item, err := getFactoryWorkItemTx(tx, factory, id)
	return item, true, err
}
