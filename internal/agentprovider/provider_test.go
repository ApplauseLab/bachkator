package agentprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/applauselab/bachkator/internal/model"
)

func TestBaseCommandForOpenCode(t *testing.T) {
	t.Parallel()

	got := BaseCommand(model.Provider{Name: "opencode", Type: "opencode"})
	want := []string{"opencode", "run"}
	assertStringSlice(t, got, want)
}

func TestRunnerSelectsProviderImplementations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		provider model.Provider
		wantOK   bool
		wantRun  bool
		wantBase []string
	}{
		{
			name:     "generic command",
			provider: model.Provider{Type: "agent", Command: []string{"custom-agent", "run"}},
			wantOK:   true,
			wantRun:  true,
			wantBase: []string{"custom-agent", "run"},
		},
		{
			name:     "generic empty command",
			provider: model.Provider{Type: "agent"},
			wantOK:   true,
			wantRun:  false,
			wantBase: nil,
		},
		{
			name:     "opencode",
			provider: model.Provider{Type: "opencode"},
			wantOK:   true,
			wantRun:  true,
			wantBase: []string{"opencode", "run"},
		},
		{
			name:     "unknown",
			provider: model.Provider{Type: "unknown"},
			wantOK:   false,
			wantRun:  false,
			wantBase: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, ok := runnerFor(tc.provider)
			if ok != tc.wantOK {
				t.Fatalf("runnerFor ok = %v, want %v", ok, tc.wantOK)
			}
			if got := Runnable(tc.provider); got != tc.wantRun {
				t.Fatalf("Runnable() = %v, want %v", got, tc.wantRun)
			}
			assertStringSlice(t, BaseCommand(tc.provider), tc.wantBase)
		})
	}
}

func TestConsumeOpenCodeEventCapturesSessionTextAndFinish(t *testing.T) {
	t.Parallel()

	summary := opencodeEventSummary{}
	var text bytes.Buffer
	lines := []string{
		`{"type":"step_start","sessionID":"ses_test"}`,
		`{"type":"text","sessionID":"ses_test","part":{"text":"hello\u001b[31m"}}`,
		`{"type":"step_finish","sessionID":"ses_test","part":{"reason":"stop","tokens":{"total":12},"cost":0}}`,
	}
	for _, line := range lines {
		if err := consumeOpenCodeEvent(line, &summary, &text); err != nil {
			t.Fatalf("consumeOpenCodeEvent() error = %v", err)
		}
	}
	if summary.SessionID != "ses_test" {
		t.Fatalf("session ID = %q, want ses_test", summary.SessionID)
	}
	if text.String() != "hello[31m\n" {
		t.Fatalf("text = %q, want sanitized hello", text.String())
	}
	if summary.FinishReason != "stop" || summary.Tokens["total"] != float64(12) {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestConsumeOpenCodeEventMirrorsToolNameWithoutArgs(t *testing.T) {
	t.Parallel()

	summary := opencodeEventSummary{}
	var text bytes.Buffer
	line := `{"type":"tool_use","sessionID":"ses_test","part":{"type":"tool","tool":"bash","state":{"status":"completed","title":"Runs demo command with $SECRET","metadata":{"description":"printf $SECRET"},"input":{"command":"printf $SECRET","timeout":120000},"output":"line 1\nline \u001b[31m2"}}}`
	if err := consumeOpenCodeEvent(line, &summary, &text); err != nil {
		t.Fatalf("consumeOpenCodeEvent() error = %v", err)
	}
	want := "[opencode tool:bash]\n"
	if text.String() != want {
		t.Fatalf("text = %q, want %q", text.String(), want)
	}
	for _, leaked := range []string{"Runs demo", "printf $SECRET", "timeout", "line 1"} {
		if strings.Contains(text.String(), leaked) {
			t.Fatalf("text includes private tool data %q: %q", leaked, text.String())
		}
	}
	wantBytes := len("[opencode tool:bash]\n")
	if summary.TextBytes != wantBytes {
		t.Fatalf("TextBytes = %d", summary.TextBytes)
	}
}

func TestConsumeOpenCodeEventCapsMirroredToolProgress(t *testing.T) {
	t.Parallel()

	summary := opencodeEventSummary{TextBytes: maxOpenCodeLogTextBytes - 2}
	var text bytes.Buffer
	line := `{"type":"tool_use","sessionID":"ses_test","part":{"type":"tool","tool":"bash","state":{"status":"completed","input":{"command":"abc"},"output":"this should not mirror"}}}`
	err := consumeOpenCodeEvent(line, &summary, &text)
	if err == nil || !strings.Contains(err.Error(), "opencode text output exceeded") {
		t.Fatalf("consumeOpenCodeEvent() error = %v, want text cap", err)
	}
	if summary.TextBytes != maxOpenCodeLogTextBytes {
		t.Fatalf("TextBytes = %d, want cap", summary.TextBytes)
	}
}

func TestCaptureOpenCodeEventsRejectsMalformedJSONL(t *testing.T) {
	t.Parallel()

	_, err := captureOpenCodeEvents(
		bytes.NewBufferString("not-json\n"),
		filepath.Join(t.TempDir(), "events.jsonl"),
		nil,
	)
	if err == nil {
		t.Fatal("captureOpenCodeEvents() error = nil, want malformed JSONL error")
	}
}

func TestCaptureOpenCodeEventsRejectsOversizedLineWhileDraining(t *testing.T) {
	t.Parallel()

	input := strings.Repeat("x", maxOpenCodeEventLineBytes+1) + "\n" +
		`{"type":"step_start","sessionID":"ses_after"}` + "\n"
	summary, err := captureOpenCodeEvents(
		bytes.NewBufferString(input),
		filepath.Join(t.TempDir(), "events.jsonl"),
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "opencode JSONL event exceeded") {
		t.Fatalf("captureOpenCodeEvents() error = %v, want oversized event", err)
	}
	if summary.SessionID != "ses_after" {
		t.Fatalf("session ID after oversized line = %q, want ses_after", summary.SessionID)
	}
}

func TestWriteOpenCodeSummaryOmitsAbsentTelemetry(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "provider-summary.json")
	if err := writeOpenCodeSummary(path, opencodeEventSummary{SessionID: "ses_test"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"finish_reason", "tokens", "cost"} {
		if _, ok := value[key]; ok {
			t.Fatalf("summary contains absent telemetry key %q: %s", key, data)
		}
	}
}

func TestValidateOpenCodeResumeChecksSessionEvidence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	if err := os.Mkdir(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, workspace, "init")
	runGit(t, workspace, "checkout", "-b", "bach/example")
	runGit(t, workspace, "config", "user.email", "test@example.com")
	runGit(t, workspace, "config", "user.name", "Test User")
	if err := os.WriteFile(
		filepath.Join(workspace, "file.txt"),
		[]byte("content\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	runGit(t, workspace, "add", "file.txt")
	runGit(t, workspace, "commit", "-m", "initial")
	sessionPath := filepath.Join(dir, "provider-session.json")
	eventsPath := filepath.Join(dir, "provider-events.raw.jsonl")
	feedbackPath := filepath.Join(dir, "feedback-bundle.json")
	for _, path := range []string{eventsPath, feedbackPath} {
		if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	session := map[string]any{"target": "agent/example", "workspace": workspace}
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sessionPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	previous := &PreviousAttempt{
		ProviderType:        "opencode",
		ProviderSessionID:   "ses_test",
		ProviderSessionPath: sessionPath,
		ProviderEventsPath:  eventsPath,
		FeedbackBundle:      feedbackPath,
	}
	if err := validateOpenCodeResume(
		context.Background(),
		"agent/example",
		workspace,
		"bach/example",
		previous,
	); err != nil {
		t.Fatalf("validateOpenCodeResume() error = %v", err)
	}
	if err := validateOpenCodeResume(
		context.Background(),
		"agent/other",
		workspace,
		"bach/example",
		previous,
	); err == nil {
		t.Fatal("validateOpenCodeResume() accepted wrong target")
	}
	if err := validateOpenCodeResume(
		context.Background(),
		"agent/example",
		workspace,
		"bach/other",
		previous,
	); err == nil {
		t.Fatal("validateOpenCodeResume() accepted wrong branch")
	}
}

func assertStringSlice(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("slice len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("slice[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
