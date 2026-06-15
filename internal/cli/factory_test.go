package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestFactoryCommandManualQueueLifecycle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

factory "sldc" {
  workflow "ship" {}

  triggers {
    manual {}
  }
}
`)
	configPath := filepath.Join(dir, "Bachfile")
	var submitOut bytes.Buffer
	if err := Execute(context.Background(), []string{
		"-f", configPath,
		"factory", "submit", "sldc",
		"--title", "Ship billing webhook",
		"--body", "Implement it",
		"--label", "billing",
		"--dedupe-key", "billing-webhook",
		"--json",
	}, &submitOut, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}
	var submitted factorySubmitView
	if err := json.Unmarshal(submitOut.Bytes(), &submitted); err != nil {
		t.Fatalf("submit JSON invalid: %v\n%s", err, submitOut.String())
	}
	if !submitted.Created || submitted.Item.ID == "" || submitted.Item.IntakeEvidenceURI == "" {
		t.Fatalf("submitted = %#v", submitted)
	}
	workItemID := submitted.Item.ID

	var duplicateOut bytes.Buffer
	if err := Execute(context.Background(), []string{
		"-f", configPath,
		"factory", "submit", "sldc",
		"--title", "Ship billing webhook again",
		"--body", "Implement it again",
		"--dedupe-key", "billing-webhook",
		"--json",
	}, &duplicateOut, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}
	var duplicate factorySubmitView
	if err := json.Unmarshal(duplicateOut.Bytes(), &duplicate); err != nil {
		t.Fatalf("duplicate JSON invalid: %v\n%s", err, duplicateOut.String())
	}
	if duplicate.Created || duplicate.Item.ID != workItemID || len(duplicate.Item.Events) != 2 {
		t.Fatalf("duplicate = %#v", duplicate)
	}

	var listOut bytes.Buffer
	if err := Execute(context.Background(), []string{
		"-f", configPath,
		"factory", "list", "sldc",
		"--json",
	}, &listOut, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}
	var list factoryListView
	if err := json.Unmarshal(listOut.Bytes(), &list); err != nil {
		t.Fatalf("list JSON invalid: %v\n%s", err, listOut.String())
	}
	if len(list.Items) != 1 || list.Items[0].ID != workItemID {
		t.Fatalf("list = %#v", list)
	}

	var inspectOut bytes.Buffer
	if err := Execute(context.Background(), []string{
		"-f", configPath,
		"factory", "inspect", "sldc", workItemID,
		"--json",
	}, &inspectOut, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(inspectOut.String(), workItemID) ||
		!strings.Contains(inspectOut.String(), "Ship billing webhook") {
		t.Fatalf("inspect output = %s", inspectOut.String())
	}

	var cancelOut bytes.Buffer
	if err := Execute(context.Background(), []string{
		"-f", configPath,
		"factory", "cancel", "sldc", workItemID,
		"--reason", "duplicate",
		"--json",
	}, &cancelOut, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}
	var cancelled factoryWorkItemView
	if err := json.Unmarshal(cancelOut.Bytes(), &cancelled); err != nil {
		t.Fatalf("cancel JSON invalid: %v\n%s", err, cancelOut.String())
	}
	if cancelled.Lifecycle != "cancelled" || cancelled.CancelReason != "duplicate" {
		t.Fatalf("cancelled = %#v", cancelled)
	}
}

func TestFactoryStatusCommandJSON(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Bachfile"), `project "example" {}

factory "sldc" {
  workflow "ship" {}
}
`)
	var out bytes.Buffer
	if err := Execute(context.Background(), []string{
		"-f", filepath.Join(dir, "Bachfile"),
		"factory", "status", "sldc",
		"--json",
	}, &out, &bytes.Buffer{}, "test"); err != nil {
		t.Fatal(err)
	}
	var status factoryStatusView
	if err := json.Unmarshal(out.Bytes(), &status); err != nil {
		t.Fatalf("status JSON invalid: %v\n%s", err, out.String())
	}
	if status.HasActiveItem || status.LifecycleCounts == nil {
		t.Fatalf("status = %#v", status)
	}
}
