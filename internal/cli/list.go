package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

func runList(project *Project, verbose bool, includeAliases bool, stdout io.Writer) error {
	names := make([]string, 0, len(project.Targets))
	for name := range project.Targets {
		names = append(names, name)
	}
	sort.Strings(names)
	nameWidth := len("TARGET")
	for _, name := range names {
		if len(name) > nameWidth {
			nameWidth = len(name)
		}
	}
	if verbose {
		_, err := fmt.Fprintf(
			stdout,
			"%-*s %-7s %-42s %s\n",
			nameWidth,
			"TARGET",
			"COST",
			"RISKS",
			"DESCRIPTION",
		)
		if err != nil {
			return err
		}
	}
	for _, name := range names {
		target := project.Targets[name]
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
			if _, err := fmt.Fprintf(
				stdout,
				"%-*s %-7s %-42s %s\n",
				nameWidth,
				name,
				cost,
				risks,
				spec.Metadata.Description,
			); err != nil {
				return err
			}
			continue
		}
		if spec.Metadata.Description == "" {
			if _, err := fmt.Fprintln(stdout, name); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(
			stdout,
			"%-24s %s\n",
			name,
			spec.Metadata.Description,
		); err != nil {
			return err
		}
	}
	if !includeAliases || len(project.Aliases) == 0 {
		return nil
	}
	aliasNames := make([]string, 0, len(project.Aliases))
	for name := range project.Aliases {
		aliasNames = append(aliasNames, name)
	}
	sort.Strings(aliasNames)
	for _, name := range aliasNames {
		alias := project.Aliases[name]
		if alias.Deprecated == "" {
			if _, err := fmt.Fprintf(stdout, "%s -> %s\n", alias.Name, alias.Target); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(
			stdout,
			"%s -> %s  %s\n",
			alias.Name,
			alias.Target,
			alias.Deprecated,
		); err != nil {
			return err
		}
	}
	return nil
}
