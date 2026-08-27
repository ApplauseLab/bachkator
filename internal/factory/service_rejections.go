package factory

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/applauselab/bachkator/internal/bacherr"
)

// MaxRejections is the number of rejection-driven retries a work item gets
// before the factory fails it with the last rejection reason.
const MaxRejections = 3

type RejectOptions struct {
	Factory        string
	ID             string
	Phase          string
	Reason         string
	Approver       string
	ApproverSource string
	MaxRejections  int
}

type RejectResult struct {
	Rejected   bool
	Exhausted  bool
	RetryID    string
	RetryCount int
	Reason     string
}

// Reject records reviewer feedback on a waiting Work Item and drives the
// rejection retry loop: the waiting item fails with the reason and a fresh
// item is enqueued at the plan phase carrying the feedback, until
// MaxRejections is exhausted.
func (s Service) Reject(ctx context.Context, opts RejectOptions) (RejectResult, error) {
	if err := s.validate(); err != nil {
		return RejectResult{}, err
	}
	if opts.Factory == "" || opts.ID == "" || opts.Phase == "" {
		return RejectResult{}, bacherr.ValidationFailedf(
			"factory, work item id, and phase are required",
		)
	}
	reason := strings.TrimSpace(opts.Reason)
	if reason == "" {
		return RejectResult{}, bacherr.ValidationFailedf(
			"a rejection reason is required; write what should change",
		)
	}
	item, err := s.Get(ctx, opts.Factory, opts.ID)
	if err != nil {
		return RejectResult{}, err
	}
	if item.Lifecycle != "waiting_approval" || item.CurrentPhase != opts.Phase {
		return RejectResult{}, bacherr.ValidationFailedf(
			"work item %q is not waiting for approval at phase %q",
			opts.ID,
			opts.Phase,
		)
	}
	count := 0
	if raw, ok := item.Metadata["rejection_count"]; ok {
		if parsed, err := strconv.Atoi(raw); err == nil {
			count = parsed
		}
	}
	max := opts.MaxRejections
	if max <= 0 {
		max = MaxRejections
	}
	now := s.now()
	if count >= max {
		_, _, err = s.Queue.Fail(
			ctx,
			opts.Factory,
			opts.ID,
			opts.Phase,
			fmt.Sprintf("rejections exhausted (%d); last reason: %s", count, reason),
			now,
		)
		if err != nil {
			return RejectResult{}, err
		}
		return RejectResult{
			Exhausted:  true,
			RetryCount: count,
			Reason:     reason,
		}, nil
	}
	feedback := fmt.Sprintf(
		"\n\n## Reviewer feedback (rejection %d)\n\n%s\n",
		count+1,
		reason,
	)
	metadata := map[string]string{}
	for k, v := range item.Metadata {
		metadata[k] = v
	}
	metadata["rejection_count"] = strconv.Itoa(count + 1)
	metadata["rejection_reason"] = reason
	metadata["retry_of"] = item.ID
	result, err := s.ProviderIntake(ctx, ProviderIntakeOptions{
		Factory:    opts.Factory,
		Trigger:    "rejection-retry",
		Workflow:   item.Workflow,
		SourceType: "rejection",
		SourceID:   fmt.Sprintf("%s#%d", item.ID, count+1),
		Title:      item.Title,
		Body:       strings.TrimSpace(item.Body) + "\n" + feedback,
		Labels:     item.Labels,
		Priority:   item.Priority,
		Metadata:   metadata,
		CreatedAt:  now,
	})
	if err != nil {
		return RejectResult{}, err
	}
	if _, _, err = s.Queue.Fail(
		ctx,
		opts.Factory,
		opts.ID,
		opts.Phase,
		fmt.Sprintf("rejected: %s (retry %s enqueued)", reason, result.WorkItemID),
		now,
	); err != nil {
		return RejectResult{}, err
	}
	return RejectResult{
		Rejected:   true,
		RetryID:    result.WorkItemID,
		RetryCount: count + 1,
		Reason:     reason,
	}, nil
}
