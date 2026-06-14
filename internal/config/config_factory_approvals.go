package config

import (
	"fmt"
	"time"
)

func validateApprovalProviders(factoryName string, providers []*FactoryApprovalProvider) error {
	names := map[string]struct{}{}
	for _, p := range providers {
		if p == nil {
			return fmt.Errorf("factory %q approval provider block is nil", factoryName)
		}
		if !factoryIdentifierPattern.MatchString(p.Name) {
			return fmt.Errorf(
				"factory %q approval provider %q name must be a simple identifier",
				factoryName,
				p.Name,
			)
		}
		if _, exists := names[p.Name]; exists {
			return fmt.Errorf(
				"factory %q has duplicate approval provider %q",
				factoryName,
				p.Name,
			)
		}
		names[p.Name] = struct{}{}
		if len(p.Command) == 0 {
			return fmt.Errorf(
				"factory %q approval provider %q command is required",
				factoryName,
				p.Name,
			)
		}
		if p.PollInterval != "" {
			if _, err := time.ParseDuration(p.PollInterval); err != nil {
				return fmt.Errorf(
					"factory %q approval provider %q poll_interval is not a valid duration: %w",
					factoryName,
					p.Name,
					ErrTriggerPollInterval,
				)
			}
		}
	}
	return nil
}

func (f *Factory) ApprovalProviders() []*FactoryApprovalProvider {
	if f == nil || len(f.Approvals) == 0 || f.Approvals[0] == nil {
		return nil
	}
	return f.Approvals[0].Provider
}

func (p *FactoryApprovalProvider) PollIntervalDuration() time.Duration {
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
