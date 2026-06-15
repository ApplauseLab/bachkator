package quality

import (
	"bufio"
	"os"
	"strings"
)

func parseLCOV(path string) (Report, error) {
	file, err := os.Open(path)
	if err != nil {
		return Report{}, err
	}
	defer func() { _ = file.Close() }()
	var found, hit float64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "LF:") {
			found += parseFloat(strings.TrimPrefix(line, "LF:"))
		}
		if strings.HasPrefix(line, "LH:") {
			hit += parseFloat(strings.TrimPrefix(line, "LH:"))
		}
	}
	if err := scanner.Err(); err != nil {
		return Report{}, err
	}
	percent := 0.0
	if found > 0 {
		percent = hit / found * 100
	}
	return Report{
		Metrics: []Metric{
			{Name: "coverage.line.percent", Value: percent, Unit: "percent"},
			{Name: "coverage.lines.covered", Value: hit, Unit: "count"},
			{Name: "coverage.lines.total", Value: found, Unit: "count"},
		},
	}, nil
}
