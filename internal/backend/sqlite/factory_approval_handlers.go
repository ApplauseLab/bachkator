package sqlite

import (
	"encoding/json"

	"github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func (p *Provider) recordFactoryApproval(
	raw json.RawMessage,
) (backendprotocol.FactoryRecordApprovalResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryRecordApprovalResult{}, err
	}
	var params backendprotocol.FactoryRecordApprovalParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return backendprotocol.FactoryRecordApprovalResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	approval, err := factoryApprovalFromProtocol(params.Approval)
	if err != nil {
		return backendprotocol.FactoryRecordApprovalResult{}, err
	}
	event, err := factoryWorkItemEventFromProtocol(params.Event)
	if err != nil {
		return backendprotocol.FactoryRecordApprovalResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryRecordApprovalResult{}, err
	}
	defer func() { _ = store.Close() }()
	recorded, existing, err := store.RecordFactoryApproval(approval, event)
	if err != nil {
		return backendprotocol.FactoryRecordApprovalResult{}, err
	}
	return backendprotocol.FactoryRecordApprovalResult{
		Approval: factoryApprovalToProtocol(recorded),
		Existing: existing,
	}, nil
}

func (p *Provider) listFactoryApprovals(
	raw json.RawMessage,
) (backendprotocol.FactoryListApprovalsResult, error) {
	if err := p.requireInitialized(); err != nil {
		return backendprotocol.FactoryListApprovalsResult{}, err
	}
	var params backendprotocol.FactoryListApprovalsParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return backendprotocol.FactoryListApprovalsResult{}, backendprotocol.NewError(
			backendprotocol.ErrorInvalidRequest,
			err.Error(),
		)
	}
	if params.WorkItemID == "" {
		return backendprotocol.FactoryListApprovalsResult{}, backendprotocol.NewError(
			backendprotocol.ErrorValidationFailed,
			"work_item_id is required",
		)
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryListApprovalsResult{}, err
	}
	defer func() { _ = store.Close() }()
	approvals, err := store.ListFactoryWorkItemApprovals(params.WorkItemID)
	if err != nil {
		return backendprotocol.FactoryListApprovalsResult{}, err
	}
	result := backendprotocol.FactoryListApprovalsResult{
		Approvals: make([]backendprotocol.FactoryApproval, 0, len(approvals)),
	}
	for _, approval := range approvals {
		result.Approvals = append(result.Approvals, factoryApprovalToProtocol(approval))
	}
	return result, nil
}

func (p *Provider) updatePendingFactoryWorkItem(
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
	event, err := factoryWorkItemEventFromProtocol(params.Event)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	store, err := state.NewStore(p.storePath)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	defer func() { _ = store.Close() }()
	updated, ok, err := store.UpdatePendingFactoryWorkItem(item, event)
	if err != nil {
		return backendprotocol.FactoryWorkItemResult{}, err
	}
	protocolItem := factoryWorkItemToProtocol(updated)
	return backendprotocol.FactoryWorkItemResult{Item: protocolItem, Created: ok}, nil
}

func factoryApprovalFromProtocol(
	approval backendprotocol.FactoryApproval,
) (state.FactoryApproval, error) {
	approvedAt, err := parseOptionalTime(approval.ApprovedAt, "approved_at")
	if err != nil {
		return state.FactoryApproval{}, err
	}
	return state.FactoryApproval{
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

func factoryApprovalToProtocol(
	approval state.FactoryApproval,
) backendprotocol.FactoryApproval {
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
	setFactoryTime(&result.ApprovedAt, approval.ApprovedAt)
	return result
}
