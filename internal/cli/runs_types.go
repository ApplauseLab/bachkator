package cli

import "github.com/applauselab/bachkator/internal/model"

type runInspection struct {
	RunID           string                    `json:"run_id"`
	RequestedTarget string                    `json:"requested_target"`
	Status          model.RunStatus           `json:"status"`
	StartedAt       string                    `json:"started_at"`
	FinishedAt      string                    `json:"finished_at,omitempty"`
	LogDir          string                    `json:"log_dir"`
	Targets         []targetRunInspection     `json:"targets,omitempty"`
	FailedTargets   []targetFailureInspection `json:"failed_targets"`
	SuggestedFixes  []string                  `json:"suggested_fixes"`
}

type targetRunInspection struct {
	Target            string                       `json:"target"`
	Status            model.RunStatus              `json:"status"`
	Operation         string                       `json:"operation,omitempty"`
	LogPath           string                       `json:"log_path,omitempty"`
	Artifacts         []string                     `json:"artifacts,omitempty"`
	Quality           targetQualityInspection      `json:"quality"`
	AgentReports      []agentReportInspection      `json:"agent_reports,omitempty"`
	PolicyEvaluations []policyEvaluationInspection `json:"policy_evaluations,omitempty"`
}

type agentReportInspection struct {
	Path               string         `json:"path"`
	Mode               string         `json:"mode,omitempty"`
	Status             string         `json:"status,omitempty"`
	ProviderName       string         `json:"provider_name,omitempty"`
	ProviderType       string         `json:"provider_type,omitempty"`
	ProviderCommand    []string       `json:"provider_command,omitempty"`
	Subject            map[string]any `json:"subject,omitempty"`
	PRURL              string         `json:"pr_url,omitempty"`
	TargetBranchCommit string         `json:"target_branch_commit,omitempty"`
	MergeCommit        string         `json:"merge_commit,omitempty"`
	Summary            string         `json:"summary,omitempty"`
}

type policyEvaluationInspection struct {
	Path    string `json:"path"`
	Schema  string `json:"schema,omitempty"`
	Target  string `json:"target,omitempty"`
	Verdict string `json:"verdict,omitempty"`
	RunID   string `json:"run_id,omitempty"`
}

type targetFailureInspection struct {
	Target            string                       `json:"target"`
	Status            model.RunStatus              `json:"status"`
	ExitCode          *int                         `json:"exit_code,omitempty"`
	Operation         string                       `json:"operation,omitempty"`
	LogPath           string                       `json:"log_path,omitempty"`
	Artifacts         []string                     `json:"artifacts,omitempty"`
	Quality           targetQualityInspection      `json:"quality"`
	PreflightFailures []preflightFailureInspection `json:"preflight_failures,omitempty"`
	MissingTools      []toolFailureInspection      `json:"missing_tools,omitempty"`
	LogExcerpt        []string                     `json:"log_excerpt,omitempty"`
}

type targetQualityInspection struct {
	Reports     []qualityReportInspection `json:"reports,omitempty"`
	FailedGates []qualityGateInspection   `json:"failed_gates,omitempty"`
}

type qualityReportInspection struct {
	Path     string          `json:"path"`
	Kind     string          `json:"kind"`
	Format   string          `json:"format,omitempty"`
	Status   model.RunStatus `json:"status"`
	Parsed   bool            `json:"parsed"`
	Metrics  int             `json:"metrics"`
	Findings int             `json:"findings"`
	Message  string          `json:"message,omitempty"`
}

type qualityGateInspection struct {
	Metric    string  `json:"metric"`
	Op        string  `json:"op"`
	Threshold float64 `json:"threshold"`
	Actual    float64 `json:"actual"`
	Message   string  `json:"message"`
}

type preflightFailureInspection struct {
	Name   string `json:"name"`
	Kind   string `json:"kind,omitempty"`
	Fix    string `json:"fix,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type toolFailureInspection struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Fix     string `json:"fix,omitempty"`
	Reason  string `json:"reason,omitempty"`
}
