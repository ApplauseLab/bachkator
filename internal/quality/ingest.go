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
	StatePath  string
	RunID      string
	TargetName string
	Workdir    string
	Env        map[string]string
	Reports    []model.QualityReportDeclaration
	Gates      []model.QualityGateSpec
	Log        io.Writer
}

type GateError struct {
	Gates []state.QualityGateResult
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

func IngestReports(ctx context.Context, req IngestRequest) error {
	if len(req.Reports) == 0 && len(req.Gates) == 0 {
		return nil
	}
	reports := make([]state.QualityReport, 0, len(req.Reports))
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
		parsed, err := ParseReport(absPath(req.Workdir, path), declaration)
		if err != nil {
			report.Status = "failed"
			report.Message = err.Error()
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
	if err := state.NewStore(req.StatePath).SaveQualityReports(reports, gates); err != nil {
		return err
	}
	var failures []state.QualityGateResult
	for _, gate := range gates {
		if gate.Status == "failed" {
			failures = append(failures, gate)
		}
	}
	if len(failures) > 0 {
		for _, failure := range failures {
			logf(req.Log, "quality gate failed: %s\n", failure.Message)
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
