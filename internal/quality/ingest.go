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

	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/model"
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
	Parsers              ReportParsers
	RegoPolicies         []model.RegoPolicySpec
	Gates                []model.QualityGateSpec
	GateEvaluators       GateEvaluators
	SaveReports          func(context.Context, []Report, []GateResult) error
	Log                  io.Writer
	SkipSave             bool
	AllowMissingReports  bool
	TreatFailuresAsNotes bool
	Now                  clock.NowFunc
}

type GateError struct {
	Gates []GateResult
}

type ParseError struct {
	Reports []Report
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
	if len(req.Reports) == 0 && len(req.RegoPolicies) == 0 && len(req.Gates) == 0 {
		return nil
	}
	reports, parseFailures, metricsByName, err := parseDeclaredReports(ctx, req)
	if err != nil {
		return err
	}
	reports, gates, failures := evaluateRegoAndGates(ctx, req, reports, metricsByName)
	if err := saveIngestedReports(ctx, req, reports, gates, parseFailures, failures); err != nil {
		return err
	}
	return finishIngestReports(req, reports, gates, parseFailures, failures)
}

func parseDeclaredReports(
	ctx context.Context,
	req IngestRequest,
) ([]Report, []Report, map[string]float64, error) {
	reports := make([]Report, 0, len(req.Reports))
	var parseFailures []Report
	metricsByName := map[string]float64{}
	metricFormats := map[string]string{}
	for _, declaration := range req.Reports {
		if err := ctx.Err(); err != nil {
			return nil, nil, nil, err
		}
		path := expandEnv(declaration.Path, req.Env)
		report := Report{
			RunID:     req.RunID,
			Target:    req.TargetName,
			Kind:      declaration.Kind,
			Format:    declaration.Format,
			Path:      path,
			Status:    "success",
			CreatedAt: req.now(),
		}
		parsed, err := ParseReport(ParseRequest{
			Context:     ctx,
			Path:        absPath(req.Workdir, path),
			DisplayPath: path,
			Declaration: declaration,
			Parsers:     req.Parsers,
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
			collision := mergeReportMetrics(
				&report,
				parsed,
				declaration,
				metricsByName,
				metricFormats,
			)
			if collision != "" {
				report.Status = "failed"
				report.Message = fmt.Sprintf(
					"policy-metric-collision: metric %q was reported more than once",
					collision,
				)
				parseFailures = append(parseFailures, report)
				logf(req.Log, "quality report %s failed: %s\n", declaration.Path, report.Message)
			} else {
				logf(
					req.Log,
					"quality report %s parsed: metrics=%d findings=%d\n",
					declaration.Path,
					len(parsed.Metrics),
					len(parsed.Findings),
				)
				if report.Status == "failed" {
					parseFailures = append(parseFailures, report)
					logf(
						req.Log,
						"quality report %s failed: %s\n",
						declaration.Path,
						report.Message,
					)
				}
			}
		}
		reports = append(reports, report)
	}
	return reports, parseFailures, metricsByName, nil
}

func mergeReportMetrics(
	report *Report,
	parsed Report,
	declaration model.QualityReportDeclaration,
	metricsByName map[string]float64,
	metricFormats map[string]string,
) string {
	collision := ""
	seenInReport := map[string]struct{}{}
	for _, metric := range parsed.Metrics {
		if metric.Name == "" {
			collision = metric.Name
			break
		}
		if _, exists := seenInReport[metric.Name]; exists {
			collision = metric.Name
			break
		}
		seenInReport[metric.Name] = struct{}{}
		canAggregate := canAggregateMetric(
			metric.Name,
			metricFormats[metric.Name],
			declaration.Format,
		)
		if _, exists := metricsByName[metric.Name]; exists && !canAggregate {
			collision = metric.Name
			break
		}
	}
	if collision != "" {
		return collision
	}
	if parsed.Status != "" {
		report.Status = parsed.Status
	}
	report.Message = parsed.Message
	report.Metrics = parsed.Metrics
	report.Findings = parsed.Findings
	for _, metric := range parsed.Metrics {
		canAggregate := canAggregateMetric(
			metric.Name,
			metricFormats[metric.Name],
			declaration.Format,
		)
		if _, exists := metricsByName[metric.Name]; exists && canAggregate {
			metricsByName[metric.Name] += metric.Value
			continue
		}
		metricsByName[metric.Name] = metric.Value
		metricFormats[metric.Name] = declaration.Format
	}
	return ""
}

func evaluateRegoAndGates(
	ctx context.Context,
	req IngestRequest,
	reports []Report,
	metricsByName map[string]float64,
) ([]Report, []GateResult, []GateResult) {
	policyReports, policyGates := EvaluateRegoPolicies(ctx, RegoEvaluationRequest{
		RunID:       req.RunID,
		TargetName:  req.TargetName,
		ProjectRoot: req.ProjectRoot,
		Workdir:     req.Workdir,
		Env:         req.Env,
		Policies:    req.RegoPolicies,
		Reports:     reports,
		Metrics:     metricsByName,
		Now:         req.Now,
	})
	reports = append(reports, policyReports...)
	gates := EvaluateGatesWithClock(
		req.GateEvaluators,
		req.RunID,
		req.TargetName,
		req.Gates,
		metricsByName,
		req.Now,
	)
	gates = append(gates, policyGates...)
	var failures []GateResult
	for _, gate := range gates {
		if gate.Status == "failed" {
			failures = append(failures, gate)
		}
	}
	return reports, gates, failures
}

func saveIngestedReports(
	ctx context.Context,
	req IngestRequest,
	reports []Report,
	gates []GateResult,
	parseFailures []Report,
	failures []GateResult,
) error {
	if req.SaveReports != nil && (!req.SkipSave || len(parseFailures) > 0 || len(failures) == 0) {
		if err := req.SaveReports(ctx, reports, gates); err != nil {
			return err
		}
	}
	return nil
}

func finishIngestReports(
	req IngestRequest,
	reports []Report,
	gates []GateResult,
	parseFailures []Report,
	failures []GateResult,
) error {
	policyPassed := len(parseFailures) == 0 && len(failures) == 0
	if err := WriteAppliedPolicyArtifact(req, reports, gates, policyPassed); err != nil {
		return err
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

func (req IngestRequest) now() time.Time {
	return clock.UTC(req.Now)
}

func canAggregateMetric(name, existingFormat, nextFormat string) bool {
	return existingFormat == "agent-report-v1" && nextFormat == "agent-report-v1" &&
		strings.HasSuffix(name, ".count")
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
