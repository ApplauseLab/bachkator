package plan

import "testing"

func TestParseInfersIDAndTitleWithoutFrontmatter(t *testing.T) {
	doc, diagnostics := Parse(
		"plans/Phase 4 Plan.md",
		[]byte("# Phase 4 - Plan Foundation\n\nBody\n"),
	)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if doc.ID != "plans-phase-4-plan" || doc.Title != "Phase 4 - Plan Foundation" {
		t.Fatalf("doc = %#v", doc)
	}
	if doc.Hash == "" {
		t.Fatal("hash is empty")
	}
}

func TestParseAppliesOptionalFrontmatter(t *testing.T) {
	doc, diagnostics := Parse("plans/default.md", []byte(`---
schema: bach.plan.v1
id: phase-4-plan-foundation
title: Plan Foundation
depends_on: [phase-3-factory]
agent_template: feature_implementer
policy: standard_feature
required_targets: [shell/test]
labels: [factory]
metadata:
  owner: platform
---

# Ignored title override
`))
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
	if doc.ID != "phase-4-plan-foundation" || doc.Title != "Plan Foundation" ||
		len(doc.DependsOn) != 1 || doc.Metadata["owner"] != "platform" {
		t.Fatalf("doc = %#v", doc)
	}
}

func TestParseRejectsUnknownAndWorkstreams(t *testing.T) {
	_, diagnostics := Parse("plans/bad.md", []byte(`---
unknown: true
workstreams: []
---

# Bad
`))
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v", diagnostics)
	}
}

func TestBuildSelectionDetectsDuplicatesMissingDepsAndCycles(t *testing.T) {
	a := Document{Path: "a.md", ID: "a", Title: "A"}
	c := Document{Path: "c.md", ID: "a", Title: "C", DependsOn: []string{"missing"}}
	selection := BuildSelection([]Document{a, c})
	if len(selection.Diagnostics) != 2 {
		t.Fatalf("diagnostics = %#v", selection.Diagnostics)
	}

	a = Document{Path: "a.md", ID: "a", Title: "A", DependsOn: []string{"b"}}
	b := Document{Path: "b.md", ID: "b", Title: "B", DependsOn: []string{"a"}}
	selection = BuildSelection([]Document{a, b})
	if len(selection.Diagnostics) != 2 {
		t.Fatalf("cycle diagnostics = %#v", selection.Diagnostics)
	}
}

func TestDeriveStatuses(t *testing.T) {
	a := Document{Path: "a.md", ID: "a", Title: "A", Hash: "sha256:a"}
	b := Document{Path: "b.md", ID: "b", Title: "B", DependsOn: []string{"a"}, Hash: "sha256:b"}
	selection := BuildSelection([]Document{a, b})
	records := DeriveStatuses(selection, map[string]Ledger{
		"a": {
			SchemaVersion: LedgerSchemaVersion,
			LedgerID:      "ledger-a",
			PlanID:        "a",
			Status:        StatusImplemented,
			Hash:          "sha256:a",
			RecordedAt:    testTime(),
		},
		"b": {
			SchemaVersion: LedgerSchemaVersion,
			LedgerID:      "ledger-b",
			PlanID:        "b",
			Status:        StatusImplemented,
			Hash:          "sha256:old",
			RecordedAt:    testTime(),
		},
	})
	if len(records) != 2 || records[0].Status != StatusImplemented ||
		records[1].Status != StatusStale {
		t.Fatalf("records = %#v", records)
	}
}
