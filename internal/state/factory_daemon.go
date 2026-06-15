package state

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/model"
)

func (s *Store) AcquireFactoryDaemonLease(
	lease FactoryDaemonLease,
) (FactoryDaemonLease, error) {
	if s.db == nil {
		return FactoryDaemonLease{}, errReadOnlyStore()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return FactoryDaemonLease{}, err
	}
	defer func() { _ = tx.Rollback() }()
	if lease.DaemonID == "" || lease.Factory == "" || lease.ExpiresAt.IsZero() {
		return FactoryDaemonLease{}, fmt.Errorf("daemon_id, factory, and expires_at are required")
	}
	now := lease.RenewedAt
	if now.IsZero() {
		now = clock.SystemNow()
	}
	if err := expireFactoryDaemonLeases(tx, lease.Factory, now); err != nil {
		return FactoryDaemonLease{}, err
	}
	var activeID string
	err = tx.QueryRow(`
		SELECT daemon_id
		FROM factory_daemon_leases
		WHERE factory = ? AND status = 'active' AND expires_at > ?
		ORDER BY renewed_at DESC, daemon_id DESC
		LIMIT 1
	`, lease.Factory, formatTime(now)).Scan(&activeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return FactoryDaemonLease{}, err
	}
	if activeID != "" && activeID != lease.DaemonID {
		return FactoryDaemonLease{}, fmt.Errorf(
			"factory %q is already leased by daemon %q",
			lease.Factory,
			activeID,
		)
	}
	lease.Status = "active"
	if lease.AcquiredAt.IsZero() {
		lease.AcquiredAt = now
	}
	lease.RenewedAt = now
	if _, err := tx.Exec(`
		INSERT INTO factory_daemon_leases (
			daemon_id, factory, hostname, pid, acquired_at, renewed_at, expires_at,
			released_at, status
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, '', 'active')
		ON CONFLICT(daemon_id) DO UPDATE SET
			factory = excluded.factory,
			hostname = excluded.hostname,
			pid = excluded.pid,
			renewed_at = excluded.renewed_at,
			expires_at = excluded.expires_at,
			released_at = '',
			status = 'active'
	`, lease.DaemonID, lease.Factory, lease.Hostname, lease.PID, formatTime(lease.AcquiredAt), formatTime(lease.RenewedAt), formatTime(lease.ExpiresAt)); err != nil {
		return FactoryDaemonLease{}, err
	}
	loaded, err := getFactoryDaemonLeaseTx(tx, lease.DaemonID)
	if err != nil {
		return FactoryDaemonLease{}, err
	}
	return loaded, tx.Commit()
}

func (s *Store) RenewFactoryDaemonLease(
	daemonID string,
	renewedAt time.Time,
	expiresAt time.Time,
) (FactoryDaemonLease, bool, error) {
	if s.db == nil {
		return FactoryDaemonLease{}, false, errReadOnlyStore()
	}
	result, err := s.db.Exec(`
		UPDATE factory_daemon_leases
		SET renewed_at = ?, expires_at = ?, status = 'active'
		WHERE daemon_id = ? AND status = 'active'
	`, formatTime(renewedAt), formatTime(expiresAt), daemonID)
	if err != nil {
		return FactoryDaemonLease{}, false, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return FactoryDaemonLease{}, false, err
	}
	if changed == 0 {
		return FactoryDaemonLease{}, false, nil
	}
	lease, err := getFactoryDaemonLeaseDB(s.db, daemonID)
	return lease, true, err
}

func (s *Store) ReleaseFactoryDaemonLease(
	daemonID string,
	releasedAt time.Time,
) (FactoryDaemonLease, bool, error) {
	if s.db == nil {
		return FactoryDaemonLease{}, false, errReadOnlyStore()
	}
	result, err := s.db.Exec(`
		UPDATE factory_daemon_leases
		SET released_at = ?, status = 'released'
		WHERE daemon_id = ? AND status = 'active'
	`, formatTime(releasedAt), daemonID)
	if err != nil {
		return FactoryDaemonLease{}, false, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return FactoryDaemonLease{}, false, err
	}
	if changed == 0 {
		return FactoryDaemonLease{}, false, nil
	}
	lease, err := getFactoryDaemonLeaseDB(s.db, daemonID)
	return lease, true, err
}

func (s *Store) ClaimNextFactoryWorkItem(
	factory string,
	daemonID string,
	claimedAt time.Time,
	expiresAt time.Time,
) (FactoryWorkItem, bool, error) {
	if s.db == nil {
		return FactoryWorkItem{}, false, errReadOnlyStore()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	var id string
	err = tx.QueryRow(`
		SELECT id
		FROM factory_work_items
		WHERE factory = ? AND (
				lifecycle = 'pending'
				OR (
					lifecycle = 'waiting_approval'
					AND EXISTS (
						SELECT 1 FROM factory_work_item_approvals
						WHERE factory_work_item_approvals.work_item_id = factory_work_items.id
							AND factory_work_item_approvals.phase = factory_work_items.current_phase
							AND factory_work_item_approvals.attempt_id = (
								SELECT id FROM factory_work_item_attempts
								WHERE work_item_id = factory_work_items.id
								ORDER BY attempt_number DESC, id DESC
								LIMIT 1
							)
					)
				)
			)
			AND EXISTS (
				SELECT 1 FROM factory_daemon_leases
				WHERE daemon_id = ? AND factory = factory_work_items.factory
					AND status = 'active' AND expires_at > ?
			)
		ORDER BY CASE priority
			WHEN 'urgent' THEN 0
			WHEN 'high' THEN 1
			WHEN 'normal' THEN 2
			WHEN 'low' THEN 3
			ELSE 4
		END ASC, created_at ASC, id ASC
		LIMIT 1
	`, factory, daemonID, formatTime(claimedAt)).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return FactoryWorkItem{}, false, nil
	}
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	result, err := tx.Exec(`
		UPDATE factory_work_items
		SET lifecycle = 'claimed', claimed_by_daemon_id = ?,
			claimed_at = ?, claim_expires_at = ?, updated_at = ?
		WHERE id = ? AND factory = ? AND lifecycle IN ('pending', 'waiting_approval')
	`, daemonID, formatTime(claimedAt), formatTime(expiresAt), formatTime(claimedAt), id, factory)
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	if changed == 0 {
		return FactoryWorkItem{}, false, nil
	}
	item, err := getFactoryWorkItemTx(tx, factory, id)
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	return item, true, tx.Commit()
}

func (s *Store) UpdateFactoryWorkItemPhase(phase FactoryWorkItemPhase) error {
	if phase.WorkItemID == "" || phase.AttemptID == "" || phase.PhaseKey == "" ||
		phase.Status == "" {
		return fmt.Errorf("work_item_id, attempt_id, phase_key, and status are required")
	}
	if s.db == nil {
		return errReadOnlyStore()
	}
	evidence, err := marshalJSONString(phase.Evidence, map[string]string{})
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.Exec(`
		INSERT INTO factory_work_item_phases (
			work_item_id, attempt_id, phase_key, status, target, run_id, plan_path,
			ledger_id, evidence, started_at, finished_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(work_item_id, attempt_id, phase_key) DO UPDATE SET
			status = excluded.status,
			target = excluded.target,
			run_id = excluded.run_id,
			plan_path = excluded.plan_path,
			ledger_id = excluded.ledger_id,
			evidence = excluded.evidence,
			started_at = COALESCE(NULLIF(factory_work_item_phases.started_at, ''), excluded.started_at),
			finished_at = excluded.finished_at,
			updated_at = excluded.updated_at
	`, phase.WorkItemID, phase.AttemptID, phase.PhaseKey, phase.Status, phase.Target, phase.RunID, phase.PlanPath, phase.LedgerID, evidence, formatTime(phase.StartedAt), formatTime(phase.FinishedAt), formatTime(phase.UpdatedAt))
	if err != nil {
		return err
	}
	if phase.Status == "running" {
		if _, err := tx.Exec(`
			UPDATE factory_work_items
			SET lifecycle = 'running', current_phase = ?, updated_at = ?
			WHERE id = ?
		`, phase.PhaseKey, formatTime(phase.UpdatedAt), phase.WorkItemID); err != nil {
			return err
		}
	}
	if phase.Status == "waiting_approval" {
		if _, err := tx.Exec(`
			UPDATE factory_work_items
			SET lifecycle = 'waiting_approval', current_phase = ?,
				claimed_by_daemon_id = '', claimed_at = '', claim_expires_at = '', updated_at = ?
			WHERE id = ?
		`, phase.PhaseKey, formatTime(phase.UpdatedAt), phase.WorkItemID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) CompleteFactoryWorkItem(
	factory string,
	id string,
	completedAt time.Time,
) (FactoryWorkItem, bool, error) {
	return s.finishFactoryWorkItem(factory, id, model.LifecycleCompleted, "", "", completedAt)
}

func (s *Store) FailFactoryWorkItem(
	factory string,
	id string,
	phase string,
	message string,
	failedAt time.Time,
) (FactoryWorkItem, bool, error) {
	return s.finishFactoryWorkItem(factory, id, model.LifecycleFailed, phase, message, failedAt)
}

func (s *Store) GetFactoryDaemonStatus(
	factory string,
	now time.Time,
) (FactoryDaemonStatus, error) {
	if s.db == nil {
		return FactoryDaemonStatus{LifecycleCounts: map[model.Lifecycle]int{}}, nil
	}
	status := FactoryDaemonStatus{LifecycleCounts: map[model.Lifecycle]int{}}
	lease, ok, err := getActiveFactoryDaemonLease(s.db, factory, now)
	if err != nil {
		return FactoryDaemonStatus{}, err
	}
	if ok {
		status.Lease = lease
	}
	rows, err := s.db.Query(`
		SELECT lifecycle, COUNT(*)
		FROM factory_work_items
		WHERE factory = ?
		GROUP BY lifecycle
	`, factory)
	if err != nil {
		return FactoryDaemonStatus{}, err
	}
	for rows.Next() {
		var lifecycle string
		var count int
		if err := rows.Scan(&lifecycle, &count); err != nil {
			_ = rows.Close()
			return FactoryDaemonStatus{}, err
		}
		status.LifecycleCounts[model.Lifecycle(lifecycle)] = count
	}
	if err := rows.Close(); err != nil {
		return FactoryDaemonStatus{}, err
	}
	var activeID string
	err = s.db.QueryRow(`
		SELECT id
		FROM factory_work_items
		WHERE factory = ? AND lifecycle IN ('claimed', 'running')
		ORDER BY updated_at DESC, id DESC
		LIMIT 1
	`, factory).Scan(&activeID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return FactoryDaemonStatus{}, err
	}
	if activeID != "" {
		item, err := getFactoryWorkItemDB(s.db, factory, activeID)
		if err != nil {
			return FactoryDaemonStatus{}, err
		}
		status.ActiveItem = item
		status.HasActiveItem = true
	}
	return status, rows.Err()
}

func (s *Store) finishFactoryWorkItem(
	factory string,
	id string,
	lifecycle model.Lifecycle,
	failurePhase string,
	failureMessage string,
	finishedAt time.Time,
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
	completedAt := ""
	failedAt := ""
	if lifecycle == model.LifecycleCompleted {
		completedAt = formatTime(finishedAt)
	} else {
		failedAt = formatTime(finishedAt)
	}
	if _, err := tx.Exec(`
		UPDATE factory_work_items
		SET lifecycle = ?, current_phase = '', updated_at = ?, completed_at = ?, failed_at = ?,
			failure_phase = ?, failure_message = ?
		WHERE id = ? AND factory = ?
	`, lifecycle, formatTime(finishedAt), completedAt, failedAt, failurePhase, failureMessage, id, factory); err != nil {
		return FactoryWorkItem{}, false, err
	}
	if len(item.Attempts) > 0 {
		if _, err := tx.Exec(`
			UPDATE factory_work_item_attempts
			SET status = ?, updated_at = ?, finished_at = ?
			WHERE id = ?
		`, lifecycle, formatTime(finishedAt), formatTime(finishedAt), item.Attempts[0].ID); err != nil {
			return FactoryWorkItem{}, false, err
		}
	}
	loaded, err := getFactoryWorkItemTx(tx, factory, id)
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	return loaded, true, tx.Commit()
}

func expireFactoryDaemonLeases(tx *sql.Tx, factory string, now time.Time) error {
	_, err := tx.Exec(`
		UPDATE factory_daemon_leases
		SET status = 'expired'
		WHERE factory = ? AND status = 'active' AND expires_at <= ?
	`, factory, formatTime(now))
	return err
}

func getFactoryDaemonLeaseTx(tx *sql.Tx, daemonID string) (FactoryDaemonLease, error) {
	return scanFactoryDaemonLease(tx.QueryRow(`
		SELECT daemon_id, factory, hostname, pid, acquired_at, renewed_at, expires_at,
			released_at, status
		FROM factory_daemon_leases
		WHERE daemon_id = ?
	`, daemonID))
}

func getFactoryDaemonLeaseDB(db *sql.DB, daemonID string) (FactoryDaemonLease, error) {
	return scanFactoryDaemonLease(db.QueryRow(`
		SELECT daemon_id, factory, hostname, pid, acquired_at, renewed_at, expires_at,
			released_at, status
		FROM factory_daemon_leases
		WHERE daemon_id = ?
	`, daemonID))
}

func getActiveFactoryDaemonLease(
	db *sql.DB,
	factory string,
	now time.Time,
) (FactoryDaemonLease, bool, error) {
	lease, err := scanFactoryDaemonLease(db.QueryRow(`
		SELECT daemon_id, factory, hostname, pid, acquired_at, renewed_at, expires_at,
			released_at, status
		FROM factory_daemon_leases
		WHERE factory = ? AND status = 'active' AND expires_at > ?
		ORDER BY renewed_at DESC, daemon_id DESC
		LIMIT 1
	`, factory, formatTime(now)))
	if errors.Is(err, sql.ErrNoRows) {
		return FactoryDaemonLease{}, false, nil
	}
	return lease, err == nil, err
}

func scanFactoryDaemonLease(row rowScanner) (FactoryDaemonLease, error) {
	var lease FactoryDaemonLease
	var acquiredAt string
	var renewedAt string
	var expiresAt string
	var releasedAt string
	if err := row.Scan(
		&lease.DaemonID,
		&lease.Factory,
		&lease.Hostname,
		&lease.PID,
		&acquiredAt,
		&renewedAt,
		&expiresAt,
		&releasedAt,
		&lease.Status,
	); err != nil {
		return FactoryDaemonLease{}, err
	}
	lease.AcquiredAt = parseTime(acquiredAt)
	lease.RenewedAt = parseTime(renewedAt)
	lease.ExpiresAt = parseTime(expiresAt)
	lease.ReleasedAt = parseTime(releasedAt)
	return lease, nil
}
