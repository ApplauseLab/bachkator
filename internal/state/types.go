package state

import (
	"time"

	"github.com/applauselab/bachkator/internal/model"
	"github.com/applauselab/bachkator/internal/quality"
)

type State struct {
	Version int
	Targets map[string]Record
	Runs    []RunRecord
}

type Record struct {
	Fingerprint      string
	FingerprintParts map[string]string
	CompletedAt      time.Time
}

type RunRecord struct {
	ID         string
	Target     string
	DryRun     bool
	Force      bool
	Status     model.RunStatus
	StartedAt  time.Time
	FinishedAt time.Time
	LogDir     string
	Targets    map[string]TargetRunRecord
	Artifacts  []ArtifactRecord
}

type TargetRunRecord struct {
	Status     model.RunStatus
	StartedAt  time.Time
	FinishedAt time.Time
	LogPath    string
	Operation  string
	ExitCode   *int
}

type ArtifactRecord struct {
	RunID     string
	Target    string
	Kind      string
	Path      string
	Value     string
	CreatedAt time.Time
}

type QualityReport = quality.Report
type QualityMetric = quality.Metric
type QualityFinding = quality.Finding
type QualityGateResult = quality.GateResult
