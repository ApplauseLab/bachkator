package quality

import (
	"encoding/xml"
	"os"
)

func parseCheckstyleXML(path string) (Report, error) {
	type checkError struct {
		Line     int    `xml:"line,attr"`
		Severity string `xml:"severity,attr"`
		Message  string `xml:"message,attr"`
		Source   string `xml:"source,attr"`
	}
	type checkFile struct {
		Name   string       `xml:"name,attr"`
		Errors []checkError `xml:"error"`
	}
	type checkstyle struct {
		Files []checkFile `xml:"file"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	var root checkstyle
	if err := xml.Unmarshal(data, &root); err != nil {
		return Report{}, err
	}
	var findings []Finding
	severityCounts := map[string]float64{}
	for _, file := range root.Files {
		for _, issue := range file.Errors {
			severity := issue.Severity
			if severity == "" {
				severity = "warning"
			}
			severityCounts[severity]++
			findings = append(
				findings,
				Finding{
					Kind:     "issue",
					File:     file.Name,
					Line:     issue.Line,
					Severity: severity,
					Rule:     issue.Source,
					Message:  issue.Message,
				},
			)
		}
	}
	metrics := []Metric{
		{Name: "issues.total.count", Value: float64(len(findings)), Unit: "count"},
	}
	for severity, count := range severityCounts {
		metrics = append(
			metrics,
			Metric{Name: "issues." + severity + ".count", Value: count, Unit: "count"},
		)
	}
	return Report{Metrics: metrics, Findings: findings}, nil
}
