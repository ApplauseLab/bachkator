package cli

import (
	"io"
	"sort"
)

func runList(
	project *Project,
	verbose bool,
	includeAliases bool,
	includeGenerated bool,
	stdout io.Writer,
) error {
	names := make([]string, 0, len(project.Targets))
	for name := range project.Targets {
		names = append(names, name)
	}
	if includeGenerated {
		for name := range project.Policies {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	nameWidth := len("TARGET")
	for _, name := range names {
		if len(name) > nameWidth {
			nameWidth = len(name)
		}
	}
	if verbose {
		if err := formatListHeader(stdout, nameWidth); err != nil {
			return err
		}
	}
	for _, name := range names {
		if policy := project.Policies[name]; policy != nil {
			if err := formatPolicyListEntry(stdout, nameWidth, name, policy, verbose); err != nil {
				return err
			}
			continue
		}
		target := project.Targets[name]
		if err := formatTargetListEntry(stdout, nameWidth, name, target, verbose); err != nil {
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
		if err := formatAliasListEntry(stdout, project.Aliases[name]); err != nil {
			return err
		}
	}
	return nil
}
