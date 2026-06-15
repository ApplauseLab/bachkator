package quality

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/model"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/rego"
)

const regoPolicyFormat = "rego-policy-v1"

type RegoEvaluationRequest struct {
	RunID       string
	TargetName  string
	ProjectRoot string
	Workdir     string
	Env         map[string]string
	Policies    []model.RegoPolicySpec
	Reports     []Report
	Metrics     map[string]float64
	Now         clock.NowFunc
}

type regoDecision struct {
	Allow    bool
	Findings []Finding
}

func EvaluateRegoPolicies(
	ctx context.Context,
	req RegoEvaluationRequest,
) ([]Report, []GateResult) {
	if len(req.Policies) == 0 {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	input := buildRegoInput(req)
	reports := make([]Report, 0, len(req.Policies))
	gates := []GateResult{}
	createdAt := clock.UTC(req.Now)
	for _, policy := range req.Policies {
		path := expandEnv(policy.Path, req.Env)
		report := Report{
			RunID:     req.RunID,
			Target:    req.TargetName,
			Kind:      "policy",
			Format:    regoPolicyFormat,
			Path:      path,
			Status:    "success",
			CreatedAt: createdAt,
		}
		decision, err := evaluateRegoPolicy(ctx, req.ProjectRoot, policy, input)
		if err != nil {
			report.Status = "failed"
			report.Message = err.Error()
			reports = append(reports, report)
			gates = append(gates, regoGateFailure(req, policy, err.Error(), createdAt))
			continue
		}
		report.Findings = decision.Findings
		if !decision.Allow {
			report.Status = "failed"
			report.Message = fmt.Sprintf("rego policy %q denied target", policy.Path)
			gates = append(gates, regoGateFailure(req, policy, report.Message, createdAt))
		}
		reports = append(reports, report)
	}
	return reports, gates
}

func evaluateRegoPolicy(
	ctx context.Context,
	projectRoot string,
	policy model.RegoPolicySpec,
	input map[string]any,
) (regoDecision, error) {
	abs := absPath(projectRoot, policy.Path)
	source, err := os.ReadFile(abs)
	if err != nil {
		return regoDecision{}, err
	}
	allowValue, err := evaluateRegoQuery(ctx, policy, abs, source, allowQuery(policy), input)
	if err != nil {
		return regoDecision{}, err
	}
	allow, ok := allowValue.(bool)
	if !ok {
		return regoDecision{}, fmt.Errorf(
			"rego policy %q allow must evaluate to a boolean",
			policy.Path,
		)
	}
	findingsValue, err := evaluateOptionalRegoQuery(
		ctx,
		policy,
		abs,
		source,
		findingsQuery(policy),
		input,
	)
	if err != nil {
		return regoDecision{}, err
	}
	findings, err := decodeRegoFindings(findingsValue)
	if err != nil {
		return regoDecision{}, fmt.Errorf("rego policy %q findings: %w", policy.Path, err)
	}
	return regoDecision{Allow: allow, Findings: findings}, nil
}

func evaluateRegoQuery(
	ctx context.Context,
	policy model.RegoPolicySpec,
	modulePath string,
	source []byte,
	query string,
	input map[string]any,
) (any, error) {
	if query == "" {
		return nil, fmt.Errorf("rego policy %q query is required", policy.Path)
	}
	results, err := rego.New(
		rego.Query(query),
		rego.Module(filepath.ToSlash(modulePath), string(source)),
		rego.Input(input),
		rego.Capabilities(restrictedRegoCapabilities()),
	).Eval(ctx)
	if err != nil {
		return nil, fmt.Errorf("rego policy %q query %q failed: %w", policy.Path, query, err)
	}
	if len(results) != 1 || len(results[0].Expressions) != 1 {
		return nil, fmt.Errorf("rego policy %q query %q must return one value", policy.Path, query)
	}
	return results[0].Expressions[0].Value, nil
}

func evaluateOptionalRegoQuery(
	ctx context.Context,
	policy model.RegoPolicySpec,
	modulePath string,
	source []byte,
	query string,
	input map[string]any,
) (any, error) {
	if query == "" {
		return nil, nil
	}
	results, err := rego.New(
		rego.Query(query),
		rego.Module(filepath.ToSlash(modulePath), string(source)),
		rego.Input(input),
		rego.Capabilities(restrictedRegoCapabilities()),
	).Eval(ctx)
	if err != nil {
		return nil, fmt.Errorf("rego policy %q query %q failed: %w", policy.Path, query, err)
	}
	if len(results) == 0 {
		return nil, nil
	}
	if len(results) != 1 || len(results[0].Expressions) != 1 {
		return nil, fmt.Errorf("rego policy %q query %q must return one value", policy.Path, query)
	}
	return results[0].Expressions[0].Value, nil
}

func restrictedRegoCapabilities() *ast.Capabilities {
	caps := ast.CapabilitiesForThisVersion()
	blocked := map[string]struct{}{
		"http.send":   {},
		"opa.runtime": {},
	}
	builtins := caps.Builtins[:0]
	for _, builtin := range caps.Builtins {
		if _, ok := blocked[builtin.Name]; ok {
			continue
		}
		builtins = append(builtins, builtin)
	}
	caps.Builtins = builtins
	return caps
}

func allowQuery(policy model.RegoPolicySpec) string {
	if policy.Allow != "" {
		return policy.Allow
	}
	if policy.Package == "" {
		return ""
	}
	return "data." + policy.Package + ".allow"
}

func findingsQuery(policy model.RegoPolicySpec) string {
	if policy.Findings != "" {
		return policy.Findings
	}
	if policy.Package == "" {
		return ""
	}
	return "data." + policy.Package + ".findings"
}

func decodeRegoFindings(value any) ([]Finding, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var raw []struct {
		Kind       string  `json:"kind"`
		File       string  `json:"file"`
		Line       int     `json:"line"`
		Severity   string  `json:"severity"`
		Rule       string  `json:"rule"`
		Message    string  `json:"message"`
		DurationMS float64 `json:"duration_ms"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}
	findings := make([]Finding, 0, len(raw))
	for _, finding := range raw {
		if finding.Kind == "" {
			return nil, fmt.Errorf("finding kind is required")
		}
		findings = append(findings, Finding{
			Kind:       finding.Kind,
			File:       finding.File,
			Line:       finding.Line,
			Severity:   finding.Severity,
			Rule:       finding.Rule,
			Message:    finding.Message,
			DurationMS: finding.DurationMS,
		})
	}
	return findings, nil
}

func buildRegoInput(req RegoEvaluationRequest) map[string]any {
	return map[string]any{
		"schema":   "bach.rego_input.v1",
		"run":      map[string]any{"id": req.RunID},
		"target":   regoTargetInput(req.TargetName),
		"git":      regoGitInput(req.Env),
		"reports":  regoReportsInput(req.Reports),
		"metrics":  req.Metrics,
		"findings": regoFindingsInput(req.Reports),
	}
}

func regoTargetInput(name string) map[string]any {
	kind := ""
	if before, _, ok := strings.Cut(name, "/"); ok {
		kind = before
	}
	return map[string]any{"name": name, "kind": kind}
}

func regoGitInput(env map[string]string) map[string]any {
	dirty := strings.EqualFold(env["BACH_GIT_DIRTY"], "true") || env["BACH_GIT_DIRTY"] == "1"
	return map[string]any{
		"commit": env["BACH_GIT_COMMIT"],
		"branch": env["BACH_GIT_BRANCH"],
		"dirty":  dirty,
	}
}

func regoReportsInput(reports []Report) []map[string]any {
	out := make([]map[string]any, 0, len(reports))
	for _, report := range reports {
		out = append(out, map[string]any{
			"kind":     report.Kind,
			"format":   report.Format,
			"path":     report.Path,
			"status":   report.Status,
			"message":  report.Message,
			"metrics":  regoMetricsInput(report.Metrics),
			"findings": regoQualityFindingsInput(report.Findings),
		})
	}
	return out
}

func regoMetricsInput(metrics []Metric) []map[string]any {
	out := make([]map[string]any, 0, len(metrics))
	for _, metric := range metrics {
		out = append(out, map[string]any{
			"name":  metric.Name,
			"scope": metric.Scope,
			"value": metric.Value,
			"unit":  metric.Unit,
		})
	}
	return out
}

func regoFindingsInput(reports []Report) []map[string]any {
	findings := []map[string]any{}
	for _, report := range reports {
		if report.Status != "success" {
			continue
		}
		findings = append(findings, regoQualityFindingsInput(report.Findings)...)
	}
	return findings
}

func regoQualityFindingsInput(findings []Finding) []map[string]any {
	out := make([]map[string]any, 0, len(findings))
	for _, finding := range findings {
		out = append(out, map[string]any{
			"kind":        finding.Kind,
			"file":        finding.File,
			"line":        finding.Line,
			"severity":    finding.Severity,
			"rule":        finding.Rule,
			"message":     finding.Message,
			"duration_ms": finding.DurationMS,
		})
	}
	return out
}

func regoGateFailure(
	req RegoEvaluationRequest,
	_ model.RegoPolicySpec,
	message string,
	createdAt time.Time,
) GateResult {
	return GateResult{
		RunID:     req.RunID,
		Target:    req.TargetName,
		Metric:    "rego.policy.allow",
		Op:        "allow",
		Status:    "failed",
		Message:   message,
		CreatedAt: createdAt,
	}
}
