package factory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/bacherr"
	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/evidence"
	"github.com/applauselab/bachkator/internal/id"
	"github.com/applauselab/bachkator/internal/model"
)

const (
	WorkItemPhasePlan = "plan"
	SourceManual      = "manual"
	EventSubmitted    = "submitted"
	EventDeduped      = "deduped"
	EventCancelled    = "cancelled"
	EventApproved     = "approved"
	maxBodyBytes      = 1024 * 1024
)

type WorkItem struct {
	ID                 string
	Factory            string
	Workflow           string
	Lifecycle          model.Lifecycle
	CurrentPhase       string
	Title              string
	Body               string
	BodyHash           string
	Priority           model.Priority
	Labels             []string
	SourceType         string
	DedupeKey          string
	SubmittedPlanPath  string
	SubmittedPlanHash  string
	IntakeEvidenceID   string
	IntakeEvidenceURI  string
	IntakeEvidenceHash string
	Metadata           map[string]string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CancelledAt        time.Time
	CancelReason       string
	FailurePhase       string
	FailureMessage     string
	Attempts           []WorkItemAttempt
	Events             []WorkItemEvent
	Approvals          []Approval
}

type Approval struct {
	ID             string
	Factory        string
	Workflow       string
	WorkItemID     string
	AttemptID      string
	Phase          string
	PlanPath       string
	PlanHash       string
	ApprovedAt     time.Time
	Approver       string
	ApproverSource string
	Reason         string
	Metadata       map[string]string
}

type WorkItemAttempt struct {
	ID                string
	WorkItemID        string
	AttemptNumber     int
	Status            model.Lifecycle
	StartPhase        string
	SubmittedPlanPath string
	SubmittedPlanHash string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	FinishedAt        time.Time
}

type WorkItemEvent struct {
	ID         string
	WorkItemID string
	AttemptID  string
	Type       string
	Message    string
	Metadata   map[string]string
	CreatedAt  time.Time
}

type WorkItemQuery struct {
	Factory string
	ID      string
	Status  string
}

type Queue interface {
	Enqueue(
		context.Context,
		WorkItem,
		WorkItemAttempt,
		WorkItemEvent,
		WorkItemEvent,
	) (WorkItem, bool, error)
	UpdatePending(
		ctx context.Context,
		item WorkItem,
		event WorkItemEvent,
	) (WorkItem, bool, error)
	Get(ctx context.Context, factory string, id string) (WorkItem, bool, error)
	List(ctx context.Context, query WorkItemQuery) ([]WorkItem, error)
	Cancel(
		ctx context.Context,
		factory string,
		id string,
		reason string,
		cancelledAt time.Time,
		event WorkItemEvent,
	) (WorkItem, bool, error)
	RecordApproval(
		ctx context.Context,
		approval Approval,
		event WorkItemEvent,
	) (Approval, bool, error)
	ListApprovals(ctx context.Context, workItemID string) ([]Approval, error)
}

type Service struct {
	Root  string
	Queue Queue
	NewID func() (string, error)
	Now   clock.NowFunc
}

type SubmitOptions struct {
	Factory            string
	Workflow           string
	Title              string
	Body               string
	BodyFile           string
	Priority           model.Priority
	Labels             []string
	DedupeKey          string
	SubmittedPlanPath  string
	SubmittedPlanHash  string
	IntakeEvidenceID   string
	IntakeEvidenceURI  string
	IntakeEvidenceHash string
	CreatedAt          time.Time
}

type SubmitResult struct {
	Item    WorkItem `json:"item"`
	Created bool     `json:"created"`
}

type ListOptions struct {
	Factory  string
	Workflow string
	Status   model.Lifecycle
}

type CancelOptions struct {
	Factory string
	ID      string
	Reason  string
}

type ApproveOptions struct {
	Factory        string
	ID             string
	Phase          string
	PlanPath       string
	PlanHash       string
	Reason         string
	Approver       string
	ApproverSource string
}

type ApproveResult struct {
	Approval Approval
	Existing bool
}

func (s Service) Submit(ctx context.Context, opts SubmitOptions) (SubmitResult, error) {
	if err := s.validate(); err != nil {
		return SubmitResult{}, err
	}
	if err := validateSubmitOptions(opts); err != nil {
		return SubmitResult{}, err
	}
	body, err := s.resolveBody(opts)
	if err != nil {
		return SubmitResult{}, err
	}
	if len([]byte(body)) > maxBodyBytes {
		return SubmitResult{}, fmt.Errorf("body exceeds %d byte limit", maxBodyBytes)
	}
	itemID, err := s.newID()
	if err != nil {
		return SubmitResult{}, err
	}
	evidenceID, err := s.newID()
	if err != nil {
		return SubmitResult{}, err
	}
	attemptID, err := s.newID()
	if err != nil {
		return SubmitResult{}, err
	}
	eventID, err := s.newID()
	if err != nil {
		return SubmitResult{}, err
	}
	dedupeEventID := ""
	if opts.DedupeKey != "" {
		dedupeEventID, err = s.newID()
		if err != nil {
			return SubmitResult{}, err
		}
	}
	now := s.now()
	if !opts.CreatedAt.IsZero() {
		now = opts.CreatedAt
	}
	priority := opts.Priority
	if priority == "" {
		priority = model.PriorityNormal
	}
	bodyHash := sha256Text(body)
	intakeURI := filepath.ToSlash(
		filepath.Join(".bach", "artifacts", "factory", itemID, "intake.json"),
	)
	intake := intakeSnapshot{
		SchemaVersion:     "bach.factory.intake.v1",
		WorkItemID:        itemID,
		Factory:           opts.Factory,
		Workflow:          opts.Workflow,
		Title:             opts.Title,
		Body:              body,
		BodyHash:          bodyHash,
		Priority:          priority,
		Labels:            append([]string(nil), opts.Labels...),
		SourceType:        SourceManual,
		DedupeKey:         opts.DedupeKey,
		SubmittedPlanPath: opts.SubmittedPlanPath,
		CreatedAt:         now.UTC().Format(time.RFC3339Nano),
	}
	intakeData, intakeHash, err := marshalIntake(intake)
	if err != nil {
		return SubmitResult{}, err
	}
	intakePath := filepath.Join(s.Root, filepath.FromSlash(intakeURI))
	if err := evidence.WritePrivateFile(intakePath, intakeData); err != nil {
		return SubmitResult{}, err
	}
	item := WorkItem{
		ID:                 itemID,
		Factory:            opts.Factory,
		Workflow:           opts.Workflow,
		Lifecycle:          model.LifecyclePending,
		CurrentPhase:       WorkItemPhasePlan,
		Title:              opts.Title,
		Body:               body,
		BodyHash:           bodyHash,
		Priority:           priority,
		Labels:             append([]string(nil), opts.Labels...),
		SourceType:         SourceManual,
		DedupeKey:          opts.DedupeKey,
		SubmittedPlanPath:  opts.SubmittedPlanPath,
		SubmittedPlanHash:  opts.SubmittedPlanHash,
		IntakeEvidenceID:   firstNonEmpty(opts.IntakeEvidenceID, evidenceID),
		IntakeEvidenceURI:  firstNonEmpty(opts.IntakeEvidenceURI, intakeURI),
		IntakeEvidenceHash: firstNonEmpty(opts.IntakeEvidenceHash, intakeHash),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	attempt := WorkItemAttempt{
		ID:                attemptID,
		WorkItemID:        itemID,
		AttemptNumber:     1,
		Status:            model.LifecyclePending,
		StartPhase:        WorkItemPhasePlan,
		SubmittedPlanPath: opts.SubmittedPlanPath,
		SubmittedPlanHash: opts.SubmittedPlanHash,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	event := WorkItemEvent{
		ID:         eventID,
		WorkItemID: itemID,
		AttemptID:  attemptID,
		Type:       EventSubmitted,
		Message:    "manual factory work item submitted",
		CreatedAt:  now,
	}
	dedupeEvent := WorkItemEvent{}
	if opts.DedupeKey != "" {
		dedupeEvent = WorkItemEvent{
			ID:        dedupeEventID,
			Type:      EventDeduped,
			Message:   "manual factory work item deduped",
			CreatedAt: now,
		}
	}
	createdItem, created, err := s.Queue.Enqueue(ctx, item, attempt, event, dedupeEvent)
	if err != nil {
		_ = os.Remove(intakePath)
		return SubmitResult{}, err
	}
	if !created {
		_ = os.Remove(intakePath)
	}
	return SubmitResult{Item: createdItem, Created: created}, nil
}

func (s Service) Get(ctx context.Context, factory string, id string) (WorkItem, error) {
	if err := s.validate(); err != nil {
		return WorkItem{}, err
	}
	if factory == "" || id == "" {
		return WorkItem{}, bacherr.ValidationFailedf("factory and work item id are required")
	}
	item, ok, err := s.Queue.Get(ctx, factory, id)
	if err != nil {
		return WorkItem{}, err
	}
	if !ok {
		return WorkItem{}, bacherr.NotFoundf("factory work item %q", id)
	}
	return item, nil
}

func (s Service) List(ctx context.Context, opts ListOptions) ([]WorkItem, error) {
	if err := s.validate(); err != nil {
		return nil, err
	}
	if opts.Factory == "" {
		return nil, bacherr.ValidationFailedf("factory is required")
	}
	items, err := s.Queue.List(ctx, WorkItemQuery{
		Factory: opts.Factory,
		Status:  string(opts.Status),
	})
	if err != nil {
		return nil, err
	}
	if opts.Workflow == "" {
		return items, nil
	}
	filtered := make([]WorkItem, 0, len(items))
	for _, item := range items {
		if item.Workflow == opts.Workflow {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (s Service) Cancel(ctx context.Context, opts CancelOptions) (WorkItem, error) {
	if err := s.validate(); err != nil {
		return WorkItem{}, err
	}
	if opts.Factory == "" || opts.ID == "" {
		return WorkItem{}, bacherr.ValidationFailedf("factory and work item id are required")
	}
	if strings.TrimSpace(opts.Reason) == "" {
		return WorkItem{}, bacherr.ValidationFailedf("cancel reason is required")
	}
	eventID, err := s.newID()
	if err != nil {
		return WorkItem{}, err
	}
	now := s.now()
	item, ok, err := s.Queue.Cancel(
		ctx,
		opts.Factory,
		opts.ID,
		opts.Reason,
		now,
		WorkItemEvent{
			ID:        eventID,
			Type:      EventCancelled,
			Message:   opts.Reason,
			CreatedAt: now,
		},
	)
	if err != nil {
		return WorkItem{}, err
	}
	if !ok {
		return WorkItem{}, bacherr.NotFoundf("factory work item %q", opts.ID)
	}
	return item, nil
}

func (s Service) validate() error {
	if s.Root == "" {
		return fmt.Errorf("factory service root is empty")
	}
	if s.Queue == nil {
		return fmt.Errorf("factory queue dependency is not configured")
	}
	return nil
}

func (s Service) resolveBody(opts SubmitOptions) (string, error) {
	if opts.Body != "" && opts.BodyFile != "" {
		return "", bacherr.ValidationFailedf("--body and --body-file cannot both be set")
	}
	if opts.BodyFile == "" {
		return opts.Body, nil
	}
	path, err := evidence.ResolveProjectFile(s.Root, opts.BodyFile)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > maxBodyBytes {
		return "", bacherr.ValidationFailedf("body file exceeds %d byte limit", maxBodyBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s Service) newID() (string, error) {
	if s.NewID != nil {
		return s.NewID()
	}
	return id.New()
}

func (s Service) now() time.Time {
	return clock.UTC(s.Now)
}

func validateSubmitOptions(opts SubmitOptions) error {
	if opts.Factory == "" {
		return bacherr.ValidationFailedf("factory is required")
	}
	if opts.Workflow == "" {
		return bacherr.ValidationFailedf("workflow is required")
	}
	if strings.TrimSpace(opts.Title) == "" {
		return bacherr.ValidationFailedf("title is required")
	}
	return nil
}
