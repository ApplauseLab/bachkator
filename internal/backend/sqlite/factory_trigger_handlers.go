package sqlite

import (
	"encoding/json"
	"time"

	"github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func (p *Provider) getFactoryTriggerCursor(
	raw json.RawMessage,
) (backendprotocol.FactoryGetTriggerCursorResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryGetTriggerCursorResult{}, err
	}
	var params backendprotocol.FactoryGetTriggerCursorParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return backendprotocol.FactoryGetTriggerCursorResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryGetTriggerCursorResult{}, err
	}
	defer func() { _ = store.Close() }()
	cursor, err := store.GetFactoryTriggerCursor(params.Factory, params.Trigger)
	if err != nil {
		return backendprotocol.FactoryGetTriggerCursorResult{}, err
	}
	return backendprotocol.FactoryGetTriggerCursorResult{
		Cursor: factoryTriggerCursorToProtocol(cursor),
	}, nil
}

func (p *Provider) recordFactoryTriggerCursor(
	raw json.RawMessage,
) (backendprotocol.FactoryRecordTriggerCursorResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryRecordTriggerCursorResult{}, err
	}
	var params backendprotocol.FactoryRecordTriggerCursorParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return backendprotocol.FactoryRecordTriggerCursorResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryRecordTriggerCursorResult{}, err
	}
	defer func() { _ = store.Close() }()
	cursor := factoryTriggerCursorFromProtocol(params.Cursor)
	if cursor.UpdatedAt.IsZero() {
		cursor.UpdatedAt = p.now()
	}
	recorded, err := store.RecordFactoryTriggerCursor(cursor)
	if err != nil {
		return backendprotocol.FactoryRecordTriggerCursorResult{}, err
	}
	return backendprotocol.FactoryRecordTriggerCursorResult{
		Cursor: factoryTriggerCursorToProtocol(recorded),
	}, nil
}

func factoryTriggerCursorToProtocol(
	cursor state.FactoryTriggerCursor,
) backendprotocol.FactoryTriggerCursor {
	return backendprotocol.FactoryTriggerCursor{
		Factory:    cursor.Factory,
		Trigger:    cursor.Trigger,
		Cursor:     cursor.Cursor,
		LastPollAt: cursor.LastPollAt.UTC().Format(time.RFC3339Nano),
		LastAckAt:  cursor.LastAckAt.UTC().Format(time.RFC3339Nano),
		LastNackAt: cursor.LastNackAt.UTC().Format(time.RFC3339Nano),
		LastError:  cursor.LastError,
		UpdatedAt:  cursor.UpdatedAt.UTC().Format(time.RFC3339Nano),
		Metadata:   cloneStringMap(cursor.Metadata),
	}
}

func factoryTriggerCursorFromProtocol(
	cursor backendprotocol.FactoryTriggerCursor,
) state.FactoryTriggerCursor {
	return state.FactoryTriggerCursor{
		Factory:    cursor.Factory,
		Trigger:    cursor.Trigger,
		Cursor:     cursor.Cursor,
		LastPollAt: parseRFC3339(cursor.LastPollAt),
		LastAckAt:  parseRFC3339(cursor.LastAckAt),
		LastNackAt: parseRFC3339(cursor.LastNackAt),
		LastError:  cursor.LastError,
		UpdatedAt:  parseRFC3339(cursor.UpdatedAt),
		Metadata:   cloneStringMap(cursor.Metadata),
	}
}

func parseRFC3339(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}
	}
	return t
}
