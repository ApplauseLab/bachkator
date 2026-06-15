package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	factorypkg "github.com/applauselab/bachkator/internal/factory"
	"github.com/applauselab/bachkator/internal/factorydaemon"
	"github.com/applauselab/bachkator/internal/model"
)

type factorySubmitView struct {
	Item    factoryWorkItemView `json:"item"`
	Created bool                `json:"created"`
}

type factoryListView struct {
	Items []factoryWorkItemView `json:"items"`
}

type factoryStartView struct {
	DaemonID string                 `json:"daemon_id"`
	Lease    factoryDaemonLeaseView `json:"lease"`
}

type factoryStatusView struct {
	Lease           factoryDaemonLeaseView  `json:"lease"`
	HasActiveItem   bool                    `json:"has_active_item"`
	ActiveItemID    string                  `json:"active_item_id,omitempty"`
	LifecycleCounts map[model.Lifecycle]int `json:"lifecycle_counts"`
}

type factoryDaemonLeaseView struct {
	DaemonID   string `json:"daemon_id,omitempty"`
	Factory    string `json:"factory,omitempty"`
	Hostname   string `json:"hostname,omitempty"`
	PID        int    `json:"pid,omitempty"`
	AcquiredAt string `json:"acquired_at,omitempty"`
	RenewedAt  string `json:"renewed_at,omitempty"`
	ExpiresAt  string `json:"expires_at,omitempty"`
	ReleasedAt string `json:"released_at,omitempty"`
	Status     string `json:"status,omitempty"`
}

type factoryWorkItemView struct {
	ID                 string                     `json:"id"`
	Factory            string                     `json:"factory"`
	Workflow           string                     `json:"workflow"`
	Lifecycle          model.Lifecycle            `json:"lifecycle"`
	CurrentPhase       string                     `json:"current_phase"`
	Title              string                     `json:"title"`
	BodyHash           string                     `json:"body_hash,omitempty"`
	Priority           model.Priority             `json:"priority"`
	Labels             []string                   `json:"labels,omitempty"`
	SourceType         string                     `json:"source_type"`
	DedupeKey          string                     `json:"dedupe_key,omitempty"`
	SubmittedPlanPath  string                     `json:"submitted_plan_path,omitempty"`
	SubmittedPlanHash  string                     `json:"submitted_plan_hash,omitempty"`
	IntakeEvidenceID   string                     `json:"intake_evidence_id,omitempty"`
	IntakeEvidenceURI  string                     `json:"intake_evidence_uri,omitempty"`
	IntakeEvidenceHash string                     `json:"intake_evidence_hash,omitempty"`
	CreatedAt          string                     `json:"created_at"`
	UpdatedAt          string                     `json:"updated_at"`
	CancelledAt        string                     `json:"cancelled_at,omitempty"`
	CancelReason       string                     `json:"cancel_reason,omitempty"`
	FailurePhase       string                     `json:"failure_phase,omitempty"`
	FailureMessage     string                     `json:"failure_message,omitempty"`
	Attempts           []factoryWorkAttemptView   `json:"attempts,omitempty"`
	Events             []factoryWorkItemEventView `json:"events,omitempty"`
	Approvals          []factoryApprovalView      `json:"approvals"`
}

type factoryApprovalView struct {
	ID             string `json:"approval_id"`
	Factory        string `json:"factory"`
	Workflow       string `json:"workflow"`
	WorkItemID     string `json:"work_item_id"`
	AttemptID      string `json:"attempt_id"`
	Phase          string `json:"phase"`
	PlanPath       string `json:"plan_path,omitempty"`
	PlanHash       string `json:"plan_hash,omitempty"`
	ApprovedAt     string `json:"approved_at"`
	Approver       string `json:"approver,omitempty"`
	ApproverSource string `json:"approver_source,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

type factoryApproveView struct {
	Approval factoryApprovalView `json:"approval"`
	Existing bool                `json:"existing"`
}

type factoryWorkAttemptView struct {
	ID                string          `json:"id"`
	WorkItemID        string          `json:"work_item_id"`
	AttemptNumber     int             `json:"attempt_number"`
	Status            model.Lifecycle `json:"status"`
	StartPhase        string          `json:"start_phase"`
	SubmittedPlanPath string          `json:"submitted_plan_path,omitempty"`
	SubmittedPlanHash string          `json:"submitted_plan_hash,omitempty"`
	CreatedAt         string          `json:"created_at"`
	UpdatedAt         string          `json:"updated_at"`
	FinishedAt        string          `json:"finished_at,omitempty"`
}

type factoryWorkItemEventView struct {
	ID         string `json:"id"`
	WorkItemID string `json:"work_item_id"`
	AttemptID  string `json:"attempt_id,omitempty"`
	Type       string `json:"type"`
	Message    string `json:"message,omitempty"`
	CreatedAt  string `json:"created_at"`
}

func factoryWorkItemViewFor(item factorypkg.WorkItem) factoryWorkItemView {
	view := factoryWorkItemView{
		ID:                 item.ID,
		Factory:            item.Factory,
		Workflow:           item.Workflow,
		Lifecycle:          item.Lifecycle,
		CurrentPhase:       item.CurrentPhase,
		Title:              item.Title,
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
		CreatedAt:          formatFactoryTime(item.CreatedAt),
		UpdatedAt:          formatFactoryTime(item.UpdatedAt),
		CancelReason:       item.CancelReason,
		FailurePhase:       item.FailurePhase,
		FailureMessage:     item.FailureMessage,
		Attempts:           make([]factoryWorkAttemptView, 0, len(item.Attempts)),
		Events:             make([]factoryWorkItemEventView, 0, len(item.Events)),
		Approvals:          make([]factoryApprovalView, 0, len(item.Approvals)),
	}
	if !item.CancelledAt.IsZero() {
		view.CancelledAt = formatFactoryTime(item.CancelledAt)
	}
	for _, attempt := range item.Attempts {
		view.Attempts = append(view.Attempts, factoryWorkAttemptView{
			ID:                attempt.ID,
			WorkItemID:        attempt.WorkItemID,
			AttemptNumber:     attempt.AttemptNumber,
			Status:            attempt.Status,
			StartPhase:        attempt.StartPhase,
			SubmittedPlanPath: attempt.SubmittedPlanPath,
			SubmittedPlanHash: attempt.SubmittedPlanHash,
			CreatedAt:         formatFactoryTime(attempt.CreatedAt),
			UpdatedAt:         formatFactoryTime(attempt.UpdatedAt),
			FinishedAt:        formatFactoryTime(attempt.FinishedAt),
		})
	}
	for _, event := range item.Events {
		view.Events = append(view.Events, factoryWorkItemEventView{
			ID:         event.ID,
			WorkItemID: event.WorkItemID,
			AttemptID:  event.AttemptID,
			Type:       event.Type,
			Message:    event.Message,
			CreatedAt:  formatFactoryTime(event.CreatedAt),
		})
	}
	for _, approval := range item.Approvals {
		view.Approvals = append(view.Approvals, factoryApprovalViewFor(approval))
	}
	return view
}

func factoryApprovalViewFor(approval factorypkg.Approval) factoryApprovalView {
	view := factoryApprovalView{
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
	}
	if !approval.ApprovedAt.IsZero() {
		view.ApprovedAt = formatFactoryTime(approval.ApprovedAt)
	}
	return view
}

func factoryStatusViewFor(status factorydaemon.StatusResultStatus) factoryStatusView {
	view := factoryStatusView{
		Lease:           factoryDaemonLeaseViewFor(status.Lease),
		HasActiveItem:   status.HasActiveItem,
		LifecycleCounts: status.LifecycleCounts,
	}
	if status.HasActiveItem {
		view.ActiveItemID = status.ActiveItem.ID
	}
	return view
}

func factoryDaemonLeaseViewFor(lease factorydaemon.StatusResultLease) factoryDaemonLeaseView {
	view := factoryDaemonLeaseView{
		DaemonID: lease.DaemonID,
		Factory:  lease.Factory,
		Hostname: lease.Hostname,
		PID:      lease.PID,
		Status:   lease.Status,
	}
	if !lease.AcquiredAt.IsZero() {
		view.AcquiredAt = formatFactoryTime(lease.AcquiredAt)
	}
	if !lease.RenewedAt.IsZero() {
		view.RenewedAt = formatFactoryTime(lease.RenewedAt)
	}
	if !lease.ExpiresAt.IsZero() {
		view.ExpiresAt = formatFactoryTime(lease.ExpiresAt)
	}
	if !lease.ReleasedAt.IsZero() {
		view.ReleasedAt = formatFactoryTime(lease.ReleasedAt)
	}
	return view
}

func formatFactoryInspection(stdout io.Writer, item factorypkg.WorkItem) error {
	if _, err := fmt.Fprintf(
		stdout,
		"work item %s %s factory=%s workflow=%s phase=%s\n",
		item.ID,
		item.Lifecycle,
		item.Factory,
		item.Workflow,
		item.CurrentPhase,
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "title: %s\n", item.Title); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "priority: %s\n", item.Priority); err != nil {
		return err
	}
	if len(item.Labels) > 0 {
		if _, err := fmt.Fprintf(
			stdout,
			"labels: %s\n",
			strings.Join(item.Labels, ","),
		); err != nil {
			return err
		}
	}
	if item.IntakeEvidenceURI != "" {
		if _, err := fmt.Fprintf(stdout, "intake: %s\n", item.IntakeEvidenceURI); err != nil {
			return err
		}
	}
	if item.CancelReason != "" {
		if _, err := fmt.Fprintf(stdout, "cancel reason: %s\n", item.CancelReason); err != nil {
			return err
		}
	}
	if len(item.Approvals) > 0 {
		if _, err := fmt.Fprintf(stdout, "approvals:\n"); err != nil {
			return err
		}
		for _, a := range item.Approvals {
			if _, err := fmt.Fprintf(
				stdout,
				"  - %s phase=%s approver=%s source=%s at=%s\n",
				a.ID,
				a.Phase,
				a.Approver,
				a.ApproverSource,
				formatFactoryTime(a.ApprovedAt),
			); err != nil {
				return err
			}
			if a.Reason != "" {
				if _, err := fmt.Fprintf(stdout, "    reason: %s\n", a.Reason); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func writeFactoryJSON(stdout io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(stdout, "%s\n", data)
	return err
}

func formatFactoryTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}
