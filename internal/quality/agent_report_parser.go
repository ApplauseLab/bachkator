package quality

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/applauselab/bachkator/internal/quality/agentreport"
)

type agentQualityReport struct {
	Status   string    `json:"status"`
	Metrics  []Metric  `json:"metrics"`
	Findings []Finding `json:"findings"`
}

func parseAgentReportJSON(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	var report agentQualityReport
	if err := json.Unmarshal(data, &report); err != nil {
		return Report{}, err
	}
	if report.Status != "" && report.Status != "pass" && report.Status != "passed" {
		report.Findings = append(report.Findings, Finding{
			Kind:     "agent-report",
			Severity: "error",
			Rule:     "agent-report-non-pass",
			Message:  "agent report status is " + report.Status,
		})
	}
	return Report{Metrics: report.Metrics, Findings: report.Findings}, nil
}

func parseAgentReportV1(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	var report agentreport.Report
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&report); err != nil {
		return Report{}, err
	}
	if err := rejectTrailingJSON(decoder); err != nil {
		return Report{}, err
	}
	if report.Schema != agentreport.Schema {
		return Report{}, fmt.Errorf("agent report schema must be bach.agent_report.v1")
	}
	if report.Agent.Role == "" {
		return Report{}, fmt.Errorf("agent report agent.role is required")
	}
	if report.Status == "" {
		report.Status = "success"
	}
	findings, blockingFindings, err := agentReportFindings(report)
	if err != nil {
		return Report{}, err
	}
	metrics, err := agentReportMetrics(report, blockingFindings)
	if err != nil {
		return Report{}, err
	}
	parsed := Report{Status: "success", Metrics: metrics, Findings: findings}
	if !strings.EqualFold(report.Status, "success") {
		parsed.Status = "failed"
		parsed.Message = "agent report status is " + report.Status
	}
	return parsed, nil
}

func agentReportFindings(report agentreport.Report) ([]Finding, int, error) {
	blockingFindings := 0
	findings := make([]Finding, 0, len(report.Findings))
	for _, finding := range report.Findings {
		if finding.Kind == "" {
			return nil, 0, fmt.Errorf("agent report finding kind is required")
		}
		switch strings.ToLower(finding.Severity) {
		case "blocker", "blocking", "critical", "error", "failure", "failed":
			blockingFindings++
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
	return findings, blockingFindings, nil
}

func agentReportMetrics(report agentreport.Report, blockingFindings int) ([]Metric, error) {
	role := strings.NewReplacer("/", ".", " ", "_", "-", "_").Replace(report.Agent.Role)
	metrics := []Metric{
		{Name: "agent." + role + ".report.count", Value: 1, Unit: "count"},
		{
			Name:  "agent." + role + ".findings.count",
			Value: float64(len(report.Findings)),
			Unit:  "count",
		},
		{
			Name:  "policy." + role + ".findings.count",
			Value: float64(len(report.Findings)),
			Unit:  "count",
		},
		{
			Name:  "policy." + role + ".blocking_findings.count",
			Value: float64(blockingFindings),
			Unit:  "count",
		},
	}
	statusValue := float64(0)
	if strings.EqualFold(report.Status, "success") {
		statusValue = 1
	}
	metrics = append(metrics, Metric{
		Name:  "agent." + role + ".status.success",
		Value: statusValue,
		Unit:  "bool",
	})
	for _, metric := range report.Metrics {
		if metric.Name == "" {
			return nil, fmt.Errorf("agent report metric name is required")
		}
		if strings.HasPrefix(metric.Name, "agent.") || strings.HasPrefix(metric.Name, "policy.") {
			return nil, fmt.Errorf(
				"agent report metric %q uses reserved agent/policy namespace",
				metric.Name,
			)
		}
	}
	for _, metric := range report.Metrics {
		metrics = append(metrics, Metric{
			Name:  metric.Name,
			Scope: metric.Scope,
			Value: metric.Value,
			Unit:  metric.Unit,
		})
	}
	return metrics, nil
}
