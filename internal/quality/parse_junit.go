package quality

import (
	"encoding/xml"
	"os"
)

func parseJUnitXML(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	var rootSuites junitTestSuites
	if err := xml.Unmarshal(data, &rootSuites); err == nil && len(rootSuites.Suites) > 0 {
		return junitFromSuites(rootSuites.Suites), nil
	}
	var suite junitTestSuite
	if err := xml.Unmarshal(data, &suite); err != nil {
		return Report{}, err
	}
	return junitFromSuites([]junitTestSuite{suite}), nil
}

func junitFromSuites(suites []junitTestSuite) Report {
	var tests, failures, errors, skipped int
	var duration float64
	findings := []Finding{}
	var walk func(junitTestSuite)
	walk = func(suite junitTestSuite) {
		tests += suite.Tests
		failures += suite.Failures
		errors += suite.Errors
		skipped += suite.Skipped
		duration += suite.Time
		for _, test := range suite.Cases {
			name := test.Name
			if test.Classname != "" {
				name = test.Classname + "." + test.Name
			}
			findings = append(
				findings,
				Finding{Kind: "test", Rule: name, DurationMS: test.Time * 1000},
			)
			for _, failure := range test.Failure {
				findings = append(
					findings,
					Finding{
						Kind:       "test-failure",
						Severity:   "failure",
						Rule:       name,
						Message:    failure.Message,
						DurationMS: test.Time * 1000,
					},
				)
			}
			for _, testError := range test.Error {
				findings = append(
					findings,
					Finding{
						Kind:       "test-failure",
						Severity:   "error",
						Rule:       name,
						Message:    testError.Message,
						DurationMS: test.Time * 1000,
					},
				)
			}
		}
		for _, child := range suite.Suites {
			walk(child)
		}
	}
	for _, suite := range suites {
		walk(suite)
	}
	return Report{Metrics: []Metric{
		{Name: "tests.total", Value: float64(tests), Unit: "count"},
		{Name: "tests.failed", Value: float64(failures + errors), Unit: "count"},
		{Name: "tests.skipped", Value: float64(skipped), Unit: "count"},
		{Name: "tests.duration.ms", Value: duration * 1000, Unit: "ms"},
	}, Findings: findings}
}
