package quality

import (
	"fmt"

	"github.com/applause/bachkator/internal/model"
	"github.com/applause/bachkator/internal/state"
)

type Parser interface {
	Parse(path string) (state.QualityReport, error)
}

type parserFunc func(path string) (state.QualityReport, error)

func (f parserFunc) Parse(path string) (state.QualityReport, error) {
	return f(path)
}

type ReportParserRegistry struct {
	parsers map[string]Parser
}

func NewReportParserRegistry() *ReportParserRegistry {
	return &ReportParserRegistry{parsers: map[string]Parser{}}
}

func (r *ReportParserRegistry) Register(format string, parser Parser) error {
	if format == "" {
		return fmt.Errorf("quality report parser has no format")
	}
	if parser == nil {
		return fmt.Errorf("quality report parser for %q is nil", format)
	}
	if _, exists := r.parsers[format]; exists {
		return fmt.Errorf("quality report parser for %q already registered", format)
	}
	r.parsers[format] = parser
	return nil
}

func (r *ReportParserRegistry) Parser(format string) (Parser, error) {
	parser, ok := r.parsers[format]
	if !ok {
		return nil, fmt.Errorf("unsupported quality report format %q", format)
	}
	return parser, nil
}

type GateEvaluator func(runID string, targetName string, gates []model.QualityGateSpec, metrics map[string]float64) []state.QualityGateResult

type GateRegistry struct {
	evaluators map[string]GateEvaluator
}

func NewGateRegistry() *GateRegistry {
	return &GateRegistry{evaluators: map[string]GateEvaluator{}}
}

func (r *GateRegistry) Register(name string, evaluator GateEvaluator) error {
	if name == "" {
		return fmt.Errorf("quality gate evaluator has no name")
	}
	if evaluator == nil {
		return fmt.Errorf("quality gate evaluator %q is nil", name)
	}
	if _, exists := r.evaluators[name]; exists {
		return fmt.Errorf("quality gate evaluator %q already registered", name)
	}
	r.evaluators[name] = evaluator
	return nil
}

func (r *GateRegistry) Evaluator(name string) (GateEvaluator, error) {
	evaluator, ok := r.evaluators[name]
	if !ok {
		return nil, fmt.Errorf("no quality gate evaluator registered for %q", name)
	}
	return evaluator, nil
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
