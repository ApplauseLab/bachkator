package quality

import (
	"bufio"
	"os"
	"strings"
)

func parseGoCover(path string) (Report, error) {
	file, err := os.Open(path)
	if err != nil {
		return Report{}, err
	}
	defer func() { _ = file.Close() }()
	statements, covered, err := scanGoCover(file)
	if err != nil {
		return Report{}, err
	}
	percent := 0.0
	if statements > 0 {
		percent = covered / statements * 100
	}
	return Report{Metrics: coverageLineMetrics(percent, covered, statements)}, nil
}

func scanGoCover(file *os.File) (float64, float64, error) {
	var statements, covered float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		count := parseFloat(parts[1])
		statements += count
		if parseFloat(parts[2]) > 0 {
			covered += count
		}
	}
	return statements, covered, scanner.Err()
}

func coverageLineMetrics(percent float64, covered float64, total float64) []Metric {
	return []Metric{
		{Name: "coverage.line.percent", Value: percent, Unit: "percent"},
		{Name: "coverage.lines.covered", Value: covered, Unit: "count"},
		{Name: "coverage.lines.total", Value: total, Unit: "count"},
	}
}
