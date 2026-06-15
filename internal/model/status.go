package model

type RunStatus string

const (
	RunStatusSuccess         RunStatus = "success"
	RunStatusFailed          RunStatus = "failed"
	RunStatusRunning         RunStatus = "running"
	RunStatusPending         RunStatus = "pending"
	RunStatusCancelled       RunStatus = "cancelled"
	RunStatusSkipped         RunStatus = "skipped"
	RunStatusQualityFailed   RunStatus = "quality-failed"
	RunStatusPreflightFailed RunStatus = "preflight-failed"
	RunStatusUnknown         RunStatus = "unknown"
)

type Lifecycle string

const (
	LifecyclePending         Lifecycle = "pending"
	LifecycleClaimed         Lifecycle = "claimed"
	LifecycleRunning         Lifecycle = "running"
	LifecycleWaitingApproval Lifecycle = "waiting_approval"
	LifecycleCancelled       Lifecycle = "cancelled"
	LifecycleCompleted       Lifecycle = "completed"
	LifecycleFailed          Lifecycle = "failed"
)

type Priority string

const (
	PriorityCritical Priority = "critical"
	PriorityUrgent   Priority = "urgent"
	PriorityHigh     Priority = "high"
	PriorityNormal   Priority = "normal"
	PriorityLow      Priority = "low"
)
