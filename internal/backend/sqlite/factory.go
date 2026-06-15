package sqlite

import (
	"encoding/json"
	"time"

	"github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func (p *Provider) enqueueFactoryWorkItem(
	raw json.RawMessage,
) (backendprotocol.FactoryWorkItemResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	var params backendprotocol.FactoryEnqueueWorkItemParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return backendprotocol.FactoryWorkItemResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	item, err := factoryWorkItemFromProtocol(params.Item)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	attempt, err := factoryWorkItemAttemptFromProtocol(params.Attempt)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	event, err := factoryWorkItemEventFromProtocol(params.Event)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	dedupeEvent, err := factoryWorkItemEventFromProtocol(params.DedupeEvent)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	defer func() { _ = store.Close() }()
	createdItem, created, err := store.EnqueueFactoryWorkItem(
		item,
		attempt,
		event,
		dedupeEvent,
	)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	return backendprotocol.FactoryWorkItemResult{
		Item:    factoryWorkItemToProtocol(createdItem),
		Created: created,
	}, nil
}

func (p *Provider) getFactoryWorkItem(
	raw json.RawMessage,
) (backendprotocol.FactoryWorkItemResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	var query backendprotocol.FactoryWorkItemQuery
	if err := json.Unmarshal(raw, &query); err != nil {
		return backendprotocol.FactoryWorkItemResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	if query.Factory == "" || query.ID == "" {
		return backendprotocol.FactoryWorkItemResult{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			"factory and id are required",
		)
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	defer func() { _ = store.Close() }()
	item, ok, err := store.GetFactoryWorkItem(query.Factory, query.ID)
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

func (p *Provider) listFactoryWorkItems(
	raw json.RawMessage,
) (backendprotocol.FactoryWorkItemListResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryWorkItemListResult{}, err
	}
	var query backendprotocol.FactoryWorkItemQuery
	if err := json.Unmarshal(raw, &query); err != nil {
		return backendprotocol.FactoryWorkItemListResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	if query.Factory == "" {
		return backendprotocol.FactoryWorkItemListResult{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			"factory is required",
		)
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryWorkItemListResult{}, err
	}
	defer func() { _ = store.Close() }()
	items, err := store.ListFactoryWorkItems(state.FactoryWorkItemQuery{
		Factory: query.Factory,
		Status:  string(query.Status),
	})
	if err != nil {
		return backendprotocol.FactoryWorkItemListResult{}, err
	}
	result := backendprotocol.FactoryWorkItemListResult{
		Items: make([]backendprotocol.FactoryWorkItem, 0, len(items)),
	}
	for _, item := range items {
		result.Items = append(result.Items, factoryWorkItemToProtocol(item))
	}
	return result, nil
}

func (p *Provider) cancelFactoryWorkItem(
	raw json.RawMessage,
) (backendprotocol.FactoryWorkItemResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	var params backendprotocol.FactoryCancelWorkItemParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return backendprotocol.FactoryWorkItemResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	if params.Factory == "" || params.ID == "" {
		return backendprotocol.FactoryWorkItemResult{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			"factory and id are required",
		)
	}
	cancelledAt, err := parseOptionalTime(params.CancelledAt, "cancelled_at")
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	event, err := factoryWorkItemEventFromProtocol(params.Event)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	defer func() { _ = store.Close() }()
	item, ok, err := store.CancelFactoryWorkItem(
		params.Factory,
		params.ID,
		params.Reason,
		cancelledAt,
		event,
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

func factoryWorkItemFromProtocol(
	item backendprotocol.FactoryWorkItem,
) (state.FactoryWorkItem, error) {
	createdAt, err := parseOptionalTime(item.CreatedAt, "created_at")
	if err != nil {
		return state.FactoryWorkItem{}, err
	}
	updatedAt, err := parseOptionalTime(item.UpdatedAt, "updated_at")
	if err != nil {
		return state.FactoryWorkItem{}, err
	}
	cancelledAt, err := parseOptionalTime(item.CancelledAt, "cancelled_at")
	if err != nil {
		return state.FactoryWorkItem{}, err
	}
	claimedAt, err := parseOptionalTime(item.ClaimedAt, "claimed_at")
	if err != nil {
		return state.FactoryWorkItem{}, err
	}
	claimExpiresAt, err := parseOptionalTime(item.ClaimExpiresAt, "claim_expires_at")
	if err != nil {
		return state.FactoryWorkItem{}, err
	}
	completedAt, err := parseOptionalTime(item.CompletedAt, "completed_at")
	if err != nil {
		return state.FactoryWorkItem{}, err
	}
	failedAt, err := parseOptionalTime(item.FailedAt, "failed_at")
	if err != nil {
		return state.FactoryWorkItem{}, err
	}
	return state.FactoryWorkItem{
		ID:                 item.ID,
		Factory:            item.Factory,
		Workflow:           item.Workflow,
		Lifecycle:          item.Lifecycle,
		CurrentPhase:       item.CurrentPhase,
		Title:              item.Title,
		Body:               item.Body,
		BodyHash:           item.BodyHash,
		Priority:           item.Priority,
		Labels:             append([]string(nil), item.Labels...),
		SourceType:         item.SourceType,
		DedupeKey:          item.DedupeKey,
		SubmittedPlanPath:  item.SubmittedPlanPath,
		SubmittedPlanHash:  item.SubmittedPlanHash,
		IntakeEvidenceID:   item.IntakeEvidenceID,
		IntakeEvidenceURI:  item.IntakeEvidenceURI,
		IntakeEvidenceHash: item.IntakeEvidenceHash,
		Metadata:           cloneStringMap(item.Metadata),
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
		CancelledAt:        cancelledAt,
		CancelReason:       item.CancelReason,
		ClaimedByDaemonID:  item.ClaimedByDaemonID,
		ClaimedAt:          claimedAt,
		ClaimExpiresAt:     claimExpiresAt,
		CompletedAt:        completedAt,
		FailedAt:           failedAt,
		FailurePhase:       item.FailurePhase,
		FailureMessage:     item.FailureMessage,
	}, nil
}

func factoryWorkItemAttemptFromProtocol(
	attempt backendprotocol.FactoryWorkItemAttempt,
) (state.FactoryWorkItemAttempt, error) {
	createdAt, err := parseOptionalTime(attempt.CreatedAt, "created_at")
	if err != nil {
		return state.FactoryWorkItemAttempt{}, err
	}
	updatedAt, err := parseOptionalTime(attempt.UpdatedAt, "updated_at")
	if err != nil {
		return state.FactoryWorkItemAttempt{}, err
	}
	finishedAt, err := parseOptionalTime(attempt.FinishedAt, "finished_at")
	if err != nil {
		return state.FactoryWorkItemAttempt{}, err
	}
	return state.FactoryWorkItemAttempt{
		ID:                attempt.ID,
		WorkItemID:        attempt.WorkItemID,
		AttemptNumber:     attempt.AttemptNumber,
		Status:            attempt.Status,
		StartPhase:        attempt.StartPhase,
		SubmittedPlanPath: attempt.SubmittedPlanPath,
		SubmittedPlanHash: attempt.SubmittedPlanHash,
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
		FinishedAt:        finishedAt,
	}, nil
}

func factoryWorkItemEventFromProtocol(
	event backendprotocol.FactoryWorkItemEvent,
) (state.FactoryWorkItemEvent, error) {
	createdAt, err := parseOptionalTime(event.CreatedAt, "created_at")
	if err != nil {
		return state.FactoryWorkItemEvent{}, err
	}
	return state.FactoryWorkItemEvent{
		ID:         event.ID,
		WorkItemID: event.WorkItemID,
		AttemptID:  event.AttemptID,
		Type:       event.Type,
		Message:    event.Message,
		Metadata:   cloneStringMap(event.Metadata),
		CreatedAt:  createdAt,
	}, nil
}

func factoryWorkItemToProtocol(item state.FactoryWorkItem) backendprotocol.FactoryWorkItem {
	result := backendprotocol.FactoryWorkItem{
		SchemaVersion:      "bach.backend.factory_work_item.v1",
		ID:                 item.ID,
		Factory:            item.Factory,
		Workflow:           item.Workflow,
		Lifecycle:          item.Lifecycle,
		CurrentPhase:       item.CurrentPhase,
		Title:              item.Title,
		Body:               item.Body,
		BodyHash:           item.BodyHash,
		Priority:           item.Priority,
		Labels:             append([]string(nil), item.Labels...),
		SourceType:         item.SourceType,
		DedupeKey:          item.DedupeKey,
		SubmittedPlanPath:  item.SubmittedPlanPath,
		SubmittedPlanHash:  item.SubmittedPlanHash,
		IntakeEvidenceID:   item.IntakeEvidenceID,
		IntakeEvidenceURI:  item.IntakeEvidenceURI,
		IntakeEvidenceHash: item.IntakeEvidenceHash,
		Metadata:           cloneStringMap(item.Metadata),
		CreatedAt:          item.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:          item.UpdatedAt.UTC().Format(time.RFC3339Nano),
		CancelReason:       item.CancelReason,
		ClaimedByDaemonID:  item.ClaimedByDaemonID,
		FailurePhase:       item.FailurePhase,
		FailureMessage:     item.FailureMessage,
		Attempts: make(
			[]backendprotocol.FactoryWorkItemAttempt,
			0,
			len(item.Attempts),
		),
		Events: make([]backendprotocol.FactoryWorkItemEvent, 0, len(item.Events)),
	}
	if !item.CancelledAt.IsZero() {
		result.CancelledAt = item.CancelledAt.UTC().Format(time.RFC3339Nano)
	}
	if !item.ClaimedAt.IsZero() {
		result.ClaimedAt = item.ClaimedAt.UTC().Format(time.RFC3339Nano)
	}
	if !item.ClaimExpiresAt.IsZero() {
		result.ClaimExpiresAt = item.ClaimExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	if !item.CompletedAt.IsZero() {
		result.CompletedAt = item.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	if !item.FailedAt.IsZero() {
		result.FailedAt = item.FailedAt.UTC().Format(time.RFC3339Nano)
	}
	for _, attempt := range item.Attempts {
		result.Attempts = append(result.Attempts, factoryWorkItemAttemptToProtocol(attempt))
	}
	for _, event := range item.Events {
		result.Events = append(result.Events, factoryWorkItemEventToProtocol(event))
	}
	return result
}

func factoryWorkItemAttemptToProtocol(
	attempt state.FactoryWorkItemAttempt,
) backendprotocol.FactoryWorkItemAttempt {
	result := backendprotocol.FactoryWorkItemAttempt{
		ID:                attempt.ID,
		WorkItemID:        attempt.WorkItemID,
		AttemptNumber:     attempt.AttemptNumber,
		Status:            attempt.Status,
		StartPhase:        attempt.StartPhase,
		SubmittedPlanPath: attempt.SubmittedPlanPath,
		SubmittedPlanHash: attempt.SubmittedPlanHash,
		CreatedAt:         attempt.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:         attempt.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if !attempt.FinishedAt.IsZero() {
		result.FinishedAt = attempt.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func factoryWorkItemEventToProtocol(
	event state.FactoryWorkItemEvent,
) backendprotocol.FactoryWorkItemEvent {
	return backendprotocol.FactoryWorkItemEvent{
		ID:         event.ID,
		WorkItemID: event.WorkItemID,
		AttemptID:  event.AttemptID,
		Type:       event.Type,
		Message:    event.Message,
		Metadata:   cloneStringMap(event.Metadata),
		CreatedAt:  event.CreatedAt.UTC().Format(time.RFC3339Nano),
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
