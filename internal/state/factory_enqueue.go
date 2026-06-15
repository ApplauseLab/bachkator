package state

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/model"
)

func (s *Store) EnqueueFactoryWorkItem(
	item FactoryWorkItem,
	attempt FactoryWorkItemAttempt,
	event FactoryWorkItemEvent,
	dedupeEvent FactoryWorkItemEvent,
) (FactoryWorkItem, bool, error) {
	if s.db == nil {
		return FactoryWorkItem{}, false, errReadOnlyStore()
	}
	tx, err := s.db.Begin()
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	if item.DedupeKey != "" {
		existing, ok, err := findOpenFactoryWorkItemByDedupe(
			tx,
			item.Factory,
			item.Workflow,
			item.DedupeKey,
		)
		if err != nil {
			return FactoryWorkItem{}, false, err
		}
		if ok {
			if dedupeEvent.ID != "" {
				dedupeEvent.WorkItemID = existing.ID
				if len(existing.Attempts) > 0 {
					dedupeEvent.AttemptID = existing.Attempts[0].ID
				}
				if err := insertFactoryWorkItemEvent(tx, dedupeEvent); err != nil {
					return FactoryWorkItem{}, false, err
				}
			}
			loaded, err := getFactoryWorkItemTx(tx, item.Factory, existing.ID)
			if err != nil {
				return FactoryWorkItem{}, false, err
			}
			return loaded, false, tx.Commit()
		}
	}
	if err := insertFactoryWorkItem(tx, item); err != nil {
		return FactoryWorkItem{}, false, err
	}
	if err := insertFactoryWorkItemAttempt(tx, attempt); err != nil {
		return FactoryWorkItem{}, false, err
	}
	if err := insertFactoryWorkItemEvent(tx, event); err != nil {
		return FactoryWorkItem{}, false, err
	}
	loaded, err := getFactoryWorkItemTx(tx, item.Factory, item.ID)
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	return loaded, true, tx.Commit()
}

func (s *Store) UpdatePendingFactoryWorkItem(
	item FactoryWorkItem,
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
	existing, ok, err := getFactoryWorkItemTxForUpdate(tx, item.Factory, item.ID)
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	if !ok || existing.Lifecycle != model.LifecyclePending {
		return FactoryWorkItem{}, false, nil
	}
	metadata, err := json.Marshal(item.Metadata)
	if err != nil {
		return FactoryWorkItem{}, false, fmt.Errorf("marshal metadata: %w", err)
	}
	_, err = tx.Exec(
		`
		UPDATE factory_work_items
		SET title = ?, body = ?, body_hash = ?, priority = ?, labels = ?, metadata = ?,
		    source_type = ?, dedupe_key = ?, submitted_plan_path = ?, submitted_plan_hash = ?,
		    intake_evidence_id = ?, intake_evidence_uri = ?, intake_evidence_hash = ?, updated_at = ?
		WHERE factory = ? AND id = ?
	`,
		item.Title,
		item.Body,
		item.BodyHash,
		item.Priority,
		strings.Join(item.Labels, ","),
		string(
			metadata,
		),
		item.SourceType,
		item.DedupeKey,
		item.SubmittedPlanPath,
		item.SubmittedPlanHash,
		item.IntakeEvidenceID,
		item.IntakeEvidenceURI,
		item.IntakeEvidenceHash,
		item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		item.Factory,
		item.ID,
	)
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	if event.ID != "" {
		if len(existing.Attempts) > 0 {
			event.AttemptID = existing.Attempts[0].ID
		}
		if err := insertFactoryWorkItemEvent(tx, event); err != nil {
			return FactoryWorkItem{}, false, err
		}
	}
	loaded, err := getFactoryWorkItemTx(tx, item.Factory, item.ID)
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	return loaded, true, tx.Commit()
}
