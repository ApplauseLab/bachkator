package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/applauselab/bachkator/internal/model"
)

func formatListHeader(stdout io.Writer, nameWidth int) error {
	_, err := fmt.Fprintf(
		stdout,
		"%-*s %-7s %-42s %s\n",
		nameWidth,
		"TARGET",
		"COST",
		"RISKS",
		"DESCRIPTION",
	)
	return err
}

func formatPolicyListEntry(
	stdout io.Writer,
	nameWidth int,
	name string,
	policy *Policy,
	verbose bool,
) error {
	description := fmt.Sprintf(
		"policy fan-out for %s requiring %s",
		policy.Subject,
		strings.Join(policy.RequiredTargets, ", "),
	)
	if verbose {
		_, err := fmt.Fprintf(
			stdout,
			"%-*s %-7s %-42s %s\n",
			nameWidth,
			name,
			"-",
			"-",
			description,
		)
		return err
	}
	_, err := fmt.Fprintf(stdout, "%-24s %s\n", name, description)
	return err
}

func formatTargetListEntry(
	stdout io.Writer,
	nameWidth int,
	name string,
	target *Target,
	verbose bool,
) error {
	spec := target.Spec
	if verbose {
		riskLabels := target.RiskLabels
		risks := strings.Join(riskLabels, ",")
		if risks == "" {
			risks = "-"
		}
		cost := spec.Metadata.Cost
		if cost == "" {
			cost = "-"
		}
		_, err := fmt.Fprintf(
			stdout,
			"%-*s %-7s %-42s %s\n",
			nameWidth,
			name,
			cost,
			risks,
			spec.Metadata.Description,
		)
		return err
	}
	if spec.Metadata.Description == "" {
		_, err := fmt.Fprintln(stdout, name)
		return err
	}
	_, err := fmt.Fprintf(
		stdout,
		"%-24s %s\n",
		name,
		spec.Metadata.Description,
	)
	return err
}

func formatAliasListEntry(stdout io.Writer, alias *model.Alias) error {
	if alias.Deprecated == "" {
		_, err := fmt.Fprintf(stdout, "%s -> %s\n", alias.Name, alias.Target)
		return err
	}
	_, err := fmt.Fprintf(
		stdout,
		"%s -> %s  %s\n",
		alias.Name,
		alias.Target,
		alias.Deprecated,
	)
	return err
}
