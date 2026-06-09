package state

import "time"

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
	Status     string
	StartedAt  time.Time
	FinishedAt time.Time
	LogDir     string
	Targets    map[string]TargetRunRecord
	Artifacts  []ArtifactRecord
}

type TargetRunRecord struct {
	Status     string
	StartedAt  time.Time
	FinishedAt time.Time
	LogPath    string
	Operation  string
}

type ArtifactRecord struct {
	RunID     string
	Target    string
	Kind      string
	Path      string
	Value     string
	CreatedAt time.Time
}

type QualityReport struct {
	ID        int64
	RunID     string
	Target    string
	Kind      string
	Format    string
	Path      string
	Status    string
	Message   string
	CreatedAt time.Time
	Metrics   []QualityMetric
	Findings  []QualityFinding
}

type QualityMetric struct {
	Name  string
	Scope string
	Value float64
	Unit  string
}

type QualityFinding struct {
	Kind       string
	File       string
	Line       int
	Severity   string
	Rule       string
	Message    string
	DurationMS float64
}

type QualityGateResult struct {
	RunID     string
	Target    string
	Metric    string
	Op        string
	Threshold float64
	Actual    float64
	Status    string
	Message   string
	CreatedAt time.Time
}
