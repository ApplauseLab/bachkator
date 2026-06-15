package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

type ValidationReport struct {
	Valid       bool                   `json:"valid"`
	Files       []string               `json:"files"`
	Summary     ValidationSummary      `json:"summary"`
	Diagnostics []ValidationDiagnostic `json:"diagnostics"`
}

type ValidationSummary struct {
	Targets  int `json:"targets"`
	Aliases  int `json:"aliases"`
	Inputs   int `json:"inputs"`
	Profiles int `json:"profiles"`
}

type ValidationDiagnostic struct {
	Severity string          `json:"severity"`
	File     string          `json:"file"`
	Range    DiagnosticRange `json:"range"`
	Message  string          `json:"message"`
	Code     string          `json:"code"`
}

type DiagnosticRange struct {
	Start DiagnosticPosition `json:"start"`
	End   DiagnosticPosition `json:"end"`
}

type DiagnosticPosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type validationFailedError struct{}

func (validationFailedError) Error() string {
	return "Bachfile validation failed"
}

func runValidate(deps Dependencies, opts *options, stdout io.Writer) error {
	if deps.ValidateProject == nil {
		return fmt.Errorf("project validator is not configured")
	}
	report := deps.ValidateProject(
		opts.configPath,
		LoadOptions{Variables: opts.variables, Profiles: opts.profiles},
	)
	switch {
	case opts.json:
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
	case report.Valid:
		_, err := fmt.Fprintf(
			stdout,
			"Bachfile valid: %d targets, %d aliases, %d inputs, %d profiles\n",
			report.Summary.Targets,
			report.Summary.Aliases,
			report.Summary.Inputs,
			report.Summary.Profiles,
		)
		if err != nil {
			return err
		}
	default:
		for _, diag := range report.Diagnostics {
			if _, err := fmt.Fprintf(
				stdout,
				"%s:%d:%d: %s\n",
				diag.File,
				diag.Range.Start.Line,
				diag.Range.Start.Column,
				diag.Message,
			); err != nil {
				return err
			}
		}
	}
	if !report.Valid {
		return validationFailedError{}
	}
	return nil
}
