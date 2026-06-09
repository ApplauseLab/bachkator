package quality

import "testing"

func TestReportParserRegistryRejectsDuplicateFormat(t *testing.T) {
	registry := NewReportParserRegistry()
	parser := parserFunc(parseJUnitXML)
	if err := registry.Register("junit-xml", parser); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("junit-xml", parser); err == nil {
		t.Fatal("duplicate parser registered without error")
	}
}

func TestReportParserRegistryReportsMissingParser(t *testing.T) {
	registry := NewReportParserRegistry()
	if _, err := registry.Parser("missing"); err == nil {
		t.Fatal("missing parser returned no error")
	}
}

func TestBuiltinReportParserRegistryWiresKnownFormats(t *testing.T) {
	registry := BuiltinReportParserRegistry()
	for _, format := range []string{"junit-xml", "checkstyle-xml", "lcov", "cobertura-xml", "jacoco-xml", "go-cover", "gocover", "gocov-json", "codecov-json", "gocyclo"} {
		if _, err := registry.Parser(format); err != nil {
			t.Fatalf("parser for %q: %v", format, err)
		}
	}
}

func TestGateRegistryRejectsDuplicateEvaluator(t *testing.T) {
	registry := NewGateRegistry()
	if err := registry.Register("threshold", evaluateThresholdGates); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register("threshold", evaluateThresholdGates); err == nil {
		t.Fatal("duplicate gate evaluator registered without error")
	}
}

func TestGateRegistryReportsMissingEvaluator(t *testing.T) {
	registry := NewGateRegistry()
	if _, err := registry.Evaluator("missing"); err == nil {
		t.Fatal("missing gate evaluator returned no error")
	}
}
