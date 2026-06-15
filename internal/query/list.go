package query

import (
	"time"

	"github.com/applauselab/bachkator/internal/model"
	statestore "github.com/applauselab/bachkator/internal/state"
)

type RunListOptions struct {
	Target       string
	Status       model.RunStatus
	Since        time.Time
	ArtifactPath string
	Limit        int
}

type RunListRecord struct {
	ID         string
	Target     string
	DryRun     bool
	Force      bool
	Status     model.RunStatus
	StartedAt  time.Time
	FinishedAt time.Time
	LogDir     string
}

type ArtifactListOptions struct {
	RunID  string
	Target string
	Status model.RunStatus
	Since  time.Time
	Path   string
	Limit  int
}

type ArtifactListRecord struct {
	RunID     string
	Target    string
	Kind      string
	Location  string
	CreatedAt time.Time
}

type RunListStore interface {
	ListRuns(statestore.RunQuery) ([]statestore.RunRecord, error)
}

type ArtifactListStore interface {
	ListArtifacts(statestore.ArtifactQuery) ([]statestore.ArtifactRecord, error)
}

func ListRuns(store RunListStore, opts RunListOptions) ([]RunListRecord, error) {
	runs, err := store.ListRuns(statestore.RunQuery{
		Target:       opts.Target,
		Status:       opts.Status,
		Since:        opts.Since,
		ArtifactPath: opts.ArtifactPath,
		Limit:        opts.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]RunListRecord, 0, len(runs))
	for _, run := range runs {
		out = append(out, RunListRecord{
			ID:         run.ID,
			Target:     run.Target,
			DryRun:     run.DryRun,
			Force:      run.Force,
			Status:     run.Status,
			StartedAt:  run.StartedAt,
			FinishedAt: run.FinishedAt,
			LogDir:     run.LogDir,
		})
	}
	return out, nil
}

func ListArtifacts(
	store ArtifactListStore,
	opts ArtifactListOptions,
) ([]ArtifactListRecord, error) {
	artifacts, err := store.ListArtifacts(statestore.ArtifactQuery{
		RunID:  opts.RunID,
		Target: opts.Target,
		Status: opts.Status,
		Since:  opts.Since,
		Path:   opts.Path,
		Limit:  opts.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ArtifactListRecord, 0, len(artifacts))
	for _, artifact := range artifacts {
		location := artifact.Path
		if location == "" {
			location = artifact.Value
		}
		out = append(out, ArtifactListRecord{
			RunID:     artifact.RunID,
			Target:    artifact.Target,
			Kind:      artifact.Kind,
			Location:  location,
			CreatedAt: artifact.CreatedAt,
		})
	}
	return out, nil
}
