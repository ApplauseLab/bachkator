package sqlite

import (
	"encoding/json"

	"github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func (p *Provider) acquireFactoryDaemonLease(
	raw json.RawMessage,
) (backendprotocol.FactoryDaemonLeaseResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	var params backendprotocol.FactoryAcquireDaemonLeaseParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	lease, err := factoryDaemonLeaseFromProtocol(params.Lease)
	if err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	defer func() { _ = store.Close() }()
	lease, err = store.AcquireFactoryDaemonLease(lease)
	if err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	return backendprotocol.FactoryDaemonLeaseResult{Lease: factoryDaemonLeaseToProtocol(lease)}, nil
}

func (p *Provider) renewFactoryDaemonLease(
	raw json.RawMessage,
) (backendprotocol.FactoryDaemonLeaseResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	var params backendprotocol.FactoryRenewDaemonLeaseParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	renewedAt, err := parseOptionalTime(params.RenewedAt, "renewed_at")
	if err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	expiresAt, err := parseOptionalTime(params.ExpiresAt, "expires_at")
	if err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	defer func() { _ = store.Close() }()
	lease, ok, err := store.
		RenewFactoryDaemonLease(params.DaemonID, renewedAt, expiresAt)
	if err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	if !ok {
		return backendprotocol.FactoryDaemonLeaseResult{}, backendprotocol.NewError(
			backendprotocol.ErrorNotFound,
			"daemon lease not found",
		)
	}
	return backendprotocol.FactoryDaemonLeaseResult{Lease: factoryDaemonLeaseToProtocol(lease)}, nil
}

func (p *Provider) releaseFactoryDaemonLease(
	raw json.RawMessage,
) (backendprotocol.FactoryDaemonLeaseResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	var params backendprotocol.FactoryReleaseDaemonLeaseParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	releasedAt, err := parseOptionalTime(params.ReleasedAt, "released_at")
	if err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	defer func() { _ = store.Close() }()
	lease, ok, err := store.
		ReleaseFactoryDaemonLease(params.DaemonID, releasedAt)
	if err != nil {
		return backendprotocol.FactoryDaemonLeaseResult{}, err
	}
	if !ok {
		return backendprotocol.FactoryDaemonLeaseResult{}, backendprotocol.NewError(
			backendprotocol.ErrorNotFound,
			"daemon lease not found",
		)
	}
	return backendprotocol.FactoryDaemonLeaseResult{Lease: factoryDaemonLeaseToProtocol(lease)}, nil
}

func (p *Provider) claimFactoryWorkItem(
	raw json.RawMessage,
) (backendprotocol.FactoryWorkItemResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	var params backendprotocol.FactoryClaimWorkItemParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return backendprotocol.FactoryWorkItemResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	claimedAt, err := parseOptionalTime(params.ClaimedAt, "claimed_at")
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	expiresAt, err := parseOptionalTime(params.ExpiresAt, "expires_at")
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	defer func() { _ = store.Close() }()
	item, ok, err := store.
		ClaimNextFactoryWorkItem(params.Factory, params.DaemonID, claimedAt, expiresAt)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	if !ok {
		return backendprotocol.FactoryWorkItemResult{}, backendprotocol.NewError(
			backendprotocol.ErrorNotFound,
			"factory work item not found",
		)
	}
	return backendprotocol.FactoryWorkItemResult{Item: factoryWorkItemToProtocol(item)}, nil
}

func (p *Provider) updateFactoryWorkItemPhase(raw json.RawMessage) (map[string]bool, error) {
	if err := p.requireInitialized(); err != nil {
		return nil, err
	}
	var params backendprotocol.FactoryUpdateWorkItemPhaseParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, backendprotocol.NewError(backendprotocol.ErrorInvalidRequest, err.Error())
	}
	phase, err := factoryWorkItemPhaseFromProtocol(params.Phase)
	if err != nil {
		return nil, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = store.Close() }()
	if err := store.UpdateFactoryWorkItemPhase(phase); err != nil {
		return nil, err
	}
	return map[string]bool{"ok": true}, nil
}

func (p *Provider) completeFactoryWorkItem(
	raw json.RawMessage,
) (backendprotocol.FactoryWorkItemResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	params, err := parseFactoryFinishParams(raw)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	finishedAt, err := parseOptionalTime(params.FinishedAt, "finished_at")
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	defer func() { _ = store.Close() }()
	item, ok, err := store.
		CompleteFactoryWorkItem(params.Factory, params.ID, finishedAt)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	if !ok {
		return backendprotocol.FactoryWorkItemResult{}, backendprotocol.NewError(
			backendprotocol.ErrorNotFound,
			"factory work item not found",
		)
	}
	return backendprotocol.FactoryWorkItemResult{Item: factoryWorkItemToProtocol(item)}, nil
}

func (p *Provider) failFactoryWorkItem(
	raw json.RawMessage,
) (backendprotocol.FactoryWorkItemResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	params, err := parseFactoryFinishParams(raw)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	failedAt, err := parseOptionalTime(params.FinishedAt, "finished_at")
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	defer func() { _ = store.Close() }()
	item, ok, err := store.
		FailFactoryWorkItem(
			params.Factory,
			params.ID,
			params.FailurePhase,
			params.FailureMessage,
			failedAt,
		)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	if !ok {
		return backendprotocol.FactoryWorkItemResult{}, backendprotocol.NewError(
			backendprotocol.ErrorNotFound,
			"factory work item not found",
		)
	}
	return backendprotocol.FactoryWorkItemResult{Item: factoryWorkItemToProtocol(item)}, nil
}

func (p *Provider) getFactoryDaemonStatus(
	raw json.RawMessage,
) (backendprotocol.FactoryDaemonStatusResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryDaemonStatusResult{}, err
	}
	var query backendprotocol.FactoryDaemonStatusQuery
	if err := json.Unmarshal(raw, &query); err != nil {
		return backendprotocol.FactoryDaemonStatusResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	now, err := parseOptionalTime(query.Now, "now")
	if err != nil {
		return backendprotocol.FactoryDaemonStatusResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryDaemonStatusResult{}, err
	}
	defer func() { _ = store.Close() }()
	status, err := store.GetFactoryDaemonStatus(query.Factory, now)
	if err != nil {
		return backendprotocol.FactoryDaemonStatusResult{}, err
	}
	activeItem := factoryWorkItemToProtocol(status.ActiveItem)
	activeItem.Body = ""
	activeItem.Metadata = nil
	activeItem.DedupeKey = ""
	activeItem.IntakeEvidenceURI = ""
	activeItem.IntakeEvidenceHash = ""
	return backendprotocol.FactoryDaemonStatusResult{
		Lease:           factoryDaemonLeaseToProtocol(status.Lease),
		ActiveItem:      activeItem,
		HasActiveItem:   status.HasActiveItem,
		LifecycleCounts: status.LifecycleCounts,
	}, nil
}

func parseFactoryFinishParams(
	raw json.RawMessage,
) (backendprotocol.FactoryFinishWorkItemParams, error) {
	var params backendprotocol.FactoryFinishWorkItemParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return params, backendprotocol.NewError(backendprotocol.ErrorInvalidRequest, err.Error())
	}
	return params, nil
}
