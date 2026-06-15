package state

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/applauselab/bachkator/internal/bacherr"
	"github.com/applauselab/bachkator/internal/model"
)

func (s *Store) RecordFactoryApproval(
	approval FactoryApproval,
	event FactoryWorkItemEvent,
) (FactoryApproval, bool, error) {
	if s.db == nil {
		return FactoryApproval{}, false, errReadOnlyStore()
	}
	if approval.ID == "" || approval.WorkItemID == "" || approval.AttemptID == "" ||
		approval.Phase == "" {
		return FactoryApproval{}, false, fmt.Errorf(
			"id, work_item_id, attempt_id, and phase are required",
		)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return FactoryApproval{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	existing, ok, err := findFactoryApprovalTx(
		tx,
		approval.WorkItemID,
		approval.AttemptID,
		approval.Phase,
	)
	if err != nil {
		return FactoryApproval{}, false, err
	}
	if ok {
		return existing, true, nil
	}
	item, err := getFactoryWorkItemTx(tx, approval.Factory, approval.WorkItemID)
	if err != nil {
		return FactoryApproval{}, false, err
	}
	if item.Lifecycle != model.LifecycleWaitingApproval {
		return FactoryApproval{}, false, bacherr.ValidationFailedf(
			"work item %s is not waiting for approval",
			approval.WorkItemID,
		)
	}
	if item.CurrentPhase != approval.Phase {
		return FactoryApproval{}, false, bacherr.ValidationFailedf(
			"work item %s is waiting at phase %q, not %q",
			approval.WorkItemID,
			item.CurrentPhase,
			approval.Phase,
		)
	}
	if err := insertFactoryApproval(tx, approval); err != nil {
		return FactoryApproval{}, false, err
	}
	if event.ID != "" {
		if err := insertFactoryWorkItemEvent(tx, event); err != nil {
			return FactoryApproval{}, false, err
		}
	}
	loaded, err := getFactoryApprovalTx(tx, approval.ID)
	if err != nil {
		return FactoryApproval{}, false, err
	}
	return loaded, false, tx.Commit()
}

func (s *Store) ListFactoryWorkItemApprovals(workItemID string) ([]FactoryApproval, error) {
	if s.db == nil {
		return []FactoryApproval{}, nil
	}
	rows, err := s.db.Query(`
		SELECT id, factory, workflow, work_item_id, attempt_id, phase,
			plan_path, plan_hash, approved_at, approver, approver_source, reason, metadata
		FROM factory_work_item_approvals
		WHERE work_item_id = ?
		ORDER BY approved_at ASC, id ASC
	`, workItemID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var approvals []FactoryApproval
	for rows.Next() {
		approval, err := scanFactoryApproval(rows)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, approval)
	}
	return approvals, rows.Err()
}

func (s *Store) ListFactoryApprovalEvidence() ([]FactoryApproval, error) {
	if s.db == nil {
		return []FactoryApproval{}, nil
	}
	rows, err := s.db.Query(`
		SELECT id, factory, workflow, work_item_id, attempt_id, phase,
			plan_path, plan_hash, approved_at, approver, approver_source, reason, metadata
		FROM factory_work_item_approvals
		ORDER BY work_item_id ASC, attempt_id ASC, phase ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	approvals := []FactoryApproval{}
	for rows.Next() {
		approval, err := scanFactoryApproval(rows)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, approval)
	}
	return approvals, rows.Err()
}

func findFactoryApprovalTx(
	tx *sql.Tx,
	workItemID string,
	attemptID string,
	phase string,
) (FactoryApproval, bool, error) {
	approval, err := getFactoryApprovalTxByKey(tx, workItemID, attemptID, phase)
	if errors.Is(err, sql.ErrNoRows) {
		return FactoryApproval{}, false, nil
	}
	if err != nil {
		return FactoryApproval{}, false, err
	}
	return approval, true, nil
}

func insertFactoryApproval(tx *sql.Tx, approval FactoryApproval) error {
	metadata, err := marshalJSONString(approval.Metadata, map[string]string{})
	if err != nil {
		return err
	}
	_, err = tx.Exec(`
		INSERT INTO factory_work_item_approvals (
			id, factory, workflow, work_item_id, attempt_id, phase,
			plan_path, plan_hash, approved_at, approver, approver_source, reason, metadata
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, approval.ID, approval.Factory, approval.Workflow, approval.WorkItemID, approval.AttemptID,
		approval.Phase, approval.PlanPath, approval.PlanHash, formatTime(approval.ApprovedAt),
		approval.Approver, approval.ApproverSource, approval.Reason, metadata)
	return err
}

func getFactoryApprovalTx(tx *sql.Tx, id string) (FactoryApproval, error) {
	row := tx.QueryRow(`
		SELECT id, factory, workflow, work_item_id, attempt_id, phase,
			plan_path, plan_hash, approved_at, approver, approver_source, reason, metadata
		FROM factory_work_item_approvals
		WHERE id = ?
	`, id)
	return scanFactoryApproval(row)
}

func getFactoryApprovalTxByKey(
	tx *sql.Tx,
	workItemID string,
	attemptID string,
	phase string,
) (FactoryApproval, error) {
	row := tx.QueryRow(`
		SELECT id, factory, workflow, work_item_id, attempt_id, phase,
			plan_path, plan_hash, approved_at, approver, approver_source, reason, metadata
		FROM factory_work_item_approvals
		WHERE work_item_id = ? AND attempt_id = ? AND phase = ?
	`, workItemID, attemptID, phase)
	return scanFactoryApproval(row)
}

type approvalScanner interface {
	Scan(dest ...any) error
}

func scanFactoryApproval(row approvalScanner) (FactoryApproval, error) {
	var approval FactoryApproval
	var approvedAt string
	var metadata string
	if err := row.Scan(
		&approval.ID,
		&approval.Factory,
		&approval.Workflow,
		&approval.WorkItemID,
		&approval.AttemptID,
		&approval.Phase,
		&approval.PlanPath,
		&approval.PlanHash,
		&approvedAt,
		&approval.Approver,
		&approval.ApproverSource,
		&approval.Reason,
		&metadata,
	); err != nil {
		return FactoryApproval{}, err
	}
	approval.ApprovedAt = parseTime(approvedAt)
	approval.Metadata = unmarshalMetadata(metadata)
	return approval, nil
}

func unmarshalMetadata(value string) map[string]string {
	var out map[string]string
	if err := unmarshalJSONString(value, &out); err != nil || out == nil {
		return map[string]string{}
	}
	return out
}
