package factory

import (
	"context"
	"time"

	"github.com/applauselab/bachkator/internal/backend"
)

type BackendQueue struct {
	Client *backend.FactoryQueueClient
}

func (q BackendQueue) Enqueue(
	ctx context.Context,
	item WorkItem,
	attempt WorkItemAttempt,
	event WorkItemEvent,
	dedupeEvent WorkItemEvent,
) (WorkItem, bool, error) {
	createdItem, created, err := q.Client.Enqueue(
		ctx,
		workItemToBackend(item),
		workItemAttemptToBackend(attempt),
		workItemEventToBackend(event),
		workItemEventToBackend(dedupeEvent),
	)
	if err != nil {
		return WorkItem{}, false, err
	}
	return workItemFromBackend(createdItem), created, nil
}

func (q BackendQueue) UpdatePending(
	ctx context.Context,
	item WorkItem,
	event WorkItemEvent,
) (WorkItem, bool, error) {
	updated, ok, err := q.Client.UpdatePending(
		ctx,
		workItemToBackend(item),
		workItemEventToBackend(event),
	)
	if err != nil {
		return WorkItem{}, false, err
	}
	return workItemFromBackend(updated), ok, nil
}

func (q BackendQueue) Get(ctx context.Context, factory string, id string) (WorkItem, bool, error) {
	item, ok, err := q.Client.Get(ctx, factory, id)
	if err != nil || !ok {
		return WorkItem{}, ok, err
	}
	return workItemFromBackend(item), true, nil
}

func (q BackendQueue) List(ctx context.Context, query WorkItemQuery) ([]WorkItem, error) {
	items, err := q.Client.List(ctx, backend.FactoryWorkItemQuery{
		Factory: query.Factory,
		ID:      query.ID,
		Status:  query.Status,
	})
	if err != nil {
		return nil, err
	}
	result := make([]WorkItem, 0, len(items))
	for _, item := range items {
		result = append(result, workItemFromBackend(item))
	}
	return result, nil
}

func (q BackendQueue) Cancel(
	ctx context.Context,
	factory string,
	id string,
	reason string,
	cancelledAt time.Time,
	event WorkItemEvent,
) (WorkItem, bool, error) {
	item, ok, err := q.Client.Cancel(
		ctx,
		factory,
		id,
		reason,
		cancelledAt,
		workItemEventToBackend(event),
	)
	if err != nil || !ok {
		return WorkItem{}, ok, err
	}
	return workItemFromBackend(item), true, nil
}

func (q BackendQueue) RecordApproval(
	ctx context.Context,
	approval Approval,
	event WorkItemEvent,
) (Approval, bool, error) {
	recorded, existing, err := q.Client.RecordApproval(
		ctx,
		approvalToBackend(approval),
		workItemEventToBackend(event),
	)
	if err != nil {
		return Approval{}, false, err
	}
	return approvalFromBackend(recorded), existing, nil
}

func (q BackendQueue) ListApprovals(ctx context.Context, workItemID string) ([]Approval, error) {
	approvals, err := q.Client.ListApprovals(ctx, workItemID)
	if err != nil {
		return nil, err
	}
	result := make([]Approval, 0, len(approvals))
	for _, a := range approvals {
		result = append(result, approvalFromBackend(a))
	}
	return result, nil
}

func approvalToBackend(approval Approval) backend.FactoryApproval {
	return backend.FactoryApproval{
		ID:             approval.ID,
		Factory:        approval.Factory,
		Workflow:       approval.Workflow,
		WorkItemID:     approval.WorkItemID,
		AttemptID:      approval.AttemptID,
		Phase:          approval.Phase,
		PlanPath:       approval.PlanPath,
		PlanHash:       approval.PlanHash,
		ApprovedAt:     approval.ApprovedAt,
		Approver:       approval.Approver,
		ApproverSource: approval.ApproverSource,
		Reason:         approval.Reason,
		Metadata:       cloneStringMap(approval.Metadata),
	}
}

func approvalFromBackend(approval backend.FactoryApproval) Approval {
	return Approval{
		ID:             approval.ID,
		Factory:        approval.Factory,
		Workflow:       approval.Workflow,
		WorkItemID:     approval.WorkItemID,
		AttemptID:      approval.AttemptID,
		Phase:          approval.Phase,
		PlanPath:       approval.PlanPath,
		PlanHash:       approval.PlanHash,
		ApprovedAt:     approval.ApprovedAt,
		Approver:       approval.Approver,
		ApproverSource: approval.ApproverSource,
		Reason:         approval.Reason,
		Metadata:       cloneStringMap(approval.Metadata),
	}
}

func workItemToBackend(item WorkItem) backend.FactoryWorkItem {
	return backend.FactoryWorkItem{
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
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
		CancelledAt:        item.CancelledAt,
		CancelReason:       item.CancelReason,
	}
}

func workItemFromBackend(item backend.FactoryWorkItem) WorkItem {
	result := WorkItem{
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
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
		CancelledAt:        item.CancelledAt,
		CancelReason:       item.CancelReason,
		FailurePhase:       item.FailurePhase,
		FailureMessage:     item.FailureMessage,
		Attempts:           make([]WorkItemAttempt, 0, len(item.Attempts)),
		Events:             make([]WorkItemEvent, 0, len(item.Events)),
	}
	for _, attempt := range item.Attempts {
		result.Attempts = append(result.Attempts, workItemAttemptFromBackend(attempt))
	}
	for _, event := range item.Events {
		result.Events = append(result.Events, workItemEventFromBackend(event))
	}
	return result
}

func workItemAttemptToBackend(attempt WorkItemAttempt) backend.FactoryWorkItemAttempt {
	return backend.FactoryWorkItemAttempt{
		ID:                attempt.ID,
		WorkItemID:        attempt.WorkItemID,
		AttemptNumber:     attempt.AttemptNumber,
		Status:            attempt.Status,
		StartPhase:        attempt.StartPhase,
		SubmittedPlanPath: attempt.SubmittedPlanPath,
		SubmittedPlanHash: attempt.SubmittedPlanHash,
		CreatedAt:         attempt.CreatedAt,
		UpdatedAt:         attempt.UpdatedAt,
		FinishedAt:        attempt.FinishedAt,
	}
}

func workItemAttemptFromBackend(attempt backend.FactoryWorkItemAttempt) WorkItemAttempt {
	return WorkItemAttempt{
		ID:                attempt.ID,
		WorkItemID:        attempt.WorkItemID,
		AttemptNumber:     attempt.AttemptNumber,
		Status:            attempt.Status,
		StartPhase:        attempt.StartPhase,
		SubmittedPlanPath: attempt.SubmittedPlanPath,
		SubmittedPlanHash: attempt.SubmittedPlanHash,
		CreatedAt:         attempt.CreatedAt,
		UpdatedAt:         attempt.UpdatedAt,
		FinishedAt:        attempt.FinishedAt,
	}
}

func workItemEventToBackend(event WorkItemEvent) backend.FactoryWorkItemEvent {
	return backend.FactoryWorkItemEvent{
		ID:         event.ID,
		WorkItemID: event.WorkItemID,
		AttemptID:  event.AttemptID,
		Type:       event.Type,
		Message:    event.Message,
		Metadata:   cloneStringMap(event.Metadata),
		CreatedAt:  event.CreatedAt,
	}
}

func workItemEventFromBackend(event backend.FactoryWorkItemEvent) WorkItemEvent {
	return WorkItemEvent{
		ID:         event.ID,
		WorkItemID: event.WorkItemID,
		AttemptID:  event.AttemptID,
		Type:       event.Type,
		Message:    event.Message,
		Metadata:   cloneStringMap(event.Metadata),
		CreatedAt:  event.CreatedAt,
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
