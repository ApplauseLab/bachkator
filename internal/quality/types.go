package quality

import (
	"time"

	"github.com/applauselab/bachkator/internal/model"
)

type Report struct {
	ID        int64
	RunID     string
	Target    string
	Kind      string
	Format    string
	Path      string
	Status    model.RunStatus
	Message   string
	CreatedAt time.Time
	Metrics   []Metric
	Findings  []Finding
}

type Metric struct {
	Name  string
	Scope string
	Value float64
	Unit  string
}

type Finding struct {
	Kind       string
	File       string
	Line       int
	Severity   string
	Rule       string
	Message    string
	DurationMS float64
}

type GateResult struct {
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
