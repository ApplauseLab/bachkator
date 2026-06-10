package docs

import (
	"strings"
	"testing"
)

func TestHeadingsIncludesReferenceSections(t *testing.T) {
	headings, err := Headings()
	if err != nil {
		t.Fatal(err)
	}
	for _, heading := range headings {
		if strings.Contains(heading.Title, "vim:") {
			t.Fatalf("modeline parsed as heading: %#v", heading)
		}
	}
	for _, heading := range headings {
		if heading.Title == "Project" {
			return
		}
	}
	t.Fatalf("Project heading not found in %#v", headings)
}

func TestFormatHeadingsGroupsByDocument(t *testing.T) {
	formatted := FormatHeadings([]Heading{
		{File: "agents.md", Level: 1, Title: "Agent Guide"},
		{File: "agents.md", Level: 2, Title: "First Move"},
		{File: "reference.md", Level: 1, Title: "Bach Reference"},
	})
	if strings.Contains(formatted, "First Move (agents.md)") {
		t.Fatalf("formatted headings should not repeat filenames per heading: %q", formatted)
	}
	if !strings.Contains(formatted, "agents.md\n  agent-guide\n    first-move") {
		t.Fatalf("formatted headings not grouped by document: %q", formatted)
	}
}

func TestSlugFormatsReferenceHeadings(t *testing.T) {
	if got := slug("Shell Targets"); got != "shell-targets" {
		t.Fatalf("slug = %q, want shell-targets", got)
	}
}

func TestSearchReturnsExactHeadingSection(t *testing.T) {
	sections, err := Search("project")
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 {
		t.Fatalf("section count = %d, want 1", len(sections))
	}
	if sections[0].Heading.Title != "Project" {
		t.Fatalf("heading = %q, want Project", sections[0].Heading.Title)
	}
	if !strings.Contains(sections[0].Body, "project \"example\"") {
		t.Fatalf("section body does not include project example: %q", sections[0].Body)
	}
}

func TestSearchPrioritizesHeadingSlug(t *testing.T) {
	sections, err := Search("computed-variables")
	if err != nil {
		t.Fatal(err)
	}
	if len(sections) != 1 {
		t.Fatalf("section count = %d, want 1", len(sections))
	}
	if sections[0].Heading.Title != "Computed Variables" {
		t.Fatalf("heading = %q, want Computed Variables", sections[0].Heading.Title)
	}
}

func TestFormatHeadingsIncludesEmbeddedAssets(t *testing.T) {
	formatted := FormatHeadings([]Heading{
		{File: "reference.md", Level: 1, Title: "Bach Reference"},
	})
	if !strings.Contains(formatted, "Embedded assets:\n  quality-plugin-report-schema") {
		t.Fatalf("formatted headings do not include embedded schema asset: %q", formatted)
	}
}

func TestSearchAssetsReturnsEmbeddedSchema(t *testing.T) {
	assets, err := SearchAssets("quality-plugin-report-schema")
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 1 {
		t.Fatalf("asset count = %d, want 1", len(assets))
	}
	if assets[0].File != "schemas/quality-plugin-report.schema.json" {
		t.Fatalf("asset file = %q, want quality plugin schema", assets[0].File)
	}
	if !strings.Contains(assets[0].Body, `"Bachkator Quality Plugin Report"`) {
		t.Fatalf("asset body does not include schema title: %q", assets[0].Body)
	}
}
