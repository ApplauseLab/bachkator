package factory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/bacherr"
	"github.com/applauselab/bachkator/internal/evidence"
	"github.com/applauselab/bachkator/internal/model"
)

const (
	SourceProvider        = "provider"
	EventSourceUpdated    = "source_updated"
	EventProviderIntake   = "provider_intake"
	providerBodyLimit     = 1024 * 1024
	providerLabelsLimit   = 100
	providerMetadataLimit = 100
)

type ProviderIntakeOptions struct {
	Factory        string
	Trigger        string
	Workflow       string
	SourceType     string
	SourceID       string
	SourceURL      string
	SourceRevision string
	Title          string
	Body           string
	Labels         []string
	Priority       model.Priority
	Metadata       map[string]string
	CreatedAt      time.Time
}

type ProviderIntakeResult struct {
	Item       WorkItem
	Created    bool
	Updated    bool
	DedupeKey  string
	WorkItemID string
}

func (s Service) ProviderIntake(
	ctx context.Context,
	opts ProviderIntakeOptions,
) (ProviderIntakeResult, error) {
	if err := s.validate(); err != nil {
		return ProviderIntakeResult{}, err
	}
	if err := validateProviderIntakeOptions(opts); err != nil {
		return ProviderIntakeResult{}, err
	}
	body := opts.Body
	if len([]byte(body)) > providerBodyLimit {
		return ProviderIntakeResult{}, fmt.Errorf(
			"provider body exceeds %d byte limit",
			providerBodyLimit,
		)
	}
	labels := normalizeLabels(opts.Labels)
	if len(labels) > providerLabelsLimit {
		return ProviderIntakeResult{}, fmt.Errorf(
			"provider labels exceed %d limit",
			providerLabelsLimit,
		)
	}
	metadata, err := normalizeMetadata(opts.Metadata, providerMetadataLimit)
	if err != nil {
		return ProviderIntakeResult{}, err
	}
	priority := opts.Priority
	if priority == "" {
		priority = model.PriorityNormal
	}
	itemID, err := s.newID()
	if err != nil {
		return ProviderIntakeResult{}, err
	}
	evidenceID, err := s.newID()
	if err != nil {
		return ProviderIntakeResult{}, err
	}
	attemptID, err := s.newID()
	if err != nil {
		return ProviderIntakeResult{}, err
	}
	eventID, err := s.newID()
	if err != nil {
		return ProviderIntakeResult{}, err
	}
	updateEventID, err := s.newID()
	if err != nil {
		return ProviderIntakeResult{}, err
	}
	now := s.now()
	if !opts.CreatedAt.IsZero() {
		now = opts.CreatedAt
	}
	dedupeKey := providerDedupeKey(
		opts.Factory,
		opts.Trigger,
		opts.Workflow,
		opts.SourceType,
		opts.SourceID,
	)
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
		Labels:            append([]string(nil), labels...),
		SourceType:        opts.SourceType,
		SourceID:          opts.SourceID,
		SourceURL:         opts.SourceURL,
		SourceRevision:    opts.SourceRevision,
		DedupeKey:         dedupeKey,
		SubmittedPlanPath: "",
		CreatedAt:         now.UTC().Format(time.RFC3339Nano),
	}
	intakeData, intakeHash, err := marshalIntake(intake)
	if err != nil {
		return ProviderIntakeResult{}, err
	}
	intakePath := filepath.Join(s.Root, filepath.FromSlash(intakeURI))
	if err := evidence.WritePrivateFile(intakePath, intakeData); err != nil {
		return ProviderIntakeResult{}, err
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
		Labels:             append([]string(nil), labels...),
		SourceType:         opts.SourceType,
		DedupeKey:          dedupeKey,
		IntakeEvidenceID:   evidenceID,
		IntakeEvidenceURI:  intakeURI,
		IntakeEvidenceHash: intakeHash,
		Metadata:           metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	attempt := WorkItemAttempt{
		ID:            attemptID,
		WorkItemID:    itemID,
		AttemptNumber: 1,
		Status:        model.LifecyclePending,
		StartPhase:    WorkItemPhasePlan,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	event := WorkItemEvent{
		ID:         eventID,
		WorkItemID: itemID,
		AttemptID:  attemptID,
		Type:       EventProviderIntake,
		Message:    fmt.Sprintf("%s/%s", opts.Trigger, opts.SourceType),
		Metadata: map[string]string{
			"trigger":         opts.Trigger,
			"source_type":     opts.SourceType,
			"source_id":       opts.SourceID,
			"source_revision": opts.SourceRevision,
		},
		CreatedAt: now,
	}
	dedupeEvent := WorkItemEvent{
		ID:      updateEventID,
		Type:    EventSourceUpdated,
		Message: fmt.Sprintf("%s/%s", opts.Trigger, opts.SourceType),
		Metadata: map[string]string{
			"trigger":         opts.Trigger,
			"source_type":     opts.SourceType,
			"source_id":       opts.SourceID,
			"source_revision": opts.SourceRevision,
		},
		CreatedAt: now,
	}
	createdItem, created, err := s.Queue.Enqueue(ctx, item, attempt, event, dedupeEvent)
	if err != nil {
		_ = os.Remove(intakePath)
		return ProviderIntakeResult{}, err
	}
	if created {
		return ProviderIntakeResult{
			Item:       createdItem,
			Created:    true,
			DedupeKey:  dedupeKey,
			WorkItemID: createdItem.ID,
		}, nil
	}
	existingItem, ok, err := s.Queue.Get(ctx, opts.Factory, createdItem.ID)
	if err != nil {
		_ = os.Remove(intakePath)
		return ProviderIntakeResult{}, err
	}
	if !ok {
		_ = os.Remove(intakePath)
		return ProviderIntakeResult{}, bacherr.NotFoundf("deduped work item %q", createdItem.ID)
	}
	if existingItem.Lifecycle != model.LifecyclePending {
		_ = os.Remove(intakePath)
		return ProviderIntakeResult{
			Item:       existingItem,
			Created:    false,
			Updated:    false,
			DedupeKey:  dedupeKey,
			WorkItemID: existingItem.ID,
		}, nil
	}
	if !providerItemChanged(existingItem, item) {
		_ = os.Remove(intakePath)
		return ProviderIntakeResult{
			Item:       existingItem,
			Created:    false,
			Updated:    false,
			DedupeKey:  dedupeKey,
			WorkItemID: existingItem.ID,
		}, nil
	}
	updateItem := item
	updateItem.ID = existingItem.ID
	updateItem.CreatedAt = existingItem.CreatedAt
	updateItem.UpdatedAt = now
	updateEvent := WorkItemEvent{
		ID:         updateEventID,
		WorkItemID: existingItem.ID,
		Type:       EventSourceUpdated,
		Message:    fmt.Sprintf("%s/%s", opts.Trigger, opts.SourceType),
		Metadata: map[string]string{
			"trigger":         opts.Trigger,
			"source_type":     opts.SourceType,
			"source_id":       opts.SourceID,
			"source_revision": opts.SourceRevision,
		},
		CreatedAt: now,
	}
	updatedItem, updated, err := s.Queue.UpdatePending(ctx, updateItem, updateEvent)
	if err != nil {
		_ = os.Remove(intakePath)
		return ProviderIntakeResult{}, err
	}
	if !updated {
		_ = os.Remove(intakePath)
	}
	return ProviderIntakeResult{
		Item:       updatedItem,
		Created:    false,
		Updated:    updated,
		DedupeKey:  dedupeKey,
		WorkItemID: updatedItem.ID,
	}, nil
}

func validateProviderIntakeOptions(opts ProviderIntakeOptions) error {
	if opts.Factory == "" {
		return bacherr.ValidationFailedf("factory is required")
	}
	if opts.Trigger == "" {
		return bacherr.ValidationFailedf("trigger is required")
	}
	if opts.Workflow == "" {
		return bacherr.ValidationFailedf("workflow is required")
	}
	if opts.SourceType == "" {
		return bacherr.ValidationFailedf("source type is required")
	}
	if opts.SourceID == "" {
		return bacherr.ValidationFailedf("source id is required")
	}
	if strings.TrimSpace(opts.Title) == "" {
		return bacherr.ValidationFailedf("title is required")
	}
	return nil
}

func providerDedupeKey(factory, trigger, workflow, sourceType, sourceID string) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", factory, trigger, workflow, sourceType, sourceID)
}

func providerItemChanged(existing, incoming WorkItem) bool {
	if existing.BodyHash != incoming.BodyHash {
		return true
	}
	if existing.Title != incoming.Title {
		return true
	}
	if existing.Priority != incoming.Priority {
		return true
	}
	if !stringSlicesEqual(existing.Labels, incoming.Labels) {
		return true
	}
	if existing.Metadata["source_revision"] != incoming.Metadata["source_revision"] {
		return true
	}
	return false
}

func normalizeLabels(labels []string) []string {
	result := make([]string, 0, len(labels))
	seen := map[string]struct{}{}
	for _, l := range labels {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if _, ok := seen[l]; ok {
			continue
		}
		seen[l] = struct{}{}
		result = append(result, l)
	}
	sort.Strings(result)
	return result
}

func normalizeMetadata(metadata map[string]string, limit int) (map[string]string, error) {
	if len(metadata) > limit {
		return nil, fmt.Errorf("metadata exceeds %d entries", limit)
	}
	result := make(map[string]string, len(metadata))
	for k, v := range metadata {
		if strings.TrimSpace(k) == "" {
			continue
		}
		result[k] = v
	}
	return result, nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
