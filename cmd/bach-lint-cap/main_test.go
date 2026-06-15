package main

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCappedFilesKeepsFirstFindingsAcrossFiles(t *testing.T) {
	files := []checkstyleFile{
		{
			Name: "a.go",
			Errors: []checkstyleError{
				{Line: "1", Message: "first"},
				{Line: "2", Message: "second"},
			},
		},
		{
			Name: "empty.go",
		},
		{
			Name: "b.go",
			Errors: []checkstyleError{
				{Line: "3", Message: "third"},
				{Line: "4", Message: "fourth"},
			},
		},
	}

	capped := cappedFiles(files, 3)

	if got := countFindings(capped); got != 3 {
		t.Fatalf("countFindings(capped) = %d, want 3", got)
	}
	if len(capped) != 2 {
		t.Fatalf("len(capped) = %d, want 2", len(capped))
	}
	if capped[1].Name != "b.go" || len(capped[1].Errors) != 1 {
		t.Fatalf("second capped file = %#v, want one finding from b.go", capped[1])
	}
	if len(files[1].Errors) != 0 || len(files[2].Errors) != 2 {
		t.Fatalf("cappedFiles mutated input: %#v", files)
	}
}

func TestCappedFilesZeroLimitDropsFindings(t *testing.T) {
	files := []checkstyleFile{{Name: "a.go", Errors: []checkstyleError{{Message: "first"}}}}

	capped := cappedFiles(files, 0)

	if capped != nil {
		t.Fatalf("cappedFiles(..., 0) = %#v, want nil", capped)
	}
}

func TestWriteCheckstyleCreatesParentAndDefaultsVersion(t *testing.T) {
	outPath := filepath.Join(t.TempDir(), "nested", "checkstyle.xml")
	doc := checkstyle{
		Files: []checkstyleFile{
			{Name: "a.go", Errors: []checkstyleError{{Line: "7"}}},
		},
	}

	if err := writeCheckstyle(outPath, doc); err != nil {
		t.Fatalf("writeCheckstyle() error = %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.HasPrefix(string(data), xml.Header) {
		t.Fatalf("output missing XML header: %q", string(data))
	}

	var parsed checkstyle
	if err := xml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if parsed.XMLName.Local != "checkstyle" || parsed.Version != "5.0" {
		t.Fatalf("parsed root = %#v, want checkstyle version 5.0", parsed)
	}
}
