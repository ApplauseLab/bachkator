package state

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

func (s *Store) GetFactoryTriggerCursor(
	factory string,
	trigger string,
) (FactoryTriggerCursor, error) {
	if s.db == nil {
		return FactoryTriggerCursor{}, errReadOnlyStore()
	}
	row := s.db.QueryRow(`
		SELECT factory, trigger, cursor, last_poll_at, last_ack_at, last_nack_at, last_error, updated_at, metadata
		FROM factory_trigger_cursors
		WHERE factory = ? AND trigger = ?
	`, factory, trigger)
	return scanFactoryTriggerCursor(row)
}

func (s *Store) RecordFactoryTriggerCursor(
	cursor FactoryTriggerCursor,
) (FactoryTriggerCursor, error) {
	if s.db == nil {
		return FactoryTriggerCursor{}, errReadOnlyStore()
	}
	if cursor.UpdatedAt.IsZero() {
		cursor.UpdatedAt = time.Now().UTC()
	}
	metadata, err := json.Marshal(cursor.Metadata)
	if err != nil {
		return FactoryTriggerCursor{}, fmt.Errorf("marshal metadata: %w", err)
	}
	_, err = s.db.Exec(`
		INSERT INTO factory_trigger_cursors (
			factory, trigger, cursor, last_poll_at, last_ack_at, last_nack_at, last_error, updated_at, metadata
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(factory, trigger) DO UPDATE SET
			cursor = excluded.cursor,
			last_poll_at = excluded.last_poll_at,
			last_ack_at = excluded.last_ack_at,
			last_nack_at = excluded.last_nack_at,
			last_error = excluded.last_error,
			updated_at = excluded.updated_at,
			metadata = excluded.metadata
	`, cursor.Factory, cursor.Trigger, cursor.Cursor,
		formatFactoryTime(cursor.LastPollAt), formatFactoryTime(cursor.LastAckAt),
		formatFactoryTime(cursor.LastNackAt), cursor.LastError,
		cursor.UpdatedAt.UTC().Format(time.RFC3339Nano), string(metadata),
	)
	if err != nil {
		return FactoryTriggerCursor{}, err
	}
	return s.GetFactoryTriggerCursor(cursor.Factory, cursor.Trigger)
}

func scanFactoryTriggerCursor(row *sql.Row) (FactoryTriggerCursor, error) {
	var cursor FactoryTriggerCursor
	var metadata string
	var lastPollAt, lastAckAt, lastNackAt, updatedAt string
	err := row.Scan(
		&cursor.Factory,
		&cursor.Trigger,
		&cursor.Cursor,
		&lastPollAt,
		&lastAckAt,
		&lastNackAt,
		&cursor.LastError,
		&updatedAt,
		&metadata,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return FactoryTriggerCursor{}, nil
		}
		return FactoryTriggerCursor{}, err
	}
	cursor.LastPollAt = parseFactoryTimeString(lastPollAt)
	cursor.LastAckAt = parseFactoryTimeString(lastAckAt)
	cursor.LastNackAt = parseFactoryTimeString(lastNackAt)
	cursor.UpdatedAt = parseFactoryTimeString(updatedAt)
	if metadata != "" {
		_ = json.Unmarshal([]byte(metadata), &cursor.Metadata)
	}
	return cursor, nil
}

func formatFactoryTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func parseFactoryTimeString(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return t
}
