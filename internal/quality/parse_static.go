package quality

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func parseGoCyclo(path string) (Report, error) {
	file, err := os.Open(path)
	if err != nil {
		return Report{}, err
	}
	defer func() { _ = file.Close() }()
	var count, total, max float64
	findings := []Finding{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 4 {
			continue
		}
		complexity := parseFloat(parts[0])
		count++
		total += complexity
		if complexity > max {
			max = complexity
		}
		file, lineNumber := splitLocation(parts[len(parts)-1])
		findings = append(
			findings,
			Finding{
				Kind:       "complexity",
				File:       file,
				Line:       lineNumber,
				Severity:   "info",
				Rule:       parts[2],
				Message:    fmt.Sprintf("cyclomatic complexity %.0f", complexity),
				DurationMS: complexity,
			},
		)
	}
	if err := scanner.Err(); err != nil {
		return Report{}, err
	}
	avg := 0.0
	if count > 0 {
		avg = total / count
	}
	return Report{
		Metrics: []Metric{
			{Name: "complexity.max", Value: max, Unit: "count"},
			{Name: "complexity.avg", Value: avg, Unit: "count"},
			{Name: "complexity.functions.count", Value: count, Unit: "count"},
		},
		Findings: findings,
	}, nil
}
