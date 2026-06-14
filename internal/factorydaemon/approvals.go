package factorydaemon

import (
	"strconv"
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/config"
	factorypkg "github.com/applauselab/bachkator/internal/factory"
	"github.com/applauselab/bachkator/pkg/approvalprotocol"
)

const approverSourceProvider = "provider"

type approvalPoller struct {
	service        Service
	factoryService factorypkg.Service
	factory        string
	provider       *config.FactoryApprovalProvider
	session        *providerSession[*approvalprotocol.Client]
}

func (s Service) startProviderApprovals(ctx context.Context) <-chan error {
	errCh := make(chan error, 1)
	providers := s.Factory.ApprovalProviders()
	if len(providers) == 0 {
		close(errCh)
		return errCh
	}
	factoryService := factorypkg.Service{
		Root:  s.ConfigProject.Root,
		Queue: factorypkg.BackendQueue{Client: &s.Backend.Factory},
		NewID: s.NewID,
		Now:   s.Now,
	}
	go func() {
		defer close(errCh)
		var wg sync.WaitGroup
		for _, provider := range providers {
			if provider == nil {
				continue
			}
			poller := &approvalPoller{
				service:        s,
				factoryService: factoryService,
				factory:        s.Factory.Name,
				provider:       provider,
				session: newProviderSession(
					provider.Name,
					provider.Command,
					s.ConfigProject.Root,
					approvalprotocol.NewClient,
					newApprovalDial(provider.Name, s.Factory.Name, provider.Config),
				),
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				poller.run(ctx)
			}()
		}
		wg.Wait()
	}()
	return errCh
}

func newApprovalDial(
	name string,
	factory string,
	providerConfig map[string]string,
) func(context.Context, *approvalprotocol.Client) error {
	return func(ctx context.Context, client *approvalprotocol.Client) error {
		result, err := client.Handshake(ctx, approvalprotocol.HandshakeParams{
			Protocol: approvalprotocol.ProtocolVersion,
			Factory:  factory,
			Approval: name,
			Config:   providerConfig,
		})
		if err != nil {
			return err
		}
		if result.Protocol != approvalprotocol.ProtocolVersion {
			return fmt.Errorf(
				"approval provider %q returned unsupported protocol %q",
				name,
				result.Protocol,
			)
		}
		if !hasCapability(result.Capabilities, approvalprotocol.CapabilityPoll) {
			return fmt.Errorf("approval provider %q does not support poll", name)
		}
		return nil
	}
}

func (p *approvalPoller) run(ctx context.Context) {
	defer p.session.close()
	ticker := time.NewTicker(p.provider.PollIntervalDuration())
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		p.poll(ctx)
	}
}

func (p *approvalPoller) poll(ctx context.Context) {
	client, err := p.session.get(ctx)
	if err != nil {
		p.logf("handshake failed: %v", err)
		return
	}
	pollCtx, cancel := context.WithTimeout(ctx, providerCallTimeout)
	result, err := client.Poll(pollCtx, approvalprotocol.PollParams{
		Config: p.provider.Config,
	})
	cancel()
	if err != nil {
		p.logf("poll failed: %v", err)
		p.session.invalidate()
		return
	}
	if len(result.Approvals) == 0 {
		return
	}
	items, err := p.service.Backend.Factory.List(
		ctx,
		backend.FactoryWorkItemQuery{
			Factory: p.factory,
			Status:  "waiting_approval",
		},
	)
	if err != nil {
		p.logf("list waiting work items failed: %v", err)
		return
	}
	for _, record := range result.Approvals {
		if err := p.processRecord(ctx, items, record); err != nil {
			p.logf(
				"rejected approval phase=%s source=%s/%s id=%s: %v",
				record.Phase,
				record.SourceType,
				record.SourceID,
				record.WorkItemID,
				err,
			)
		}
	}
}

// processRecord records one external approval against every waiting Work Item
// it matches. Store-level idempotency makes redelivery safe: an approval that
// was already recorded resolves to the existing approval, and an item whose
// gated phase moved on fails store validation and is skipped until the
// provider stops resending.
func (p *approvalPoller) processRecord(
	ctx context.Context,
	items []backend.FactoryWorkItem,
	record approvalprotocol.ApprovalRecord,
) error {
	matches, err := p.matchingItems(items, record)
	if err != nil {
		return err
	}
	for _, item := range matches {
		if record.Rejected {
			if err := p.rejectItem(ctx, item, record); err != nil {
				return fmt.Errorf("work item %s: %w", item.ID, err)
			}
			continue
		}
		options, err := p.approveOptions(item, record)
		if err != nil {
			return err
		}
		if _, err := p.factoryService.Approve(ctx, options); err != nil {
			return fmt.Errorf("work item %s: %w", item.ID, err)
		}
		p.logApproval(item, record)
	}
	return nil
}

// maxRejections is the number of rejection-driven retries a work item gets
// before the factory gives up and fails it. The count lives in item metadata
// so it survives across retries.
const maxRejections = 3

// rejectItem implements the feedback loop for a rejected gate: fail the
// waiting item and re-enqueue a fresh one at the plan phase carrying the
// rejection reason, until maxRejections is exhausted.
func (p *approvalPoller) rejectItem(
	ctx context.Context,
	item backend.FactoryWorkItem,
	record approvalprotocol.ApprovalRecord,
) error {
	count := 0
	if raw, ok := item.Metadata["rejection_count"]; ok {
		if parsed, err := strconv.Atoi(raw); err == nil {
			count = parsed
		}
	}
	reason := strings.TrimSpace(record.Reason)
	if reason == "" {
		reason = "rejected without a reason"
	}
	if count >= maxRejections {
		_, _, _ = p.service.Backend.Factory.FailWorkItem(
			ctx,
			item.Factory,
			item.ID,
			item.CurrentPhase,
			fmt.Sprintf("rejections exhausted (%d); last reason: %s", count, reason),
			clock.UTC(p.service.Now),
		)
		p.logf("work item %s: rejections exhausted (%d)", item.ID, count)
		return nil
	}
	feedback := fmt.Sprintf(
		"\n\n## Reviewer feedback (rejection %d)\n\n%s\n",
		count+1, reason,
	)
	retry := item
	retry.ID = ""
	retry.Lifecycle = "pending"
	retry.CurrentPhase = "plan"
	retry.Body = strings.TrimSpace(item.Body) + "\n" + feedback
	retry.Metadata = map[string]string{}
	for k, v := range item.Metadata {
		retry.Metadata[k] = v
	}
	retry.Metadata["rejection_count"] = strconv.Itoa(count + 1)
	retry.Metadata["rejection_reason"] = reason
	retry.Metadata["retry_of"] = item.ID
	if _, err := p.factoryService.ProviderIntake(ctx, factorypkg.ProviderIntakeOptions{
		Factory:    item.Factory,
		Trigger:    "rejection-retry",
		Workflow:   item.Workflow,
		SourceType: "rejection",
		SourceID:   fmt.Sprintf("%s#%d", item.ID, count+1),
		Title:      item.Title,
		Body:       retry.Body,
		Labels:     item.Labels,
		Priority:   item.Priority,
		Metadata:   retry.Metadata,
		CreatedAt:  clock.UTC(p.service.Now),
	}); err != nil {
		return err
	}
		_, _, _ = p.service.Backend.Factory.FailWorkItem(
			ctx,
			item.Factory,
			item.ID,
			item.CurrentPhase,
			fmt.Sprintf("rejected: %s (retry %d enqueued)", reason, count+1),
			clock.UTC(p.service.Now),
		)
	p.logf("work item %s rejected; retry %d enqueued with feedback", item.ID, count+1)
	return nil
}

func (p *approvalPoller) matchingItems(
	items []backend.FactoryWorkItem,
	record approvalprotocol.ApprovalRecord,
) ([]backend.FactoryWorkItem, error) {
	if strings.TrimSpace(record.Phase) == "" {
		return nil, fmt.Errorf("phase is required")
	}
	switch {
	case record.WorkItemID != "":
		if record.SourceType != "" || record.SourceID != "" || record.HeadRef != "" {
			return nil, fmt.Errorf(
				"work_item_id, head_ref, and source identity are mutually exclusive",
			)
		}
	case record.HeadRef != "":
		if record.SourceType != "" || record.SourceID != "" {
			return nil, fmt.Errorf(
				"head_ref and source identity are mutually exclusive",
			)
		}
	case record.SourceType != "" && record.SourceID != "":
		// matched by intake source identity below.
	default:
		return nil, fmt.Errorf(
			"either work_item_id, head_ref, or source_type and source_id are required",
		)
	}
	matches := []backend.FactoryWorkItem{}
	for _, item := range items {
		if item.CurrentPhase != record.Phase {
			continue
		}
		switch {
		case record.WorkItemID != "":
			if item.ID == record.WorkItemID {
				matches = append(matches, item)
			}
		case record.HeadRef != "":
			// Factory head branches embed the work item id:
			// bach/factory/<work-item-id>/<lane>.
			if item.Factory == p.factory {
				if branch, err := factoryItemHeadBranch(item.ID); err == nil {
					if branch == record.HeadRef {
						matches = append(matches, item)
					}
				}
			}
		default:
			if item.SourceType == record.SourceType && item.SourceID == record.SourceID {
				matches = append(matches, item)
			}
		}
	}
	return matches, nil
}

// factoryItemHeadBranch returns the plan-PR head branch for a work item under
// the standard factory naming convention.
func factoryItemHeadBranch(itemID string) (string, error) {
	if itemID == "" {
		return "", fmt.Errorf("empty work item id")
	}
	return "bach/factory/" + itemID + "/plan", nil
}

func (p *approvalPoller) approveOptions(
	item backend.FactoryWorkItem,
	record approvalprotocol.ApprovalRecord,
) (factorypkg.ApproveOptions, error) {
	options := factorypkg.ApproveOptions{
		Factory:        p.factory,
		ID:             item.ID,
		Phase:          record.Phase,
		Reason:         record.Reason,
		Approver:       record.Approver,
		ApproverSource: approverSourceProvider,
		Metadata:       record.Metadata,
	}
	workflow, err := p.service.workflow(item.Workflow)
	if err != nil {
		return options, err
	}
	if !approvablePhase(workflow, record.Phase) {
		return options, fmt.Errorf(
			"phase %q does not require approval in workflow %q",
			record.Phase,
			workflow.Name,
		)
	}
	if record.Phase == config.FactoryPhasePlan {
		planPath := interpolate(workflow.Plan[0].Path, item, p.factory, workflow.Name)
		hash, err := p.service.planHash(planPath)
		if err != nil {
			return options, err
		}
		options.PlanPath = planPath
		options.PlanHash = hash
	}
	return options, nil
}

func approvablePhase(workflow *config.FactoryWorkflow, phase string) bool {
	if workflow == nil {
		return false
	}
	if phase == config.FactoryPhasePlan {
		return workflow.PlanRequiresApproval()
	}
	if phase == config.FactoryPhaseImplement {
		return workflow.ImplementRequiresApproval()
	}
	if phase == config.FactoryPhaseMerge {
		return workflow.MergeRequiresApproval()
	}
	if strings.HasPrefix(phase, "deploy.") {
		name := strings.TrimPrefix(phase, "deploy.")
		return workflow.DeployRequiresApproval(name)
	}
	return false
}

func (p *approvalPoller) logApproval(
	item backend.FactoryWorkItem,
	record approvalprotocol.ApprovalRecord,
) {
	p.logf(
		"recorded approval work_item=%s phase=%s approver=%s",
		item.ID,
		record.Phase,
		record.Approver,
	)
}

func (p *approvalPoller) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(
		p.service.stderr(),
		"approval "+p.provider.Name+": "+format+"\n",
		args...,
	)
}
