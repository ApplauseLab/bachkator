package backend

import (
	"time"

	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func factoryWorkItemToProtocol(item FactoryWorkItem) backendprotocol.FactoryWorkItem {
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
	result.FailurePhase = item.FailurePhase
	result.FailureMessage = item.FailureMessage
	if !item.CancelledAt.IsZero() {
		result.CancelledAt = item.CancelledAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func factoryWorkItemFromProtocol(item backendprotocol.FactoryWorkItem) (FactoryWorkItem, error) {
	createdAt, err := parseFactoryTime(item.CreatedAt, "created_at")
	if err != nil {
		return FactoryWorkItem{}, err
	}
	updatedAt, err := parseFactoryTime(item.UpdatedAt, "updated_at")
	if err != nil {
		return FactoryWorkItem{}, err
	}
	cancelledAt, err := parseFactoryTime(item.CancelledAt, "cancelled_at")
	if err != nil {
		return FactoryWorkItem{}, err
	}
	claimedAt, err := parseFactoryTime(item.ClaimedAt, "claimed_at")
	if err != nil {
		return FactoryWorkItem{}, err
	}
	claimExpiresAt, err := parseFactoryTime(item.ClaimExpiresAt, "claim_expires_at")
	if err != nil {
		return FactoryWorkItem{}, err
	}
	completedAt, err := parseFactoryTime(item.CompletedAt, "completed_at")
	if err != nil {
		return FactoryWorkItem{}, err
	}
	failedAt, err := parseFactoryTime(item.FailedAt, "failed_at")
	if err != nil {
		return FactoryWorkItem{}, err
	}
	result := FactoryWorkItem{
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
		Attempts:           make([]FactoryWorkItemAttempt, 0, len(item.Attempts)),
		Events:             make([]FactoryWorkItemEvent, 0, len(item.Events)),
	}
	for _, attempt := range item.Attempts {
		mapped, err := factoryWorkItemAttemptFromProtocol(attempt)
		if err != nil {
			return FactoryWorkItem{}, err
		}
		result.Attempts = append(result.Attempts, mapped)
	}
	for _, event := range item.Events {
		mapped, err := factoryWorkItemEventFromProtocol(event)
		if err != nil {
			return FactoryWorkItem{}, err
		}
		result.Events = append(result.Events, mapped)
	}
	return result, nil
}

func factoryWorkItemAttemptToProtocol(
	attempt FactoryWorkItemAttempt,
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

func factoryWorkItemAttemptFromProtocol(
	attempt backendprotocol.FactoryWorkItemAttempt,
) (FactoryWorkItemAttempt, error) {
	createdAt, err := parseFactoryTime(attempt.CreatedAt, "created_at")
	if err != nil {
		return FactoryWorkItemAttempt{}, err
	}
	updatedAt, err := parseFactoryTime(attempt.UpdatedAt, "updated_at")
	if err != nil {
		return FactoryWorkItemAttempt{}, err
	}
	finishedAt, err := parseFactoryTime(attempt.FinishedAt, "finished_at")
	if err != nil {
		return FactoryWorkItemAttempt{}, err
	}
	return FactoryWorkItemAttempt{
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

func factoryWorkItemEventToProtocol(
	event FactoryWorkItemEvent,
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

func factoryWorkItemEventFromProtocol(
	event backendprotocol.FactoryWorkItemEvent,
) (FactoryWorkItemEvent, error) {
	createdAt, err := parseFactoryTime(event.CreatedAt, "created_at")
	if err != nil {
		return FactoryWorkItemEvent{}, err
	}
	return FactoryWorkItemEvent{
		ID:         event.ID,
		WorkItemID: event.WorkItemID,
		AttemptID:  event.AttemptID,
		Type:       event.Type,
		Message:    event.Message,
		Metadata:   cloneStringMap(event.Metadata),
		CreatedAt:  createdAt,
	}, nil
}

func factoryApprovalToProtocol(approval FactoryApproval) backendprotocol.FactoryApproval {
	result := backendprotocol.FactoryApproval{
		ID:             approval.ID,
		Factory:        approval.Factory,
		Workflow:       approval.Workflow,
		WorkItemID:     approval.WorkItemID,
		AttemptID:      approval.AttemptID,
		Phase:          approval.Phase,
		PlanPath:       approval.PlanPath,
		PlanHash:       approval.PlanHash,
		Approver:       approval.Approver,
		ApproverSource: approval.ApproverSource,
		Reason:         approval.Reason,
		Metadata:       cloneStringMap(approval.Metadata),
	}
	if !approval.ApprovedAt.IsZero() {
		result.ApprovedAt = approval.ApprovedAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func factoryApprovalFromProtocol(
	approval backendprotocol.FactoryApproval,
) (FactoryApproval, error) {
	approvedAt, err := parseFactoryTime(approval.ApprovedAt, "approved_at")
	if err != nil {
		return FactoryApproval{}, err
	}
	return FactoryApproval{
		ID:             approval.ID,
		Factory:        approval.Factory,
		Workflow:       approval.Workflow,
		WorkItemID:     approval.WorkItemID,
		AttemptID:      approval.AttemptID,
		Phase:          approval.Phase,
		PlanPath:       approval.PlanPath,
		PlanHash:       approval.PlanHash,
		ApprovedAt:     approvedAt,
		Approver:       approval.Approver,
		ApproverSource: approval.ApproverSource,
		Reason:         approval.Reason,
		Metadata:       cloneStringMap(approval.Metadata),
	}, nil
}

func factoryDaemonLeaseToProtocol(lease FactoryDaemonLease) backendprotocol.FactoryDaemonLease {
	result := backendprotocol.FactoryDaemonLease{
		DaemonID: lease.DaemonID,
		Factory:  lease.Factory,
		Hostname: lease.Hostname,
		PID:      lease.PID,
		Status:   lease.Status,
	}
	if !lease.AcquiredAt.IsZero() {
		result.AcquiredAt = lease.AcquiredAt.UTC().Format(time.RFC3339Nano)
	}
	if !lease.RenewedAt.IsZero() {
		result.RenewedAt = lease.RenewedAt.UTC().Format(time.RFC3339Nano)
	}
	if !lease.ExpiresAt.IsZero() {
		result.ExpiresAt = lease.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	if !lease.ReleasedAt.IsZero() {
		result.ReleasedAt = lease.ReleasedAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func factoryDaemonLeaseFromProtocol(
	lease backendprotocol.FactoryDaemonLease,
) (FactoryDaemonLease, error) {
	acquiredAt, err := parseFactoryTime(lease.AcquiredAt, "acquired_at")
	if err != nil {
		return FactoryDaemonLease{}, err
	}
	renewedAt, err := parseFactoryTime(lease.RenewedAt, "renewed_at")
	if err != nil {
		return FactoryDaemonLease{}, err
	}
	expiresAt, err := parseFactoryTime(lease.ExpiresAt, "expires_at")
	if err != nil {
		return FactoryDaemonLease{}, err
	}
	releasedAt, err := parseFactoryTime(lease.ReleasedAt, "released_at")
	if err != nil {
		return FactoryDaemonLease{}, err
	}
	return FactoryDaemonLease{
		DaemonID:   lease.DaemonID,
		Factory:    lease.Factory,
		Hostname:   lease.Hostname,
		PID:        lease.PID,
		AcquiredAt: acquiredAt,
		RenewedAt:  renewedAt,
		ExpiresAt:  expiresAt,
		ReleasedAt: releasedAt,
		Status:     lease.Status,
	}, nil
}

func factoryWorkItemPhaseToProtocol(
	phase FactoryWorkItemPhase,
) backendprotocol.FactoryWorkItemPhase {
	result := backendprotocol.FactoryWorkItemPhase{
		WorkItemID: phase.WorkItemID,
		AttemptID:  phase.AttemptID,
		PhaseKey:   phase.PhaseKey,
		Status:     phase.Status,
		Target:     phase.Target,
		RunID:      phase.RunID,
		PlanPath:   phase.PlanPath,
		LedgerID:   phase.LedgerID,
		Evidence:   cloneStringMap(phase.Evidence),
	}
	if !phase.StartedAt.IsZero() {
		result.StartedAt = phase.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if !phase.FinishedAt.IsZero() {
		result.FinishedAt = phase.FinishedAt.UTC().Format(time.RFC3339Nano)
	}
	if !phase.UpdatedAt.IsZero() {
		result.UpdatedAt = phase.UpdatedAt.UTC().Format(time.RFC3339Nano)
	}
	return result
}

func factoryDaemonStatusFromProtocol(
	result backendprotocol.FactoryDaemonStatusResult,
) (FactoryDaemonStatus, error) {
	lease, err := factoryDaemonLeaseFromProtocol(result.Lease)
	if err != nil {
		return FactoryDaemonStatus{}, err
	}
	item, err := factoryWorkItemFromProtocol(result.ActiveItem)
	if err != nil && result.HasActiveItem {
		return FactoryDaemonStatus{}, err
	}
	return FactoryDaemonStatus{
		Lease:           lease,
		ActiveItem:      item,
		HasActiveItem:   result.HasActiveItem,
		LifecycleCounts: result.LifecycleCounts,
	}, nil
}
