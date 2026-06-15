package backend

import (
	"context"
	"time"

	statestore "github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func (c FactoryQueueClient) GetTriggerCursor(
	ctx context.Context,
	factory string,
	trigger string,
) (FactoryTriggerCursor, error) {
	if !c.client.provider {
		return withStore(
			ctx,
			c.client.path,
			func(store *statestore.Store) (FactoryTriggerCursor, error) {
				return store.GetFactoryTriggerCursor(factory, trigger)
			},
		)
	}
	var result backendprotocol.FactoryGetTriggerCursorResult
	err := c.client.callProviderResult(
		ctx,
		"factory.getTriggerCursor",
		backendprotocol.FactoryGetTriggerCursorParams{Factory: factory, Trigger: trigger},
		&result,
	)
	if err != nil {
		return FactoryTriggerCursor{}, err
	}
	return factoryTriggerCursorFromProtocol(result.Cursor), nil
}

func (c FactoryQueueClient) RecordTriggerCursor(
	ctx context.Context,
	cursor FactoryTriggerCursor,
) (FactoryTriggerCursor, error) {
	if !c.client.provider {
		return withStore(
			ctx,
			c.client.path,
			func(store *statestore.Store) (FactoryTriggerCursor, error) {
				return store.RecordFactoryTriggerCursor(cursor)
			},
		)
	}
	var result backendprotocol.FactoryRecordTriggerCursorResult
	err := c.client.callProviderResult(
		ctx,
		"factory.recordTriggerCursor",
		backendprotocol.FactoryRecordTriggerCursorParams{
			Cursor: factoryTriggerCursorToProtocol(cursor),
		},
		&result,
	)
	if err != nil {
		return FactoryTriggerCursor{}, err
	}
	return factoryTriggerCursorFromProtocol(result.Cursor), nil
}

func factoryTriggerCursorToProtocol(
	cursor FactoryTriggerCursor,
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
) FactoryTriggerCursor {
	return FactoryTriggerCursor{
		Factory:    cursor.Factory,
		Trigger:    cursor.Trigger,
		Cursor:     cursor.Cursor,
		LastPollAt: mustParseFactoryTime(cursor.LastPollAt),
		LastAckAt:  mustParseFactoryTime(cursor.LastAckAt),
		LastNackAt: mustParseFactoryTime(cursor.LastNackAt),
		LastError:  cursor.LastError,
		UpdatedAt:  mustParseFactoryTime(cursor.UpdatedAt),
		Metadata:   cloneStringMap(cursor.Metadata),
	}
}

func mustParseFactoryTime(value string) time.Time {
	t, _ := parseFactoryTime(value, "")
	return t
}
