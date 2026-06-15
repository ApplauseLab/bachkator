package backendprotocol

import "github.com/applauselab/bachkator/internal/model"

const ProtocolVersion = "bach.backend.v1"

type Capability string

const (
	CapabilityRuns           Capability = "runs"
	CapabilityEvidenceRefs   Capability = "evidence_refs"
	CapabilityQualityReports Capability = "quality_reports"
	CapabilityFindings       Capability = "findings"
	CapabilityFactoryQueue   Capability = "factory_queue"
	CapabilityPlanLedger     Capability = "plan_ledger"
	CapabilityApprovals      Capability = "approvals"
)

type InitializeParams struct {
	Protocol    string            `json:"protocol"`
	ProjectName string            `json:"project_name"`
	ProjectRoot string            `json:"project_root"`
	Config      map[string]string `json:"config"`
}

type InitializeResult struct {
	Protocol     string            `json:"protocol"`
	Provider     string            `json:"provider"`
	Version      string            `json:"version"`
	Capabilities []Capability      `json:"capabilities"`
	Limits       map[string]string `json:"limits,omitempty"`
}

type RunRecord struct {
	SchemaVersion string            `json:"schema_version"`
	ID            string            `json:"id"`
	Target        string            `json:"target"`
	Status        model.RunStatus   `json:"status"`
	StartedAt     string            `json:"started_at"`
	FinishedAt    string            `json:"finished_at,omitempty"`
	LogDir        string            `json:"log_dir,omitempty"`
	DryRun        bool              `json:"dry_run"`
	Force         bool              `json:"force"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type TargetStateRecord struct {
	Fingerprint      string            `json:"fingerprint"`
	FingerprintParts map[string]string `json:"fingerprint_parts,omitempty"`
	CompletedAt      string            `json:"completed_at"`
}

type RunFinishParams struct {
	Run        RunRecord                    `json:"run"`
	Targets    map[string]TargetStateRecord `json:"targets,omitempty"`
	TargetRuns map[string]TargetRunRecord   `json:"target_runs,omitempty"`
	Evidence   []EvidenceRef                `json:"evidence,omitempty"`
}

type RunQuery struct {
	ID     string          `json:"id,omitempty"`
	Target string          `json:"target,omitempty"`
	Status model.RunStatus `json:"status,omitempty"`
	Since  string          `json:"since,omitempty"`
	Limit  int             `json:"limit,omitempty"`
}

type RunListResult struct {
	Runs []RunRecord `json:"runs"`
}

type RunResult struct {
	Run RunRecord `json:"run"`
}

type TargetRunRecord struct {
	Target     string            `json:"target"`
	Status     model.RunStatus   `json:"status"`
	StartedAt  string            `json:"started_at"`
	FinishedAt string            `json:"finished_at,omitempty"`
	LogPath    string            `json:"log_path,omitempty"`
	Operation  string            `json:"operation,omitempty"`
	ExitCode   *int              `json:"exit_code,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type TargetRunWrite struct {
	RunID string `json:"run_id"`
	TargetRunRecord
}

type EvidenceRef struct {
	SchemaVersion string            `json:"schema_version"`
	ID            string            `json:"id"`
	Kind          string            `json:"kind"`
	URI           string            `json:"uri"`
	Hash          string            `json:"hash,omitempty"`
	RunID         string            `json:"run_id,omitempty"`
	Target        string            `json:"target,omitempty"`
	CreatedAt     string            `json:"created_at,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type EvidenceRefListResult struct {
	Evidence []EvidenceRef `json:"evidence"`
}

type FactoryWorkItem struct {
	SchemaVersion      string                   `json:"schema_version"`
	ID                 string                   `json:"id"`
	Factory            string                   `json:"factory"`
	Workflow           string                   `json:"workflow"`
	Lifecycle          model.Lifecycle          `json:"lifecycle"`
	CurrentPhase       string                   `json:"current_phase"`
	Title              string                   `json:"title"`
	Body               string                   `json:"body,omitempty"`
	BodyHash           string                   `json:"body_hash,omitempty"`
	Priority           model.Priority           `json:"priority"`
	Labels             []string                 `json:"labels,omitempty"`
	SourceType         string                   `json:"source_type"`
	DedupeKey          string                   `json:"dedupe_key,omitempty"`
	SubmittedPlanPath  string                   `json:"submitted_plan_path,omitempty"`
	SubmittedPlanHash  string                   `json:"submitted_plan_hash,omitempty"`
	IntakeEvidenceID   string                   `json:"intake_evidence_id,omitempty"`
	IntakeEvidenceURI  string                   `json:"intake_evidence_uri,omitempty"`
	IntakeEvidenceHash string                   `json:"intake_evidence_hash,omitempty"`
	Metadata           map[string]string        `json:"metadata,omitempty"`
	CreatedAt          string                   `json:"created_at"`
	UpdatedAt          string                   `json:"updated_at"`
	CancelledAt        string                   `json:"cancelled_at,omitempty"`
	CancelReason       string                   `json:"cancel_reason,omitempty"`
	ClaimedByDaemonID  string                   `json:"claimed_by_daemon_id,omitempty"`
	ClaimedAt          string                   `json:"claimed_at,omitempty"`
	ClaimExpiresAt     string                   `json:"claim_expires_at,omitempty"`
	CompletedAt        string                   `json:"completed_at,omitempty"`
	FailedAt           string                   `json:"failed_at,omitempty"`
	FailurePhase       string                   `json:"failure_phase,omitempty"`
	FailureMessage     string                   `json:"failure_message,omitempty"`
	Attempts           []FactoryWorkItemAttempt `json:"attempts,omitempty"`
	Events             []FactoryWorkItemEvent   `json:"events,omitempty"`
}

type FactoryWorkItemAttempt struct {
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

type FactoryWorkItemEvent struct {
	ID         string            `json:"id"`
	WorkItemID string            `json:"work_item_id"`
	AttemptID  string            `json:"attempt_id,omitempty"`
	Type       string            `json:"type"`
	Message    string            `json:"message,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  string            `json:"created_at"`
}

type FactoryEnqueueWorkItemParams struct {
	Item        FactoryWorkItem        `json:"item"`
	Attempt     FactoryWorkItemAttempt `json:"attempt"`
	Event       FactoryWorkItemEvent   `json:"event"`
	DedupeEvent FactoryWorkItemEvent   `json:"dedupe_event,omitempty"`
}

type FactoryWorkItemResult struct {
	Item    FactoryWorkItem `json:"item"`
	Created bool            `json:"created,omitempty"`
}

type FactoryWorkItemQuery struct {
	Factory string          `json:"factory"`
	ID      string          `json:"id,omitempty"`
	Status  model.Lifecycle `json:"status,omitempty"`
}

type FactoryWorkItemListResult struct {
	Items []FactoryWorkItem `json:"items"`
}

type FactoryCancelWorkItemParams struct {
	Factory     string               `json:"factory"`
	ID          string               `json:"id"`
	Reason      string               `json:"reason"`
	CancelledAt string               `json:"cancelled_at"`
	Event       FactoryWorkItemEvent `json:"event"`
}

type FactoryDaemonLease struct {
	DaemonID   string `json:"daemon_id"`
	Factory    string `json:"factory"`
	Hostname   string `json:"hostname,omitempty"`
	PID        int    `json:"pid,omitempty"`
	AcquiredAt string `json:"acquired_at"`
	RenewedAt  string `json:"renewed_at"`
	ExpiresAt  string `json:"expires_at"`
	ReleasedAt string `json:"released_at,omitempty"`
	Status     string `json:"status"`
}

type FactoryDaemonLeaseResult struct {
	Lease FactoryDaemonLease `json:"lease"`
}

type FactoryAcquireDaemonLeaseParams struct {
	Lease FactoryDaemonLease `json:"lease"`
}

type FactoryRenewDaemonLeaseParams struct {
	DaemonID  string `json:"daemon_id"`
	RenewedAt string `json:"renewed_at"`
	ExpiresAt string `json:"expires_at"`
}

type FactoryReleaseDaemonLeaseParams struct {
	DaemonID   string `json:"daemon_id"`
	ReleasedAt string `json:"released_at"`
}

type FactoryClaimWorkItemParams struct {
	Factory   string `json:"factory"`
	DaemonID  string `json:"daemon_id"`
	ClaimedAt string `json:"claimed_at"`
	ExpiresAt string `json:"expires_at"`
}

type FactoryWorkItemPhase struct {
	WorkItemID string            `json:"work_item_id"`
	AttemptID  string            `json:"attempt_id"`
	PhaseKey   string            `json:"phase_key"`
	Status     string            `json:"status"`
	Target     string            `json:"target,omitempty"`
	RunID      string            `json:"run_id,omitempty"`
	PlanPath   string            `json:"plan_path,omitempty"`
	LedgerID   string            `json:"ledger_id,omitempty"`
	Evidence   map[string]string `json:"evidence,omitempty"`
	StartedAt  string            `json:"started_at,omitempty"`
	FinishedAt string            `json:"finished_at,omitempty"`
	UpdatedAt  string            `json:"updated_at"`
}

type FactoryUpdateWorkItemPhaseParams struct {
	Phase FactoryWorkItemPhase `json:"phase"`
}

type FactoryFinishWorkItemParams struct {
	Factory        string `json:"factory"`
	ID             string `json:"id"`
	FinishedAt     string `json:"finished_at"`
	FailurePhase   string `json:"failure_phase,omitempty"`
	FailureMessage string `json:"failure_message,omitempty"`
}

type FactoryDaemonStatusResult struct {
	Lease           FactoryDaemonLease      `json:"lease,omitempty"`
	ActiveItem      FactoryWorkItem         `json:"active_item,omitempty"`
	HasActiveItem   bool                    `json:"has_active_item"`
	LifecycleCounts map[model.Lifecycle]int `json:"lifecycle_counts"`
}

type FactoryDaemonStatusQuery struct {
	Factory string `json:"factory"`
	Now     string `json:"now"`
}

type FactoryApproval struct {
	ID             string            `json:"approval_id"`
	Factory        string            `json:"factory"`
	Workflow       string            `json:"workflow"`
	WorkItemID     string            `json:"work_item_id"`
	AttemptID      string            `json:"attempt_id"`
	Phase          string            `json:"phase"`
	PlanPath       string            `json:"plan_path,omitempty"`
	PlanHash       string            `json:"plan_hash,omitempty"`
	ApprovedAt     string            `json:"approved_at"`
	Approver       string            `json:"approver,omitempty"`
	ApproverSource string            `json:"approver_source,omitempty"`
	Reason         string            `json:"reason,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type FactoryRecordApprovalParams struct {
	Approval FactoryApproval      `json:"approval"`
	Event    FactoryWorkItemEvent `json:"event"`
}

type FactoryRecordApprovalResult struct {
	Approval FactoryApproval `json:"approval"`
	Existing bool            `json:"existing"`
}

type FactoryListApprovalsParams struct {
	WorkItemID string `json:"work_item_id"`
}

type FactoryListApprovalsResult struct {
	Approvals []FactoryApproval `json:"approvals"`
}

type PlanLedger struct {
	SchemaVersion string         `json:"schema_version"`
	LedgerID      string         `json:"ledger_id"`
	PlanID        string         `json:"plan_id"`
	Status        string         `json:"status"`
	Hash          string         `json:"hash"`
	RunID         string         `json:"run_id,omitempty"`
	Commit        string         `json:"commit,omitempty"`
	RecordedAt    string         `json:"recorded_at"`
	Evidence      []PlanEvidence `json:"evidence,omitempty"`
	ImplementedAt string         `json:"implemented_at,omitempty"`
}

type PlanEvidence struct {
	ID       string            `json:"id"`
	Kind     string            `json:"kind"`
	Hash     string            `json:"hash,omitempty"`
	Content  map[string]any    `json:"content,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type PlanLedgerQuery struct {
	PlanID string `json:"plan_id"`
}

type PlanLedgerResult struct {
	Ledger PlanLedger `json:"ledger"`
}

type QualityMetric struct {
	Name  string  `json:"name"`
	Scope string  `json:"scope,omitempty"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit,omitempty"`
}

type QualityFinding struct {
	Kind       string  `json:"kind,omitempty"`
	File       string  `json:"file,omitempty"`
	Line       int     `json:"line,omitempty"`
	Severity   string  `json:"severity,omitempty"`
	Rule       string  `json:"rule,omitempty"`
	Message    string  `json:"message"`
	DurationMS float64 `json:"duration_ms,omitempty"`
}

type QualityReport struct {
	RunID     string           `json:"run_id"`
	Target    string           `json:"target,omitempty"`
	Kind      string           `json:"kind"`
	Format    string           `json:"format,omitempty"`
	Path      string           `json:"path,omitempty"`
	Status    model.RunStatus  `json:"status"`
	Message   string           `json:"message,omitempty"`
	CreatedAt string           `json:"created_at"`
	Metrics   []QualityMetric  `json:"metrics,omitempty"`
	Findings  []QualityFinding `json:"findings,omitempty"`
}

type QualityGateResult struct {
	RunID     string  `json:"run_id"`
	Target    string  `json:"target,omitempty"`
	Metric    string  `json:"metric"`
	Op        string  `json:"op"`
	Threshold float64 `json:"threshold"`
	Actual    float64 `json:"actual"`
	Status    string  `json:"status"`
	Message   string  `json:"message,omitempty"`
	CreatedAt string  `json:"created_at"`
}

type QualityReportBatch struct {
	Reports []QualityReport     `json:"reports,omitempty"`
	Gates   []QualityGateResult `json:"gates,omitempty"`
}

type FindingQuery struct {
	Fingerprint string `json:"fingerprint,omitempty"`
	Status      string `json:"status,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type FindingListResult struct {
	Findings []FindingObservation `json:"findings"`
}

type FindingSeverity string

const (
	FindingInfo     FindingSeverity = "info"
	FindingWarning  FindingSeverity = "warning"
	FindingError    FindingSeverity = "error"
	FindingCritical FindingSeverity = "critical"
)

type FindingLocation struct {
	Path        string `json:"path"`
	StartLine   int    `json:"start_line,omitempty"`
	StartColumn int    `json:"start_column,omitempty"`
	EndLine     int    `json:"end_line,omitempty"`
	EndColumn   int    `json:"end_column,omitempty"`
}

type FindingObservation struct {
	SchemaVersion        string            `json:"schema_version"`
	ID                   string            `json:"id"`
	SourceType           string            `json:"source_type"`
	SourceID             string            `json:"source_id"`
	Severity             FindingSeverity   `json:"severity"`
	Category             string            `json:"category"`
	Message              string            `json:"message"`
	Location             *FindingLocation  `json:"location,omitempty"`
	SuggestedFingerprint string            `json:"suggested_fingerprint,omitempty"`
	Fingerprint          string            `json:"fingerprint"`
	ObservedAt           string            `json:"observed_at"`
	Status               string            `json:"status,omitempty"`
	Metadata             map[string]string `json:"metadata,omitempty"`
}

type FactoryTriggerCursor struct {
	Factory    string            `json:"factory"`
	Trigger    string            `json:"trigger"`
	Cursor     string            `json:"cursor,omitempty"`
	LastPollAt string            `json:"last_poll_at,omitempty"`
	LastAckAt  string            `json:"last_ack_at,omitempty"`
	LastNackAt string            `json:"last_nack_at,omitempty"`
	LastError  string            `json:"last_error,omitempty"`
	UpdatedAt  string            `json:"updated_at"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type FactoryGetTriggerCursorParams struct {
	Factory string `json:"factory"`
	Trigger string `json:"trigger"`
}

type FactoryGetTriggerCursorResult struct {
	Cursor FactoryTriggerCursor `json:"cursor"`
}

type FactoryRecordTriggerCursorParams struct {
	Cursor FactoryTriggerCursor `json:"cursor"`
}

type FactoryRecordTriggerCursorResult struct {
	Cursor FactoryTriggerCursor `json:"cursor"`
}
