package main

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadBaselineParsesCommentsAndSlashPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.txt")
	contents := strings.Join([]string{
		"# generated baseline",
		"",
		"internal/example.go 501",
		"cmd/tool/main.go 12",
	}, "\n")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}

	baseline, err := readBaseline(path)
	if err != nil {
		t.Fatalf("readBaseline() error = %v", err)
	}
	want := map[string]int{
		"internal/example.go": 501,
		"cmd/tool/main.go":    12,
	}
	if !maps.Equal(baseline, want) {
		t.Fatalf("readBaseline() = %#v, want %#v", baseline, want)
	}
}

func TestReadBaselineRejectsMalformedRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.txt")
	if err := os.WriteFile(path, []byte("internal/example.go nope\n"), 0o600); err != nil {
		t.Fatalf("write baseline: %v", err)
	}

	_, err := readBaseline(path)
	if err == nil {
		t.Fatal("readBaseline() error = nil, want invalid limit error")
	}
	if !strings.Contains(err.Error(), "invalid limit") {
		t.Fatalf("readBaseline() error = %q, want invalid limit", err)
	}
}

func TestCountLinesCountsScannerLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "example.go")
	if err := os.WriteFile(path, []byte("package main\n\nfunc main() {}\n"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	lines, err := countLines(path)
	if err != nil {
		t.Fatalf("countLines() error = %v", err)
	}
	if lines != 3 {
		t.Fatalf("countLines() = %d, want 3", lines)
	}
}

func TestShouldSkipDirMatchesGeneratedAndVendorDirs(t *testing.T) {
	skipped := []string{".git", ".bach", "dist", "node_modules", "vendor"}
	for _, dir := range skipped {
		if !shouldSkipDir(filepath.Join("root", dir)) {
			t.Fatalf("shouldSkipDir(%q) = false, want true", dir)
		}
	}
	if shouldSkipDir(filepath.Join("root", "internal")) {
		t.Fatal("shouldSkipDir(internal) = true, want false")
	}
}
