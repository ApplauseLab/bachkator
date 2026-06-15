package plan

import "time"

const (
	SchemaVersion       = "bach.plan.v1"
	LedgerSchemaVersion = "bach.plan_ledger.v1"
	StatusReady         = "ready"
	StatusPlanned       = "planned"
	StatusBlocked       = "blocked"
	StatusPending       = "pending"
	StatusInProgress    = "in_progress"
	StatusImplemented   = "implemented"
	StatusFailed        = "failed"
	StatusStale         = "stale"
	StatusInvalidLedger = "invalid_ledger"
)

type Document struct {
	Path            string
	ID              string
	Title           string
	Description     string
	DependsOn       []string
	AgentTemplate   string
	Policy          string
	RequiredTargets []string
	Labels          []string
	Metadata        map[string]string
	Hash            string
}

type Diagnostic struct {
	Severity string `json:"severity"`
	File     string `json:"file,omitempty"`
	Message  string `json:"message"`
	Code     string `json:"code"`
}

type Ledger struct {
	SchemaVersion string
	LedgerID      string
	PlanID        string
	Status        string
	Hash          string
	RunID         string
	Commit        string
	RecordedAt    time.Time
	Evidence      []Evidence
	ImplementedAt time.Time
}

type Evidence struct {
	ID       string
	Kind     string
	Hash     string
	Content  map[string]any
	Metadata map[string]string
}

type StatusRecord struct {
	Document    Document
	Status      string
	Diagnostics []Diagnostic
	Ledger      *LedgerSummary
}

type LedgerSummary struct {
	LedgerID   string
	Status     string
	Hash       string
	RecordedAt time.Time
}

type Selection struct {
	Documents   []Document
	Waves       [][]string
	Diagnostics []Diagnostic
}
