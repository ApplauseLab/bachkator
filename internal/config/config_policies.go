package config

import (
	"fmt"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
)

func registerPolicies(
	project *Project,
	policies []*Policy,
	shells []*Target,
	agents []*Target,
	images []*Target,
	pipelines []*Target,
	groups []*Target,
) error {
	policyRefs := &hcl.EvalContext{
		Variables: targetRefEvalContext(shells, images, pipelines, groups, agents).Variables,
	}
	for _, policy := range policies {
		if err := resolvePolicyExtraAttributes(policy, policyRefs); err != nil {
			return err
		}
		key := "policy/" + policy.Name
		if policy.Subject != "" {
			key = policy.Address()
		}
		if _, exists := project.Policies[key]; exists {
			return fmt.Errorf("duplicate policy %q", policy.Name)
		}
		project.Policies[key] = policy
	}
	return nil
}

func resolvePolicyExtraAttributes(policy *Policy, refs *hcl.EvalContext) error {
	content, _, diags := policy.Remain.PartialContent(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{{Name: "reviewers"}, {Name: "required_targets"}},
		Blocks:     []hcl.BlockHeaderSchema{{Type: "quality_gate"}},
	})
	if diags.HasErrors() {
		return fmt.Errorf("policy %q: %s", policy.Name, diags.Error())
	}
	if attr, ok := content.Attributes["reviewers"]; ok {
		reviewers, err := decodeTargetRefList(attr, refs)
		if err != nil {
			return fmt.Errorf("policy %q reviewers: %w", policy.Name, err)
		}
		policy.Reviewers = reviewers
	}
	if attr, ok := content.Attributes["required_targets"]; ok {
		targets, err := decodeTargetRefList(attr, refs)
		if err != nil {
			return fmt.Errorf("policy %q required_targets: %w", policy.Name, err)
		}
		policy.RequiredTargets = targets
	}
	for _, block := range content.Blocks {
		var gate QualityGate
		if diags := gohcl.DecodeBody(block.Body, nil, &gate); diags.HasErrors() {
			return fmt.Errorf("policy %q quality_gate: %s", policy.Name, diags.Error())
		}
		policy.QualityGates = append(policy.QualityGates, &gate)
	}
	return nil
}

func validateSubjectPolicies(project *Project) error {
	for key, policy := range project.Policies {
		if policy.Subject == "" {
			continue
		}
		if len(policy.RequiredTargets) == 0 {
			return fmt.Errorf("policy %q required_targets must not be empty", policy.Name)
		}
		for i, target := range policy.RequiredTargets {
			canonical := strings.ReplaceAll(target, ".", "/")
			if _, exists := project.Targets[canonical]; !exists {
				return fmt.Errorf(
					"policy %q references unknown required target %q",
					policy.Name,
					target,
				)
			}
			policy.RequiredTargets[i] = canonical
		}
		if _, exists := project.Policies[key]; !exists {
			return fmt.Errorf("policy %q was not registered", policy.Name)
		}
	}
	return nil
}

func (p *Policy) Address() string {
	return "policy." + p.Name + "@" + p.Subject
}
