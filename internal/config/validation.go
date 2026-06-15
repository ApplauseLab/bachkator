package config

import (
	"errors"
	"strings"

	"github.com/hashicorp/hcl/v2"
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

type validationLoadError struct {
	err         error
	diagnostics []ValidationDiagnostic
}

func (e *validationLoadError) Error() string {
	return e.err.Error()
}

func (e *validationLoadError) Unwrap() error {
	return e.err
}

func ValidateWithOptions(path string, options LoadOptions) ValidationReport {
	project, err := loadWithOptions(path, options, false)
	report := ValidationReport{Valid: err == nil, Files: []string{path}}
	if project != nil {
		report.Summary = validationSummary(project)
	}
	if err == nil {
		return report
	}

	var loadErr *validationLoadError
	if errors.As(err, &loadErr) {
		report.Diagnostics = append(report.Diagnostics, loadErr.diagnostics...)
		return report
	}
	report.Diagnostics = []ValidationDiagnostic{semanticDiagnostic(path, err)}
	return report
}

func validationSummary(project *Project) ValidationSummary {
	return ValidationSummary{
		Targets:  len(project.Targets),
		Aliases:  len(project.Aliases),
		Inputs:   len(project.Inputs),
		Profiles: project.ProfileCount,
	}
}

func diagnosticError(path string, err error, diags hcl.Diagnostics, code string) error {
	return &validationLoadError{err: err, diagnostics: diagnosticsFromHCL(path, diags, code)}
}

func diagnosticsFromHCL(path string, diags hcl.Diagnostics, code string) []ValidationDiagnostic {
	out := make([]ValidationDiagnostic, 0, len(diags))
	for _, diag := range diags {
		if diag == nil {
			continue
		}
		file := path
		rangeValue := fallbackRange()
		if diag.Subject != nil {
			if diag.Subject.Filename != "" {
				file = diag.Subject.Filename
			}
			rangeValue = DiagnosticRange{
				Start: DiagnosticPosition{
					Line:   diag.Subject.Start.Line,
					Column: diag.Subject.Start.Column,
				},
				End: DiagnosticPosition{
					Line:   diag.Subject.End.Line,
					Column: diag.Subject.End.Column,
				},
			}
		}
		out = append(out, ValidationDiagnostic{
			Severity: severityFromHCL(diag.Severity),
			File:     file,
			Range:    rangeValue,
			Message:  diag.Error(),
			Code:     code,
		})
	}
	return out
}

func semanticDiagnostic(path string, err error) ValidationDiagnostic {
	message := err.Error()
	return ValidationDiagnostic{
		Severity: "error",
		File:     path,
		Range:    fallbackRange(),
		Message:  message,
		Code:     codeForMessage(message),
	}
}

func fallbackRange() DiagnosticRange {
	return DiagnosticRange{
		Start: DiagnosticPosition{Line: 1, Column: 1},
		End:   DiagnosticPosition{Line: 1, Column: 1},
	}
}

func severityFromHCL(severity hcl.DiagnosticSeverity) string {
	if severity == hcl.DiagWarning {
		return "warning"
	}
	return "error"
}

func codeForMessage(message string) string {
	switch {
	case strings.Contains(message, "obsolete target reference"):
		return "obsolete-target-reference"
	case strings.Contains(message, "unknown target") ||
		strings.Contains(message, "missing target") ||
		strings.Contains(message, "does not exist"):
		return "unknown-target-reference"
	case strings.Contains(message, "duplicate target"):
		return "duplicate-target"
	case strings.Contains(message, "duplicate alias"):
		return "duplicate-alias"
	case strings.Contains(message, "duplicate input"):
		return "duplicate-input"
	case strings.Contains(message, "missing project block"):
		return "missing-project-block"
	case strings.Contains(message, "parse "):
		return "hcl-parse-error"
	case strings.Contains(message, "decode "):
		return "hcl-decode-error"
	default:
		return "bachfile-validation-error"
	}
}
