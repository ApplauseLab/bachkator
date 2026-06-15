package quality

import (
	"fmt"
	"time"

	"github.com/applauselab/bachkator/internal/clock"
	"github.com/applauselab/bachkator/internal/model"
)

func EvaluateGates(
	runID string,
	targetName string,
	gates []model.QualityGateSpec,
	metrics map[string]float64,
) []GateResult {
	return EvaluateGatesWithClock(BuiltinGateRegistry(), runID, targetName, gates, metrics, nil)
}

func EvaluateGatesWith(
	evaluators GateEvaluators,
	runID string,
	targetName string,
	gates []model.QualityGateSpec,
	metrics map[string]float64,
) []GateResult {
	return EvaluateGatesWithClock(evaluators, runID, targetName, gates, metrics, nil)
}

func EvaluateGatesWithClock(
	evaluators GateEvaluators,
	runID string,
	targetName string,
	gates []model.QualityGateSpec,
	metrics map[string]float64,
	now clock.NowFunc,
) []GateResult {
	createdAt := clock.UTC(now)
	if evaluators == nil {
		evaluators = BuiltinGateRegistry()
	}
	evaluator, err := evaluators.Evaluator("threshold")
	if err != nil {
		return []GateResult{
			{
				RunID:     runID,
				Target:    targetName,
				Status:    "failed",
				Message:   err.Error(),
				CreatedAt: createdAt,
			},
		}
	}
	results := evaluator(runID, targetName, gates, metrics)
	stampGateResults(results, createdAt)
	return results
}

func stampGateResults(results []GateResult, createdAt time.Time) {
	for i := range results {
		if results[i].CreatedAt.IsZero() {
			results[i].CreatedAt = createdAt
		}
	}
}

func evaluateThresholdGates(
	runID string,
	targetName string,
	gates []model.QualityGateSpec,
	metrics map[string]float64,
) []GateResult {
	results := make([]GateResult, 0, len(gates)*2)
	for _, gate := range gates {
		actual, ok := metrics[gate.Metric]
		if !ok {
			results = append(results, missingMetricGateResult(runID, targetName, gate.Metric))
			continue
		}
		if gate.Min != nil {
			results = append(
				results,
				thresholdGateResult(runID, targetName, gate.Metric, ">=", *gate.Min, actual),
			)
		}
		if gate.Max != nil {
			results = append(
				results,
				thresholdGateResult(runID, targetName, gate.Metric, "<=", *gate.Max, actual),
			)
		}
	}
	return results
}

func missingMetricGateResult(runID string, targetName string, metric string) GateResult {
	return GateResult{
		RunID:   runID,
		Target:  targetName,
		Metric:  metric,
		Op:      "exists",
		Status:  "failed",
		Message: fmt.Sprintf("metric %q was not reported", metric),
	}
}

func thresholdGateResult(
	runID string,
	targetName string,
	metric string,
	op string,
	threshold float64,
	actual float64,
) GateResult {
	status := "success"
	if op == ">=" && actual < threshold {
		status = "failed"
	}
	if op == "<=" && actual > threshold {
		status = "failed"
	}
	return GateResult{
		RunID:     runID,
		Target:    targetName,
		Metric:    metric,
		Op:        op,
		Threshold: threshold,
		Actual:    actual,
		Status:    status,
		Message:   fmt.Sprintf("%s actual %.3f must be %s %.3f", metric, actual, op, threshold),
	}
}
