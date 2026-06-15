package state

import (
	"database/sql"
	"errors"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/model"
)

func errReadOnlyStore() error {
	return errors.New("store is closed or read-only")
}

type Store struct {
	db *sql.DB
}

type RunQuery struct {
	Target       string
	Status       model.RunStatus
	Since        time.Time
	ArtifactPath string
	Limit        int
}

func NewStore(path string) (*Store, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func OpenReadOnlyStore(path string) (*Store, error) {
	db, err := openReadOnlyDB(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Store{db: nil}, nil
		}
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Load() (*State, error) {
	if s.db == nil {
		return newEmptyState(), nil
	}
	return s.loadFromDB()
}

func (s *Store) LoadReadOnly() (*State, error) {
	return s.Load()
}

func (s *Store) Save(snapshot *State) error {
	return s.SaveSnapshot(snapshot.Targets, snapshot.Runs)
}

func (s *Store) SaveTargetFingerprints(targets map[string]Record) error {
	return s.SaveSnapshot(targets, nil)
}

func (s *Store) RecordRunStart(run RunRecord) error {
	return s.SaveSnapshot(nil, []RunRecord{run})
}

func (s *Store) RecordRunCompletion(targets map[string]Record, run RunRecord) error {
	return s.SaveSnapshot(targets, []RunRecord{run})
}

func (s *Store) ListRuns(query RunQuery) ([]RunRecord, error) {
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

func runHasArtifactPath(run RunRecord, path string) bool {
	for _, artifact := range run.Artifacts {
		if strings.Contains(artifact.Path, path) {
			return true
		}
	}
	return false
}
