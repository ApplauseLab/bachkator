package factorydaemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/applauselab/bachkator/internal/bacherr"
	"github.com/applauselab/bachkator/internal/backend"
	"github.com/applauselab/bachkator/internal/config"
	"github.com/applauselab/bachkator/internal/plan"
)

func (s Service) ensurePlanApproval(
	ctx context.Context,
	item backend.FactoryWorkItem,
	attemptID string,
	planPath string,
) error {
	hash, err := s.planHash(planPath)
	if err != nil {
		return s.failItem(ctx, item, config.FactoryPhasePlan, err)
	}
	approval, ok, err := s.findApproval(ctx, item.ID, attemptID, config.FactoryPhasePlan)
	if err != nil {
		return s.failItem(ctx, item, config.FactoryPhasePlan, err)
	}
	if ok {
		if approval.PlanPath != planPath || approval.PlanHash != hash {
			return s.failItem(
				ctx,
				item,
				config.FactoryPhasePlan,
				fmt.Errorf("approved plan hash does not match current plan; replan required"),
			)
		}
		return nil
	}
	return s.waitForApproval(ctx, item, attemptID, config.FactoryPhasePlan, planPath, hash)
}

func (s Service) ensureDeployApproval(
	ctx context.Context,
	item backend.FactoryWorkItem,
	attemptID string,
	phaseKey string,
) error {
	_, ok, err := s.findApproval(ctx, item.ID, attemptID, phaseKey)
	if err != nil {
		return s.failItem(ctx, item, phaseKey, err)
	}
	if ok {
		return nil
	}
	return s.waitForApproval(ctx, item, attemptID, phaseKey, "", "")
}

func (s Service) waitForApproval(
	ctx context.Context,
	item backend.FactoryWorkItem,
	attemptID string,
	phaseKey string,
	planPath string,
	planHash string,
) error {
	evidence := map[string]string{}
	if planPath != "" {
		evidence["plan_path"] = planPath
	}
	if planHash != "" {
		evidence["plan_hash"] = planHash
	}
	if err := s.writePhase(
		ctx,
		item,
		attemptID,
		phaseKey,
		PhaseWaitingApproval,
		"",
		"",
		planPath,
		"",
	); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		s.stdout(),
		"work item %s waiting for approval phase=%s\n",
		item.ID,
		phaseKey,
	); err != nil {
		return err
	}
	return bacherr.ErrWaitingApproval
}

func (s Service) findApproval(
	ctx context.Context,
	workItemID string,
	attemptID string,
	phase string,
) (backend.FactoryApproval, bool, error) {
	approvals, err := s.Backend.Factory.ListApprovals(ctx, workItemID)
	if err != nil {
		return backend.FactoryApproval{}, false, err
	}
	for _, approval := range approvals {
		if approval.AttemptID == attemptID && approval.Phase == phase {
			return approval, true, nil
		}
	}
	return backend.FactoryApproval{}, false, nil
}

func (s Service) planHash(planPath string) (string, error) {
	path := filepath.Join(s.ConfigProject.Root, filepath.FromSlash(planPath))
	if err := ensureContained(s.ConfigProject.Root, path); err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	doc, diagnostics := plan.Parse(planPath, data)
	if len(diagnostics) > 0 {
		return "", fmt.Errorf("plan %q has %d diagnostics", planPath, len(diagnostics))
	}
	return doc.Hash, nil
}
