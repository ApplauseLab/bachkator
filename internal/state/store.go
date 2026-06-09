package state

import (
	"sort"
	"strings"
	"time"
)

type Store struct {
	path string
}

type RunQuery struct {
	Target       string
	Status       string
	Since        time.Time
	ArtifactPath string
	Limit        int
}

func NewStore(path string) Store {
	return Store{path: path}
}

func (s Store) Load() (*State, error) {
	return Load(s.path)
}

func (s Store) LoadReadOnly() (*State, error) {
	return LoadReadOnly(s.path)
}

func (s Store) Save(snapshot *State) error {
	return Save(s.path, snapshot)
}

func (s Store) SaveTargetFingerprints(targets map[string]Record) error {
	return SaveSnapshot(s.path, targets, nil)
}

func (s Store) RecordRunCompletion(targets map[string]Record, run RunRecord) error {
	return SaveSnapshot(s.path, targets, []RunRecord{run})
}

func (s Store) SaveQualityReports(reports []QualityReport, gates []QualityGateResult) error {
	return SaveQualityReports(s.path, reports, gates)
}

func (s Store) ListRuns(query RunQuery) ([]RunRecord, error) {
	snapshot, err := s.Load()
	if err != nil {
		return nil, err
	}
	runs := []RunRecord{}
	for _, run := range snapshot.Runs {
		if query.Target != "" && run.Target != query.Target &&
			run.Targets[query.Target].Status == "" {
			continue
		}
		if query.Status != "" && run.Status != query.Status {
			continue
		}
		if !query.Since.IsZero() && run.StartedAt.Before(query.Since) {
			continue
		}
		if query.ArtifactPath != "" && !runHasArtifactPath(run, query.ArtifactPath) {
			continue
		}
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	if query.Limit > 0 && len(runs) > query.Limit {
		runs = runs[:query.Limit]
	}
	return runs, nil
}

func (s Store) ListArtifacts(query ArtifactQuery) ([]ArtifactRecord, error) {
	return ListArtifacts(s.path, query)
}

func (s Store) ListQualityReports(limit int) ([]QualityReport, error) {
	return ListQualityReports(s.path, limit)
}

func (s Store) ListQualityMetrics(limit int) ([]QualityMetric, error) {
	return ListQualityMetrics(s.path, limit)
}

func (s Store) ListQualityFindings(limit int) ([]QualityFinding, error) {
	return ListQualityFindings(s.path, limit)
}

func (s Store) ListQualityGateResults(limit int) ([]QualityGateResult, error) {
	return ListQualityGateResults(s.path, limit)
}

func runHasArtifactPath(run RunRecord, path string) bool {
	for _, artifact := range run.Artifacts {
		if strings.Contains(artifact.Path, path) {
			return true
		}
	}
	return false
}
