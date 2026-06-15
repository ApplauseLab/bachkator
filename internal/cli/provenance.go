package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

type provenanceJSON struct {
	Paths []pathProvenanceJSON `json:"paths"`
}

type pathProvenanceJSON struct {
	Path      string                 `json:"path"`
	Generated bool                   `json:"generated"`
	Source    bool                   `json:"source"`
	Producers []provenanceTargetJSON `json:"producers"`
	Consumers []provenanceTargetJSON `json:"consumers"`
	Status    string                 `json:"status"`
	Reasons   []string               `json:"reasons"`
}

type provenanceTargetJSON struct {
	Target            string   `json:"target"`
	Operation         string   `json:"operation"`
	RegenerateCommand string   `json:"regenerate_command"`
	Outputs           []string `json:"outputs"`
	Inputs            []string `json:"inputs"`
}

func runProvenance(
	project *Project,
	deps Dependencies,
	jsonOutput bool,
	args []string,
	stdout io.Writer,
) error {
	if len(args) == 0 {
		return fmt.Errorf("provenance requires at least one path")
	}
	if deps.Provenance == nil {
		return fmt.Errorf("provenance service is not configured")
	}
	records, err := deps.Provenance(project, args)
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeProvenanceJSON(stdout, records)
	}
	return writeProvenanceHuman(stdout, records)
}

func writeProvenanceJSON(stdout io.Writer, records []PathProvenance) error {
	out := provenanceJSON{Paths: make([]pathProvenanceJSON, 0, len(records))}
	for _, record := range records {
		out.Paths = append(out.Paths, pathProvenanceJSON{
			Path:      record.Path,
			Generated: record.Generated,
			Source:    record.Source,
			Producers: provenanceTargetsJSON(record.Producers),
			Consumers: provenanceTargetsJSON(record.Consumers),
			Status:    record.Status,
			Reasons:   nonNilStrings(record.Reasons),
		})
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func provenanceTargetsJSON(targets []ProvenanceTarget) []provenanceTargetJSON {
	out := make([]provenanceTargetJSON, 0, len(targets))
	for _, target := range targets {
		out = append(out, provenanceTargetJSON{
			Target:            target.Target,
			Operation:         target.Operation,
			RegenerateCommand: target.RegenerateCommand,
			Outputs:           nonNilStrings(target.Outputs),
			Inputs:            nonNilStrings(target.Inputs),
		})
	}
	return out
}

func writeProvenanceHuman(stdout io.Writer, records []PathProvenance) error {
	for index, record := range records {
		if index > 0 {
			if _, err := fmt.Fprintln(stdout); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(stdout, "%s\n", record.Path); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "generated: %t\n", record.Generated); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "source: %t\n", record.Source); err != nil {
			return err
		}
		if err := writeProvenanceTargets(
			stdout,
			"generated_by",
			record.Producers,
			true,
		); err != nil {
			return err
		}
		if err := writeProvenanceTargets(
			stdout,
			"consumed_by",
			record.Consumers,
			false,
		); err != nil {
			return err
		}
		if len(record.Producers) == 0 && len(record.Consumers) == 0 {
			if _, err := fmt.Fprintln(
				stdout,
				"note: no declared producers or consumers",
			); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(stdout, "status: %s\n", record.Status); err != nil {
			return err
		}
	}
	return nil
}

func writeProvenanceTargets(
	stdout io.Writer,
	label string,
	targets []ProvenanceTarget,
	includeRegenerate bool,
) error {
	if len(targets) == 0 {
		if _, err := fmt.Fprintf(stdout, "%s: []\n", label); err != nil {
			return err
		}
		return nil
	}
	if _, err := fmt.Fprintf(stdout, "%s:\n", label); err != nil {
		return err
	}
	for _, target := range targets {
		if _, err := fmt.Fprintf(stdout, "  - %s\n", target.Target); err != nil {
			return err
		}
		if target.Operation != "" {
			if _, err := fmt.Fprintf(stdout, "    operation: %s\n", target.Operation); err != nil {
				return err
			}
		}
		if includeRegenerate {
			if _, err := fmt.Fprintf(
				stdout,
				"    regenerate: %s\n",
				target.RegenerateCommand,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
