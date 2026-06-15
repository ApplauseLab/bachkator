package config

import (
	"fmt"
	"time"
)

const (
	minPollInterval     = 1 * time.Second
	defaultPollInterval = 5 * time.Minute
)

var ErrTriggerPollInterval = fmt.Errorf("invalid poll_interval")

func validateProviderTriggers(
	factoryName string,
	workflows []*FactoryWorkflow,
	providers []*FactoryProviderTrigger,
) error {
	names := map[string]struct{}{}
	workflowSet := map[string]struct{}{}
	for _, w := range workflows {
		if w != nil {
			workflowSet[w.Name] = struct{}{}
		}
	}
	multiWorkflow := len(workflowSet) > 1

	for _, p := range providers {
		if p == nil {
			return fmt.Errorf("factory %q provider trigger block is nil", factoryName)
		}
		if !factoryIdentifierPattern.MatchString(p.Name) {
			return fmt.Errorf(
				"factory %q provider trigger %q name must be a simple identifier",
				factoryName,
				p.Name,
			)
		}
		if _, exists := names[p.Name]; exists {
			return fmt.Errorf(
				"factory %q has duplicate provider trigger %q",
				factoryName,
				p.Name,
			)
		}
		names[p.Name] = struct{}{}
		if len(p.Command) == 0 {
			return fmt.Errorf(
				"factory %q provider trigger %q command is required",
				factoryName,
				p.Name,
			)
		}
		if p.PollInterval != "" {
			if _, err := time.ParseDuration(p.PollInterval); err != nil {
				return fmt.Errorf(
					"factory %q provider trigger %q poll_interval is not a valid duration: %w",
					factoryName,
					p.Name,
					ErrTriggerPollInterval,
				)
			}
		}
		if multiWorkflow && len(p.Route) == 0 {
			return fmt.Errorf(
				"factory %q provider trigger %q requires route rules because the factory has multiple workflows",
				factoryName,
				p.Name,
			)
		}
		routeLabels := map[string]struct{}{}
		for _, r := range p.Route {
			if r == nil {
				return fmt.Errorf(
					"factory %q provider trigger %q route block is nil",
					factoryName,
					p.Name,
				)
			}
			if r.Label == "" {
				return fmt.Errorf(
					"factory %q provider trigger %q route.label is required",
					factoryName,
					p.Name,
				)
			}
			if _, exists := routeLabels[r.Label]; exists {
				return fmt.Errorf(
					"factory %q provider trigger %q has duplicate route label %q",
					factoryName,
					p.Name,
					r.Label,
				)
			}
			routeLabels[r.Label] = struct{}{}
			if r.Workflow == "" {
				return fmt.Errorf(
					"factory %q provider trigger %q route.workflow is required",
					factoryName,
					p.Name,
				)
			}
			if _, ok := workflowSet[r.Workflow]; !ok {
				return fmt.Errorf(
					"factory %q provider trigger %q route.workflow %q is not a declared workflow",
					factoryName,
					p.Name,
					r.Workflow,
				)
			}
		}
	}
	return nil
}

func (f *Factory) ProviderTriggers() []*FactoryProviderTrigger {
	if f == nil || len(f.Triggers) == 0 || f.Triggers[0] == nil {
		return nil
	}
	return f.Triggers[0].Provider
}

func (p *FactoryProviderTrigger) PollIntervalDuration() time.Duration {
	if p == nil || p.PollInterval == "" {
		return defaultPollInterval
	}
	d, err := time.ParseDuration(p.PollInterval)
	if err != nil {
		return defaultPollInterval
	}
	if d < minPollInterval {
		return minPollInterval
	}
	return d
}

func (p *FactoryProviderTrigger) RouteWorkflow(
	labels []string,
	defaultWorkflow string,
) (string, error) {
	if p == nil {
		return "", fmt.Errorf("provider trigger is nil")
	}
	if len(p.Route) == 0 {
		if defaultWorkflow == "" {
			return "", fmt.Errorf(
				"provider trigger %q has no route rules and no default workflow",
				p.Name,
			)
		}
		return defaultWorkflow, nil
	}
	labelSet := map[string]struct{}{}
	for _, l := range labels {
		labelSet[l] = struct{}{}
	}
	var matched string
	for _, r := range p.Route {
		if r == nil {
			continue
		}
		if _, ok := labelSet[r.Label]; !ok {
			continue
		}
		if matched != "" {
			return "", fmt.Errorf(
				"provider trigger %q item matches multiple routes",
				p.Name,
			)
		}
		matched = r.Workflow
	}
	if matched == "" {
		return "", fmt.Errorf(
			"provider trigger %q item did not match any route",
			p.Name,
		)
	}
	return matched, nil
}
