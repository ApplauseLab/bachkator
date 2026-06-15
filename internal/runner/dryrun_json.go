package runner

import (
	"context"
	"encoding/json"
	"io"

	"github.com/applauselab/bachkator/internal/model"
)

type dryRunPlanJSON struct {
	Target           string                    `json:"target"`
	RequestedTargets []string                  `json:"requested_targets"`
	SelectedProfiles []string                  `json:"selected_profiles"`
	TargetOrder      []string                  `json:"target_order"`
	DependencyEdges  []dryRunPlanEdgeJSON      `json:"dependency_edges"`
	PipelineEdges    []dryRunPlanEdgeJSON      `json:"pipeline_edges,omitempty"`
	GroupEdges       []dryRunPlanEdgeJSON      `json:"group_edges,omitempty"`
	CompositeEdges   []dryRunPlanTypedEdgeJSON `json:"composite_edges,omitempty"`
	EffectiveRisk    dryRunPlanRiskJSON        `json:"effective_risk"`
	Targets          []dryRunPlanTargetJSON    `json:"targets"`
}

type dryRunPlanEdgeJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type dryRunPlanTypedEdgeJSON struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type dryRunPlanRiskJSON struct {
	Remote               bool     `json:"remote"`
	Destructive          bool     `json:"destructive"`
	RequiresConfirmation bool     `json:"requires_confirmation"`
	Labels               []string `json:"labels"`
}

type dryRunPlanTargetJSON struct {
	Name       string                  `json:"name"`
	Type       model.TargetType        `json:"type"`
	Operation  string                  `json:"operation,omitempty"`
	WorkDir    string                  `json:"workdir,omitempty"`
	DependsOn  []string                `json:"depends_on,omitempty"`
	Steps      []string                `json:"steps,omitempty"`
	Targets    []string                `json:"targets,omitempty"`
	Lock       string                  `json:"lock,omitempty"`
	Risks      dryRunPlanRiskJSON      `json:"risks"`
	Cache      dryRunPlanCacheJSON     `json:"cache"`
	Inputs     []string                `json:"inputs,omitempty"`
	Outputs    []string                `json:"outputs,omitempty"`
	Env        []string                `json:"env,omitempty"`
	Tools      []model.ToolRequirement `json:"tools,omitempty"`
	Preflights []model.PreflightCheck  `json:"preflights,omitempty"`
}

type dryRunPlanCacheJSON struct {
	Cacheable   bool     `json:"cacheable"`
	Expectation string   `json:"expectation"`
	Fresh       bool     `json:"fresh"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Reasons     []string `json:"reasons,omitempty"`
}

func (r *Runner) writeDryRunPlanJSON(
	stdout io.Writer,
	force bool,
	project *Project,
	state *State,
	plan *Plan,
	gitContext GitContext,
	dotenv map[string]string,
) error {
	fingerprints := map[string]string{}
	planTargets := make([]dryRunPlanTargetJSON, 0, len(plan.Order))
	for _, targetName := range plan.Order {
		target := plan.Target(targetName)
		planTarget, err := r.dryRunPlanTarget(
			force,
			project,
			state,
			plan,
			target,
			gitContext,
			dotenv,
			fingerprints,
		)
		if err != nil {
			return err
		}
		fingerprints[targetName] = planTarget.Cache.Fingerprint
		planTargets = append(planTargets, planTarget)
	}
	selectedProfiles := append([]string(nil), project.SelectedProfiles...)
	if selectedProfiles == nil {
		selectedProfiles = []string{}
	}
	jsonPlan := dryRunPlanJSON{
		Target:           plan.TargetName,
		RequestedTargets: append([]string(nil), plan.RequestedTargets...),
		SelectedProfiles: selectedProfiles,
		TargetOrder:      append([]string(nil), plan.Order...),
		DependencyEdges:  dryRunEdges(plan.DependencyEdges),
		PipelineEdges:    dryRunEdges(plan.PipelineEdges),
		GroupEdges:       dryRunEdges(plan.GroupEdges),
		CompositeEdges:   dryRunTypedEdges(plan.CompositeEdges),
		EffectiveRisk:    dryRunRisk(plan.EffectiveRisk),
		Targets:          planTargets,
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonPlan)
}

func (r *Runner) dryRunPlanTarget(
	force bool,
	project *Project,
	state *State,
	plan *Plan,
	target *Target,
	gitContext GitContext,
	dotenv map[string]string,
	fingerprints map[string]string,
) (dryRunPlanTargetJSON, error) {
	spec := target.Spec()
	runtimeEnv := commandEnv(
		gitContext,
		dotenv,
		projectRuntimeEnv(project),
		[]string{
			"BACH_RUN_DIRECTORY=.bach/runs/dry-run/" + target.Name,
			"RUN_DIRECTORY=.bach/runs/dry-run/" + target.Name,
		},
		target.Env,
	)
	description, err := targetOperation(
		context.Background(),
		r.targetHandlers(),
		target,
		runtimeEnv,
	)
	if err != nil {
		return dryRunPlanTargetJSON{}, err
	}
	fingerprint, fingerprintParts, err := targetFingerprintParts(
		r.targetHandlers(),
		project,
		target,
		dotenv,
		plan.DependencyFingerprints(target.Name, fingerprints),
	)
	if err != nil {
		return dryRunPlanTargetJSON{}, err
	}
	cacheable := targetCacheable(target)
	fresh := false
	expectation := "not_cacheable"
	reasons := []string(nil)
	if cacheable {
		record := state.Targets[target.Name]
		fresh = !force && targetFresh(target, project.Root, record, fingerprint)
		expectation = "run"
		if fresh {
			expectation = "cached"
		} else {
			reasons = targetStaleReasons(
				target,
				project.Root,
				record,
				fingerprint,
				fingerprintParts,
				force,
			)
		}
	}
	shell, _ := spec.Body.(model.ShellSpec)
	pipeline, _ := spec.Body.(model.PipelineSpec)
	group, _ := spec.Body.(model.GroupSpec)
	return dryRunPlanTargetJSON{
		Name:      target.Name,
		Type:      spec.TargetType(),
		Operation: description.Operation,
		WorkDir:   shell.WorkDir,
		DependsOn: append([]string(nil), target.DependsOn...),
		Steps:     append([]string(nil), pipeline.Steps...),
		Targets:   append([]string(nil), group.Targets...),
		Lock:      spec.Runtime.Lock,
		Risks: dryRunRisk(
			PlannedRisk{
				Remote:               spec.Metadata.Remote,
				Destructive:          spec.Metadata.Destructive,
				RequiresConfirmation: spec.Metadata.RequiresConfirmation,
			},
		),
		Cache: dryRunPlanCacheJSON{
			Cacheable:   cacheable,
			Expectation: expectation,
			Fresh:       fresh,
			Fingerprint: fingerprint,
			Reasons:     reasons,
		},
		Inputs:     append([]string(nil), spec.Cache.Inputs...),
		Outputs:    append([]string(nil), spec.Cache.Outputs...),
		Env:        append([]string(nil), spec.Runtime.Env...),
		Tools:      append([]model.ToolRequirement(nil), spec.Runtime.Tools...),
		Preflights: append([]model.PreflightCheck(nil), spec.Runtime.Preflights...),
	}, nil
}

func dryRunEdges(edges []PlanEdge) []dryRunPlanEdgeJSON {
	out := make([]dryRunPlanEdgeJSON, 0, len(edges))
	for _, edge := range edges {
		out = append(out, dryRunPlanEdgeJSON(edge))
	}
	return out
}

func dryRunTypedEdges(edges []PlanTypedEdge) []dryRunPlanTypedEdgeJSON {
	out := make([]dryRunPlanTypedEdgeJSON, 0, len(edges))
	for _, edge := range edges {
		out = append(out, dryRunPlanTypedEdgeJSON(edge))
	}
	return out
}

func dryRunRisk(risk PlannedRisk) dryRunPlanRiskJSON {
	return dryRunPlanRiskJSON{
		Remote:               risk.Remote,
		Destructive:          risk.Destructive,
		RequiresConfirmation: risk.RequiresConfirmation,
		Labels:               risk.Labels(),
	}
}
