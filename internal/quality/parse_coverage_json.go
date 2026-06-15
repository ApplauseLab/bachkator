package quality

import (
	"encoding/json"
	"fmt"
	"os"
)

func parseCoverageJSON(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	var values map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		return Report{}, err
	}
	metrics := coverageJSONMetrics(values)
	if len(metrics) == 0 {
		return Report{}, fmt.Errorf(
			"coverage JSON did not contain coverage, line_coverage, line_percent, or totals coverage",
		)
	}
	return Report{Metrics: metrics}, nil
}

func coverageJSONMetrics(values map[string]any) []Metric {
	metrics := []Metric{}
	for _, key := range []string{"coverage", "line_coverage", "line_percent"} {
		if value, ok := numberFromJSON(values[key]); ok {
			metrics = append(
				metrics,
				Metric{Name: "coverage.line.percent", Value: value, Unit: "percent"},
			)
			break
		}
	}
	if totals, ok := values["totals"].(map[string]any); ok {
		for _, key := range []string{"coverage", "line_coverage", "line_percent"} {
			if value, ok := numberFromJSON(totals[key]); ok {
				metrics = append(
					metrics,
					Metric{Name: "coverage.line.percent", Value: value, Unit: "percent"},
				)
				break
			}
		}
	}
	return metrics
}
