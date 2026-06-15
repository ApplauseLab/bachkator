package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunReferencePrintsEmbeddedAsset(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := runReference([]string{"quality-plugin-report-schema"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	if !strings.Contains(output, "# Quality Plugin Report Schema") {
		t.Fatalf("output missing schema heading: %q", output)
	}
	if !strings.Contains(output, `"Bachkator Quality Plugin Report"`) {
		t.Fatalf("output missing schema body: %q", output)
	}
}
