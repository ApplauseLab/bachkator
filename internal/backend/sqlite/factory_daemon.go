package sqlite

import (
	"time"

	"github.com/applauselab/bachkator/internal/state"
	"github.com/applauselab/bachkator/pkg/backendprotocol"
)

func factoryDaemonLeaseFromProtocol(
	lease backendprotocol.FactoryDaemonLease,
) (state.FactoryDaemonLease, error) {
	acquiredAt, err := parseOptionalTime(lease.AcquiredAt, "acquired_at")
	if err != nil {
		return state.FactoryDaemonLease{}, err
	}
	renewedAt, err := parseOptionalTime(lease.RenewedAt, "renewed_at")
	if err != nil {
		return state.FactoryDaemonLease{}, err
	}
	expiresAt, err := parseOptionalTime(lease.ExpiresAt, "expires_at")
	if err != nil {
		return state.FactoryDaemonLease{}, err
	}
	releasedAt, err := parseOptionalTime(lease.ReleasedAt, "released_at")
	if err != nil {
		return state.FactoryDaemonLease{}, err
	}
	return state.FactoryDaemonLease{
		DaemonID:   lease.DaemonID,
		Factory:    lease.Factory,
		Hostname:   lease.Hostname,
		PID:        lease.PID,
		AcquiredAt: acquiredAt,
		RenewedAt:  renewedAt,
		ExpiresAt:  expiresAt,
		ReleasedAt: releasedAt,
		Status:     lease.Status,
	}, nil
}

func factoryDaemonLeaseToProtocol(
	lease state.FactoryDaemonLease,
) backendprotocol.FactoryDaemonLease {
	result := backendprotocol.FactoryDaemonLease{
		DaemonID: lease.DaemonID,
		Factory:  lease.Factory,
		Hostname: lease.Hostname,
		PID:      lease.PID,
		Status:   lease.Status,
	}
	setFactoryTime(&result.AcquiredAt, lease.AcquiredAt)
	setFactoryTime(&result.RenewedAt, lease.RenewedAt)
	setFactoryTime(&result.ExpiresAt, lease.ExpiresAt)
	setFactoryTime(&result.ReleasedAt, lease.ReleasedAt)
	return result
}

func factoryWorkItemPhaseFromProtocol(
	phase backendprotocol.FactoryWorkItemPhase,
) (state.FactoryWorkItemPhase, error) {
	startedAt, err := parseOptionalTime(phase.StartedAt, "started_at")
	if err != nil {
		return state.FactoryWorkItemPhase{}, err
	}
	finishedAt, err := parseOptionalTime(phase.FinishedAt, "finished_at")
	if err != nil {
		return state.FactoryWorkItemPhase{}, err
	}
	updatedAt, err := parseOptionalTime(phase.UpdatedAt, "updated_at")
	if err != nil {
		return state.FactoryWorkItemPhase{}, err
	}
	return state.FactoryWorkItemPhase{
		WorkItemID: phase.WorkItemID,
		AttemptID:  phase.AttemptID,
		PhaseKey:   phase.PhaseKey,
		Status:     phase.Status,
		Target:     phase.Target,
		RunID:      phase.RunID,
		PlanPath:   phase.PlanPath,
		LedgerID:   phase.LedgerID,
		Evidence:   cloneStringMap(phase.Evidence),
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		UpdatedAt:  updatedAt,
	}, nil
}

func setFactoryTime(out *string, value time.Time) {
	if !value.IsZero() {
		*out = value.UTC().Format(time.RFC3339Nano)
	}
}
