package quality

import (
	"encoding/xml"
	"fmt"
	"os"
)

func parseCoberturaXML(path string) (Report, error) {
	type coverage struct {
		LineRate     string `xml:"line-rate,attr"`
		BranchRate   string `xml:"branch-rate,attr"`
		LinesValid   string `xml:"lines-valid,attr"`
		LinesCovered string `xml:"lines-covered,attr"`
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	var root coverage
	if err := xml.Unmarshal(data, &root); err != nil {
		return Report{}, err
	}
	return Report{Metrics: []Metric{
		{Name: "coverage.line.percent", Value: parseFloat(root.LineRate) * 100, Unit: "percent"},
		{
			Name:  "coverage.branch.percent",
			Value: parseFloat(root.BranchRate) * 100,
			Unit:  "percent",
		},
		{Name: "coverage.lines.covered", Value: parseFloat(root.LinesCovered), Unit: "count"},
		{Name: "coverage.lines.total", Value: parseFloat(root.LinesValid), Unit: "count"},
	}}, nil
}

func parseJaCoCoXML(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	var root jacocoReport
	if err := xml.Unmarshal(data, &root); err != nil {
		return Report{}, err
	}
	metrics := jacocoMetrics(root.Counters)
	if len(metrics) == 0 {
		return Report{}, fmt.Errorf("JaCoCo XML did not contain supported counters")
	}
	return Report{Metrics: metrics}, nil
}

type jacocoReport struct {
	Counters []jacocoCounter `xml:"counter"`
}

type jacocoCounter struct {
	Type    string `xml:"type,attr"`
	Missed  string `xml:"missed,attr"`
	Covered string `xml:"covered,attr"`
}

func jacocoMetrics(counters []jacocoCounter) []Metric {
	metrics := []Metric{}
	for _, item := range counters {
		covered := parseFloat(item.Covered)
		missed := parseFloat(item.Missed)
		total := covered + missed
		percent := 0.0
		if total > 0 {
			percent = covered / total * 100
		}
		switch item.Type {
		case "LINE":
			metrics = append(metrics, coverageLineMetrics(percent, covered, total)...)
		case "BRANCH":
			metrics = append(
				metrics,
				Metric{Name: "coverage.branch.percent", Value: percent, Unit: "percent"},
			)
		case "COMPLEXITY":
			metrics = append(metrics, Metric{Name: "complexity.total", Value: total, Unit: "count"})
		}
	}
	return metrics
}
