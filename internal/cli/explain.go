package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/applauselab/bachkator/internal/graph"
)

func runExplain(project *Project, deps Dependencies, args []string, stdout io.Writer) error {
	if len(args) != 1 {
		return UsageErrorf("usage: bach explain <target>")
	}
	if deps.ExplainTarget == nil {
		return fmt.Errorf("explain service is not configured")
	}
	record, err := deps.ExplainTarget(project, args[0])
	if err != nil {
		return err
	}
	_, err = fmt.Fprint(stdout, formatExplainRecord(record))
	return err
}

func formatExplainRecord(record graph.ExplainRecord) string {
	var out strings.Builder
	if record.GeneratedPolicy {
		fmt.Fprintf(&out, "%s\n", record.Target)
		fmt.Fprintf(&out, "  generated: true\n")
		fmt.Fprintf(&out, "  subject: %s\n", record.Subject)
		fmt.Fprintf(&out, "  subject_workspace: %s\n", record.SubjectWorkspace)
		fmt.Fprintf(&out, "  subject_commit: %s\n", record.SubjectCommit)
		fmt.Fprintf(&out, "  required_targets: %s\n", strings.Join(record.RequiredTargets, ", "))
		return out.String()
	}
	if record.Alias != "" {
		writeExplainField(&out, "Alias", record.Alias)
		writeExplainField(&out, "Canonical target", record.CanonicalTarget)
		writeExplainField(&out, "Deprecated", record.Deprecated)
	}
	writeExplainField(&out, "Target", record.Target)
	writeExplainField(&out, "Description", record.Description)
	writeExplainField(&out, "When", record.When)
	writeExplainField(&out, "Cost", record.Cost)
	writeExplainField(&out, "Risks", strings.Join(record.Risks, ", "))
	writeExplainList(&out, "Depends on", record.DependsOn)
	writeExplainList(&out, "Steps", record.Steps)
	writeExplainList(&out, "Inputs", record.Inputs)
	writeExplainList(&out, "Outputs", record.Outputs)
	writeExplainList(&out, "Produces", record.Produces)
	writeExplainList(&out, "Required tools", record.RequiredTools)
	writeExplainList(&out, "Preflights", record.Preflights)
	return out.String()
}

func writeExplainField(out *strings.Builder, label string, value string) {
	if value == "" {
		value = "-"
	}
	fmt.Fprintf(out, "%s: %s\n", label, value)
}

func writeExplainList(out *strings.Builder, label string, values []string) {
	if len(values) == 0 {
		writeExplainField(out, label, "")
		return
	}
	fmt.Fprintf(out, "%s:\n", label)
	for _, value := range values {
		fmt.Fprintf(out, "  - %s\n", value)
	}
}
