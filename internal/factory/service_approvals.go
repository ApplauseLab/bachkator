package factory

import (
	"context"

	"github.com/applauselab/bachkator/internal/bacherr"
)

func (s Service) GetWithApprovals(
	ctx context.Context,
	factory string,
	id string,
) (WorkItem, error) {
	item, err := s.Get(ctx, factory, id)
	if err != nil {
		return WorkItem{}, err
	}
	approvals, err := s.Queue.ListApprovals(ctx, id)
	if err != nil {
		return WorkItem{}, err
	}
	item.Approvals = approvals
	return item, nil
}

func (s Service) Approve(ctx context.Context, opts ApproveOptions) (ApproveResult, error) {
	if err := s.validate(); err != nil {
		return ApproveResult{}, err
	}
	if opts.Factory == "" || opts.ID == "" || opts.Phase == "" {
		return ApproveResult{}, bacherr.ValidationFailedf(
			"factory, work item id, and phase are required",
		)
	}
	item, err := s.Get(ctx, opts.Factory, opts.ID)
	if err != nil {
		return ApproveResult{}, err
	}
	attemptID := ""
	if len(item.Attempts) > 0 {
		attemptID = item.Attempts[0].ID
	}
	if attemptID == "" {
		return ApproveResult{}, bacherr.ValidationFailedf("work item has no attempt")
	}
	approvalID, err := s.newID()
	if err != nil {
		return ApproveResult{}, err
	}
	eventID, err := s.newID()
	if err != nil {
		return ApproveResult{}, err
	}
	now := s.now()
	approval := Approval{
		ID:             approvalID,
		Factory:        opts.Factory,
		Workflow:       item.Workflow,
		WorkItemID:     opts.ID,
		AttemptID:      attemptID,
		Phase:          opts.Phase,
		PlanPath:       opts.PlanPath,
		PlanHash:       opts.PlanHash,
		ApprovedAt:     now,
		Approver:       opts.Approver,
		ApproverSource: opts.ApproverSource,
		Reason:         opts.Reason,
	}
	event := WorkItemEvent{
		ID:         eventID,
		WorkItemID: opts.ID,
		AttemptID:  attemptID,
		Type:       EventApproved,
		Message:    opts.Phase,
		CreatedAt:  now,
	}
	recorded, existing, err := s.Queue.RecordApproval(ctx, approval, event)
	if err != nil {
		return ApproveResult{}, err
	}
	return ApproveResult{Approval: recorded, Existing: existing}, nil
}
