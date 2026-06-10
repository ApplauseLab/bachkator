package quality

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/applause/bachkator/internal/model"
	"github.com/applause/bachkator/internal/state"
)

type IngestRequest struct {
	StatePath            string
	RunID                string
	TargetName           string
	ProjectRoot          string
	Workdir              string
	Env                  map[string]string
	Plugins              map[string]*model.Plugin
	Reports              []model.QualityReportDeclaration
	Gates                []model.QualityGateSpec
	Log                  io.Writer
	SkipSave             bool
	AllowMissingReports  bool
	TreatFailuresAsNotes bool
}

type GateError struct {
	Gates []state.QualityGateResult
}

type ParseError struct {
	Reports []state.QualityReport
}

func (e ParseError) Error() string {
	var out strings.Builder
	out.WriteString("quality reports failed:")
	for _, report := range e.Reports {
		fmt.Fprintf(&out, "\n- %s %s failed: %s", report.Kind, report.Path, report.Message)
	}
	return out.String()
}

func (e GateError) Error() string {
	var out strings.Builder
	out.WriteString("quality gates failed:\n")
	for _, gate := range e.Gates {
		fmt.Fprintf(
			&out,
			"- %s %s %.3f failed: actual %.3f for %s\n",
			gate.Metric,
			gate.Op,
			gate.Threshold,
			gate.Actual,
			gate.Target,
		)
	}
	return strings.TrimRight(out.String(), "\n")
}

func IsGateError(err error) bool {
	var gateErr GateError
	return errors.As(err, &gateErr)
}

func IsParseError(err error) bool {
	var parseErr ParseError
	return errors.As(err, &parseErr)
}

func IngestReports(ctx context.Context, req IngestRequest) error {
	if len(req.Reports) == 0 && len(req.Gates) == 0 {
		return nil
	}
	reports := make([]state.QualityReport, 0, len(req.Reports))
	var parseFailures []state.QualityReport
	metricsByName := map[string]float64{}
	for _, declaration := range req.Reports {
		if err := ctx.Err(); err != nil {
			return err
		}
		path := expandEnv(declaration.Path, req.Env)
		report := state.QualityReport{
			RunID:     req.RunID,
			Target:    req.TargetName,
			Kind:      declaration.Kind,
			Format:    declaration.Format,
			Path:      path,
			Status:    "success",
			CreatedAt: time.Now().UTC(),
		}
		parsed, err := ParseReport(ParseRequest{
			Context:     ctx,
			Path:        absPath(req.Workdir, path),
			DisplayPath: path,
			Declaration: declaration,
			Workdir:     req.Workdir,
			ProjectRoot: req.ProjectRoot,
			Env:         req.Env,
			Plugins:     req.Plugins,
			RunID:       req.RunID,
			TargetName:  req.TargetName,
		})
		if err != nil {
			if req.AllowMissingReports && os.IsNotExist(err) {
				logf(
					req.Log,
					"quality report %s missing after command failure; skipped\n",
					declaration.Path,
				)
				continue
			}
			report.Status = "failed"
			report.Message = err.Error()
			parseFailures = append(parseFailures, report)
			logf(req.Log, "quality report %s failed: %s\n", declaration.Path, err)
		} else {
			report.Metrics = parsed.Metrics
			report.Findings = parsed.Findings
			for _, metric := range parsed.Metrics {
				metricsByName[metric.Name] = metric.Value
			}
			logf(
				req.Log,
				"quality report %s parsed: metrics=%d findings=%d\n",
				declaration.Path,
				len(parsed.Metrics),
				len(parsed.Findings),
			)
		}
		reports = append(reports, report)
	}
	gates := EvaluateGates(req.RunID, req.TargetName, req.Gates, metricsByName)
	var failures []state.QualityGateResult
	for _, gate := range gates {
		if gate.Status == "failed" {
			failures = append(failures, gate)
		}
	}
	if !req.SkipSave || len(parseFailures) > 0 || len(failures) == 0 {
		if err := state.NewStore(req.StatePath).SaveQualityReports(reports, gates); err != nil {
			return err
		}
	}
	if len(parseFailures) > 0 {
		if req.TreatFailuresAsNotes {
			return nil
		}
		return ParseError{Reports: parseFailures}
	}
	if len(failures) > 0 {
		for _, failure := range failures {
			logf(req.Log, "quality gate failed: %s\n", failure.Message)
		}
		if req.TreatFailuresAsNotes {
			return nil
		}
		return GateError{Gates: failures}
	}
	return nil
}

func expandEnv(value string, env map[string]string) string {
	value = strings.ReplaceAll(value, "$(RUN_DIRECTORY)", env["BACH_RUN_DIRECTORY"])
	return os.Expand(value, func(key string) string { return env[key] })
}

func absPath(root, path string) string {
	path = expandHome(path)
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func expandHome(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func logf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
