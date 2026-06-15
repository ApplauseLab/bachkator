package quality

import (
	"fmt"

	"github.com/applauselab/bachkator/internal/model"
	typedregistry "github.com/applauselab/bachkator/internal/registry"
)

type Parser interface {
	Parse(path string) (Report, error)
}

type ReportParsers interface {
	Parser(format string) (Parser, error)
}

type parserFunc func(path string) (Report, error)

func (f parserFunc) Parse(path string) (Report, error) {
	return f(path)
}

type ReportParserRegistry struct {
	parsers typedregistry.Registry[string, Parser]
}

func NewReportParserRegistry() *ReportParserRegistry {
	return &ReportParserRegistry{}
}

func (r *ReportParserRegistry) Register(format string, parser Parser) error {
	if format == "" {
		return fmt.Errorf("quality report parser has no format")
	}
	if parser == nil {
		return fmt.Errorf("quality report parser for %q is nil", format)
	}
	return r.parsers.Register(format, parser, duplicateReportParserError)
}

func (r *ReportParserRegistry) Parser(format string) (Parser, error) {
	return r.parsers.Get(format, missingReportParserError)
}

func duplicateReportParserError(format string) error {
	return fmt.Errorf("quality report parser for %q already registered", format)
}

func missingReportParserError(format string) error {
	return fmt.Errorf("unsupported quality report format %q", format)
}

type GateEvaluator func(runID string, targetName string, gates []model.QualityGateSpec, metrics map[string]float64) []GateResult

type GateEvaluators interface {
	Evaluator(name string) (GateEvaluator, error)
}

type GateRegistry struct {
	evaluators typedregistry.Registry[string, GateEvaluator]
}

func NewGateRegistry() *GateRegistry {
	return &GateRegistry{}
}

func (r *GateRegistry) Register(name string, evaluator GateEvaluator) error {
	if name == "" {
		return fmt.Errorf("quality gate evaluator has no name")
	}
	if evaluator == nil {
		return fmt.Errorf("quality gate evaluator %q is nil", name)
	}
	return r.evaluators.Register(name, evaluator, duplicateGateEvaluatorError)
}

func (r *GateRegistry) Evaluator(name string) (GateEvaluator, error) {
	return r.evaluators.Get(name, missingGateEvaluatorError)
}

func duplicateGateEvaluatorError(name string) error {
	return fmt.Errorf("quality gate evaluator %q already registered", name)
}

func missingGateEvaluatorError(name string) error {
	return fmt.Errorf("no quality gate evaluator registered for %q", name)
}

func BuiltinReportParserRegistry() *ReportParserRegistry {
	registry := NewReportParserRegistry()
	mustRegisterReportParser(registry, "junit-xml", parserFunc(parseJUnitXML))
	mustRegisterReportParser(registry, "checkstyle-xml", parserFunc(parseCheckstyleXML))
	mustRegisterReportParser(registry, "lcov", parserFunc(parseLCOV))
	mustRegisterReportParser(registry, "cobertura-xml", parserFunc(parseCoberturaXML))
	mustRegisterReportParser(registry, "jacoco-xml", parserFunc(parseJaCoCoXML))
	mustRegisterReportParser(registry, "go-cover", parserFunc(parseGoCover))
	mustRegisterReportParser(registry, "gocover", parserFunc(parseGoCover))
	mustRegisterReportParser(registry, "gocov-json", parserFunc(parseCoverageJSON))
	mustRegisterReportParser(registry, "codecov-json", parserFunc(parseCoverageJSON))
	mustRegisterReportParser(registry, "gocyclo", parserFunc(parseGoCyclo))
	mustRegisterReportParser(registry, "agent-report-json", parserFunc(parseAgentReportJSON))
	mustRegisterReportParser(registry, "agent-report-v1", parserFunc(parseAgentReportV1))
	return registry
}

func BuiltinGateRegistry() *GateRegistry {
	registry := NewGateRegistry()
	mustRegisterGateEvaluator(registry, "threshold", evaluateThresholdGates)
	return registry
}

func mustRegisterReportParser(registry *ReportParserRegistry, format string, parser Parser) {
	if err := registry.Register(format, parser); err != nil {
		panic(err)
	}
}

func mustRegisterGateEvaluator(registry *GateRegistry, name string, evaluator GateEvaluator) {
	if err := registry.Register(name, evaluator); err != nil {
		panic(err)
	}
}
