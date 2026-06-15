package backend

import (
	"context"
	"errors"
	"time"

	"github.com/applauselab/bachkator/internal/model"
	statestore "github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func (c FactoryQueueClient) Enqueue(
	ctx context.Context,
	item FactoryWorkItem,
	attempt FactoryWorkItemAttempt,
	event FactoryWorkItemEvent,
	dedupeEvent FactoryWorkItemEvent,
) (FactoryWorkItem, bool, error) {
	if !c.client.provider {
		store, err := statestore.NewStore(c.client.path)
		if err != nil {
			return FactoryWorkItem{}, false, err
		}
		defer func() { _ = store.Close() }()
		return store.EnqueueFactoryWorkItem(
			item,
			attempt,
			event,
			dedupeEvent,
		)
	}
	var result backendprotocol.FactoryWorkItemResult
	err := c.client.callProviderResult(
		ctx,
		"factory.enqueueWorkItem",
		backendprotocol.FactoryEnqueueWorkItemParams{
			Item:        factoryWorkItemToProtocol(item),
			Attempt:     factoryWorkItemAttemptToProtocol(attempt),
			Event:       factoryWorkItemEventToProtocol(event),
			DedupeEvent: factoryWorkItemEventToProtocol(dedupeEvent),
		},
		&result,
	)
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	createdItem, err := factoryWorkItemFromProtocol(result.Item)
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	return createdItem, result.Created, nil
}

func (c FactoryQueueClient) UpdatePending(
	ctx context.Context,
	item FactoryWorkItem,
	event FactoryWorkItemEvent,
) (FactoryWorkItem, bool, error) {
	if !c.client.provider {
		store, err := statestore.NewStore(c.client.path)
		if err != nil {
			return FactoryWorkItem{}, false, err
		}
		defer func() { _ = store.Close() }()
		return store.UpdatePendingFactoryWorkItem(item, event)
	}
	var result backendprotocol.FactoryWorkItemResult
	err := c.client.callProviderResult(
		ctx,
		"factory.updatePendingWorkItem",
		backendprotocol.FactoryEnqueueWorkItemParams{
			Item:  factoryWorkItemToProtocol(item),
			Event: factoryWorkItemEventToProtocol(event),
		},
		&result,
	)
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	updated, err := factoryWorkItemFromProtocol(result.Item)
	return updated, result.Created, err
}

func (c FactoryQueueClient) Get(
	ctx context.Context,
	factory string,
	id string,
) (FactoryWorkItem, bool, error) {
	if !c.client.provider {
		store, err := statestore.NewStore(c.client.path)
		if err != nil {
			return FactoryWorkItem{}, false, err
		}
		defer func() { _ = store.Close() }()
		return store.GetFactoryWorkItem(factory, id)
	}
	var result backendprotocol.FactoryWorkItemResult
	err := c.client.callProviderResult(
		ctx,
		"factory.getWorkItem",
		backendprotocol.FactoryWorkItemQuery{Factory: factory, ID: id},
		&result,
	)
	if isBackendNotFound(err) {
		return FactoryWorkItem{}, false, nil
	}
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	item, err := factoryWorkItemFromProtocol(result.Item)
	return item, true, err
}

func (c FactoryQueueClient) List(
	ctx context.Context,
	query FactoryWorkItemQuery,
) ([]FactoryWorkItem, error) {
	if !c.client.provider {
		return withStore(
			ctx,
			c.client.path,
			func(store *statestore.Store) ([]FactoryWorkItem, error) {
				return store.ListFactoryWorkItems(query)
			},
		)
	}
	var result backendprotocol.FactoryWorkItemListResult
	if err := c.client.callProviderResult(
		ctx,
		"factory.listWorkItems",
		backendprotocol.FactoryWorkItemQuery{
			Factory: query.Factory,
			ID:      query.ID,
			Status:  model.Lifecycle(query.Status),
		},
		&result,
	); err != nil {
		return nil, err
	}
	items := make([]FactoryWorkItem, 0, len(result.Items))
	for _, protocolItem := range result.Items {
		item, err := factoryWorkItemFromProtocol(protocolItem)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (c FactoryQueueClient) Cancel(
	ctx context.Context,
	factory string,
	id string,
	reason string,
	cancelledAt time.Time,
	event FactoryWorkItemEvent,
) (FactoryWorkItem, bool, error) {
	if !c.client.provider {
		store, err := statestore.NewStore(c.client.path)
		if err != nil {
			return FactoryWorkItem{}, false, err
		}
		defer func() { _ = store.Close() }()
		return store.CancelFactoryWorkItem(
			factory,
			id,
			reason,
			cancelledAt,
			event,
		)
	}
	var result backendprotocol.FactoryWorkItemResult
	err := c.client.callProviderResult(
		ctx,
		"factory.cancelWorkItem",
		backendprotocol.FactoryCancelWorkItemParams{
			Factory:     factory,
			ID:          id,
			Reason:      reason,
			CancelledAt: cancelledAt.UTC().Format(time.RFC3339Nano),
			Event:       factoryWorkItemEventToProtocol(event),
		},
		&result,
	)
	if isBackendNotFound(err) {
		return FactoryWorkItem{}, false, nil
	}
	if err != nil {
		return FactoryWorkItem{}, false, err
	}
	item, err := factoryWorkItemFromProtocol(result.Item)
	return item, true, err
}

func isBackendNotFound(err error) bool {
	if err == nil {
		return false
	}
	var domainErr backendprotocol.Error
	return errors.As(err, &domainErr) && domainErr.Code == backendprotocol.ErrorNotFound
}

func parseFactoryTime(value string, field string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			field+" must be RFC3339 UTC",
		)
	}
	return parsed, nil
}
