package factory

import (
	"context"
	"strings"
	"testing"
	"time"
)

func rejectionFixture(t *testing.T, rejectionCount string) (*Service, *captureQueue) {
	t.Helper()
	queue := &captureQueue{
		item: WorkItem{
			ID:           "waiting-item",
			Factory:      "sldc",
			Workflow:     "ship",
			Lifecycle:    "waiting_approval",
			CurrentPhase: "plan",
			Title:        "Ship billing webhook",
			Body:         "Implement it",
			SourceType:   "github_issue",
			SourceID:     "sldc/repo#7",
			Labels:       []string{"billing"},
			Metadata:     map[string]string{"rejection_count": rejectionCount},
			Attempts:     []WorkItemAttempt{{ID: "attempt-1"}},
		},
	}
	ids := []string{"retry", "intake-evidence", "intake-attempt", "intake-event", "intake-dedupe"}
	svc := &Service{
		Root:  t.TempDir(),
		Queue: queue,
		NewID: func() (string, error) {
			id := ids[0]
			ids = ids[1:]
			if len(ids) == 0 {
				ids = append(ids, "extra")
			}
			return id, nil
		},
		Now: func() time.Time { return time.Date(2026, 6, 14, 12, 0, 0, 0, time.UTC) },
	}
	return svc, queue
}

func TestRejectReenqueuesRetryWithFeedback(t *testing.T) {
	svc, queue := rejectionFixture(t, "0")

	result, err := svc.Reject(context.Background(), RejectOptions{
		Factory: "sldc",
		ID:      "waiting-item",
		Phase:   "plan",
		Reason:  "use dark theme and validate titles",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Rejected || result.Exhausted {
		t.Fatalf("result = %#v", result)
	}
	if result.RetryID != "retry" || result.RetryCount != 1 {
		t.Fatalf("retry = %q count = %d", result.RetryID, result.RetryCount)
	}
	if queue.failCalls != 1 {
		t.Fatalf("fail calls = %d", queue.failCalls)
	}
	if !strings.Contains(queue.item.Body, "Reviewer feedback (rejection 1)") ||
		!strings.Contains(queue.item.Body, "dark theme") {
		t.Fatalf("retry body missing feedback: %q", queue.item.Body)
	}
	if queue.item.Metadata["rejection_count"] != "1" {
		t.Fatalf("rejection_count = %q", queue.item.Metadata["rejection_count"])
	}
}

func TestRejectFailsItemWhenRejectionsExhausted(t *testing.T) {
	svc, queue := rejectionFixture(t, "3")

	result, err := svc.Reject(context.Background(), RejectOptions{
		Factory: "sldc",
		ID:      "waiting-item",
		Phase:   "plan",
		Reason:  "still wrong",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Exhausted {
		t.Fatalf("expected exhaustion: %#v", result)
	}
	if queue.failCalls != 1 || !strings.Contains(queue.failMessage, "rejections exhausted") {
		t.Fatalf("fail calls = %d message = %q", queue.failCalls, queue.failMessage)
	}
}

func TestRejectRequiresReason(t *testing.T) {
	svc, _ := rejectionFixture(t, "0")
	if _, err := svc.Reject(context.Background(), RejectOptions{
		Factory: "sldc",
		ID:      "waiting-item",
		Phase:   "plan",
	}); err == nil || !strings.Contains(err.Error(), "reason is required") {
		t.Fatalf("err = %v, want missing-reason validation", err)
	}
}
