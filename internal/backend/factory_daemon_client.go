package backend

import (
	"context"
	"time"

	statestore "github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func (c FactoryQueueClient) AcquireDaemonLease(
	ctx context.Context,
	lease FactoryDaemonLease,
) (FactoryDaemonLease, error) {
	if !c.client.provider {
		return withStore(
			ctx,
			c.client.path,
			func(store *statestore.Store) (FactoryDaemonLease, error) {
				return store.AcquireFactoryDaemonLease(lease)
			},
		)
	}
	var result backendprotocol.FactoryDaemonLeaseResult
	err := c.client.callProviderResult(
		ctx,
		"factory.acquireDaemonLease",
		backendprotocol.FactoryAcquireDaemonLeaseParams{Lease: factoryDaemonLeaseToProtocol(lease)},
		&result,
	)
	if err != nil {
		return FactoryDaemonLease{}, err
	}
	return factoryDaemonLeaseFromProtocol(result.Lease)
}

func (c FactoryQueueClient) RenewDaemonLease(
	ctx context.Context,
	daemonID string,
	renewedAt time.Time,
	expiresAt time.Time,
) (FactoryDaemonLease, bool, error) {
	if !c.client.provider {
		store, err := statestore.NewStore(c.client.path)
		if err != nil {
			return FactoryDaemonLease{}, false, err
		}
		defer func() { _ = store.Close() }()
		return store.
			RenewFactoryDaemonLease(daemonID, renewedAt, expiresAt)
	}
	var result backendprotocol.FactoryDaemonLeaseResult
	err := c.client.callProviderResult(
		ctx,
		"factory.renewDaemonLease",
		backendprotocol.FactoryRenewDaemonLeaseParams{
			DaemonID:  daemonID,
			RenewedAt: renewedAt.UTC().Format(time.RFC3339Nano),
			ExpiresAt: expiresAt.UTC().Format(time.RFC3339Nano),
		},
		&result,
	)
	if isBackendNotFound(err) {
		return FactoryDaemonLease{}, false, nil
	}
	if err != nil {
		return FactoryDaemonLease{}, false, err
	}
	lease, err := factoryDaemonLeaseFromProtocol(result.Lease)
	return lease, true, err
}

func (c FactoryQueueClient) ReleaseDaemonLease(
	ctx context.Context,
	daemonID string,
	releasedAt time.Time,
) (FactoryDaemonLease, bool, error) {
	if !c.client.provider {
		store, err := statestore.NewStore(c.client.path)
		if err != nil {
			return FactoryDaemonLease{}, false, err
		}
		defer func() { _ = store.Close() }()
		return store.ReleaseFactoryDaemonLease(daemonID, releasedAt)
	}
	var result backendprotocol.FactoryDaemonLeaseResult
	err := c.client.callProviderResult(
		ctx,
		"factory.releaseDaemonLease",
		backendprotocol.FactoryReleaseDaemonLeaseParams{
			DaemonID:   daemonID,
			ReleasedAt: releasedAt.UTC().Format(time.RFC3339Nano),
		},
		&result,
	)
	if isBackendNotFound(err) {
		return FactoryDaemonLease{}, false, nil
	}
	if err != nil {
		return FactoryDaemonLease{}, false, err
	}
	lease, err := factoryDaemonLeaseFromProtocol(result.Lease)
	return lease, true, err
}

func (c FactoryQueueClient) ClaimWorkItem(
	ctx context.Context,
	factory string,
	daemonID string,
	claimedAt time.Time,
	expiresAt time.Time,
) (FactoryWorkItem, bool, error) {
	if !c.client.provider {
		store, err := statestore.NewStore(c.client.path)
		if err != nil {
			return FactoryWorkItem{}, false, err
		}
		defer func() { _ = store.Close() }()
		return store.
			ClaimNextFactoryWorkItem(factory, daemonID, claimedAt, expiresAt)
	}
	var result backendprotocol.FactoryWorkItemResult
	err := c.client.callProviderResult(
		ctx,
		"factory.claimWorkItem",
		backendprotocol.FactoryClaimWorkItemParams{
			Factory:   factory,
			DaemonID:  daemonID,
			ClaimedAt: claimedAt.UTC().Format(time.RFC3339Nano),
			ExpiresAt: expiresAt.UTC().Format(time.RFC3339Nano),
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

func (c FactoryQueueClient) UpdatePhase(
	ctx context.Context,
	phase FactoryWorkItemPhase,
) error {
	if !c.client.provider {
		_, err := withStore(ctx, c.client.path, func(store *statestore.Store) (struct{}, error) {
			return struct{}{}, store.UpdateFactoryWorkItemPhase(phase)
		})
		return err
	}
	return c.client.callProvider(
		ctx,
		"factory.updateWorkItemPhase",
		backendprotocol.FactoryUpdateWorkItemPhaseParams{
			Phase: factoryWorkItemPhaseToProtocol(phase),
		},
	)
}

func (c FactoryQueueClient) RecordApproval(
	ctx context.Context,
	approval FactoryApproval,
	event FactoryWorkItemEvent,
) (FactoryApproval, bool, error) {
	if !c.client.provider {
		store, err := statestore.NewStore(c.client.path)
		if err != nil {
			return FactoryApproval{}, false, err
		}
		defer func() { _ = store.Close() }()
		return store.RecordFactoryApproval(approval, event)
	}
	var result backendprotocol.FactoryRecordApprovalResult
	err := c.client.callProviderResult(
		ctx,
		"factory.recordApproval",
		backendprotocol.FactoryRecordApprovalParams{
			Approval: factoryApprovalToProtocol(approval),
			Event:    factoryWorkItemEventToProtocol(event),
		},
		&result,
	)
	if err != nil {
		return FactoryApproval{}, false, err
	}
	mapped, err := factoryApprovalFromProtocol(result.Approval)
	return mapped, result.Existing, err
}

func (c FactoryQueueClient) ListApprovals(
	ctx context.Context,
	workItemID string,
) ([]FactoryApproval, error) {
	if !c.client.provider {
		store, err := statestore.NewStore(c.client.path)
		if err != nil {
			return nil, err
		}
		defer func() { _ = store.Close() }()
		return store.ListFactoryWorkItemApprovals(workItemID)
	}
	var result backendprotocol.FactoryListApprovalsResult
	err := c.client.callProviderResult(
		ctx,
		"factory.listApprovals",
		backendprotocol.FactoryListApprovalsParams{WorkItemID: workItemID},
		&result,
	)
	if err != nil {
		return nil, err
	}
	approvals := make([]FactoryApproval, 0, len(result.Approvals))
	for _, a := range result.Approvals {
		mapped, err := factoryApprovalFromProtocol(a)
		if err != nil {
			return nil, err
		}
		approvals = append(approvals, mapped)
	}
	return approvals, nil
}

func (c FactoryQueueClient) CompleteWorkItem(
	ctx context.Context,
	factory string,
	id string,
	completedAt time.Time,
) (FactoryWorkItem, bool, error) {
	if !c.client.provider {
		store, err := statestore.NewStore(c.client.path)
		if err != nil {
			return FactoryWorkItem{}, false, err
		}
		defer func() { _ = store.Close() }()
		return store.CompleteFactoryWorkItem(factory, id, completedAt)
	}
	var result backendprotocol.FactoryWorkItemResult
	err := c.client.callProviderResult(
		ctx,
		"factory.completeWorkItem",
		backendprotocol.FactoryFinishWorkItemParams{
			Factory:    factory,
			ID:         id,
			FinishedAt: completedAt.UTC().Format(time.RFC3339Nano),
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

func (c FactoryQueueClient) FailWorkItem(
	ctx context.Context,
	factory string,
	id string,
	phase string,
	message string,
	failedAt time.Time,
) (FactoryWorkItem, bool, error) {
	if !c.client.provider {
		store, err := statestore.NewStore(c.client.path)
		if err != nil {
			return FactoryWorkItem{}, false, err
		}
		defer func() { _ = store.Close() }()
		return store.
			FailFactoryWorkItem(factory, id, phase, message, failedAt)
	}
	var result backendprotocol.FactoryWorkItemResult
	err := c.client.callProviderResult(
		ctx,
		"factory.failWorkItem",
		backendprotocol.FactoryFinishWorkItemParams{
			Factory:        factory,
			ID:             id,
			FinishedAt:     failedAt.UTC().Format(time.RFC3339Nano),
			FailurePhase:   phase,
			FailureMessage: message,
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

func (c FactoryQueueClient) DaemonStatus(
	ctx context.Context,
	factory string,
	now time.Time,
) (FactoryDaemonStatus, error) {
	if !c.client.provider {
		return withStore(
			ctx,
			c.client.path,
			func(store *statestore.Store) (FactoryDaemonStatus, error) {
				return store.GetFactoryDaemonStatus(factory, now)
			},
		)
	}
	var result backendprotocol.FactoryDaemonStatusResult
	err := c.client.callProviderResult(
		ctx,
		"factory.getDaemonStatus",
		backendprotocol.FactoryDaemonStatusQuery{
			Factory: factory,
			Now:     now.UTC().Format(time.RFC3339Nano),
		},
		&result,
	)
	if err != nil {
		return FactoryDaemonStatus{}, err
	}
	return factoryDaemonStatusFromProtocol(result)
}
