package githubissuetrigger

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/applauselab/bachkator/pkg/triggerprotocol"
)

func TestProviderPollMapsGitHubIssues(t *testing.T) {
	t.Setenv("BACH_TEST_GITHUB_TOKEN", "secret-token")

	var firstRequest *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if firstRequest == nil {
			firstRequest = r.Clone(r.Context())
		}
		if r.URL.Query().Get("page") == "2" {
			_ = json.NewEncoder(w).Encode([]any{})
			return
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"number":   42,
				"html_url": "https://github.com/acme/widgets/issues/42",
				"title":    "Ship the widget",
				"body":     "Build it with tests.",
				"state":    "open",
				"labels": []map[string]string{
					{"name": "factory:ship"},
					{"name": "priority:high"},
				},
				"user":       map[string]string{"login": "octo"},
				"created_at": "2026-06-01T10:00:00Z",
				"updated_at": "2026-06-02T10:00:00Z",
			},
			{
				"number":     43,
				"html_url":   "https://github.com/acme/widgets/pull/43",
				"title":      "Ignore pull requests",
				"state":      "open",
				"labels":     []map[string]string{{"name": "factory:ship"}},
				"user":       map[string]string{"login": "octo"},
				"created_at": "2026-06-01T10:00:00Z",
				"updated_at": "2026-06-03T10:00:00Z",
				"pull_request": map[string]string{
					"url": "https://api.github.com/repos/acme/widgets/pulls/43",
				},
			},
		})
	}))
	defer server.Close()

	provider := New(server.Client())
	_, err := provider.Handshake(context.Background(), triggerprotocol.HandshakeParams{
		Protocol: triggerprotocol.ProtocolVersion,
		Factory:  "delivery",
		Trigger:  "github_issues",
		Config: map[string]string{
			"repo":      "acme/widgets",
			"api_url":   server.URL,
			"token_env": "BACH_TEST_GITHUB_TOKEN",
			"labels":    "factory:ship",
			"per_page":  "2",
		},
	})
	if err != nil {
		t.Fatalf("handshake error = %v", err)
	}

	result, err := provider.Poll(context.Background(), triggerprotocol.PollParams{
		Cursor: "2026-06-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("poll error = %v", err)
	}
	if firstRequest == nil {
		t.Fatal("server did not receive request")
	}
	if got := firstRequest.URL.Path; got != "/repos/acme/widgets/issues" {
		t.Fatalf("request path = %q", got)
	}
	query := firstRequest.URL.Query()
	if query.Get("state") != "open" || query.Get("sort") != "updated" ||
		query.Get("direction") != "asc" || query.Get("since") != "2026-06-01T00:00:00Z" ||
		query.Get("labels") != "factory:ship" {
		t.Fatalf("unexpected query = %s", firstRequest.URL.RawQuery)
	}
	if got := firstRequest.Header.Get("Authorization"); got != "Bearer secret-token" {
		t.Fatalf("authorization header = %q", got)
	}
	if result.Cursor != "2026-06-03T10:00:00Z" {
		t.Fatalf("cursor = %q", result.Cursor)
	}
	if len(result.Items) != 1 {
		t.Fatalf("items len = %d", len(result.Items))
	}
	item := result.Items[0]
	if item.Source.Type != "github_issue" || item.Source.ID != "acme/widgets#42" ||
		item.Source.URL != "https://github.com/acme/widgets/issues/42" ||
		item.Source.Revision != "2026-06-02T10:00:00Z" {
		t.Fatalf("source = %#v", item.Source)
	}
	if item.Title != "Ship the widget" || item.Body != "Build it with tests." {
		t.Fatalf("item text = %#v", item)
	}
	if item.Priority != "high" {
		t.Fatalf("priority = %q", item.Priority)
	}
	if item.Metadata["github_author"] != "octo" || item.Metadata["github_number"] != "42" {
		t.Fatalf("metadata = %#v", item.Metadata)
	}
}

func TestProviderHandshakeValidatesConfig(t *testing.T) {
	provider := New(nil)
	_, err := provider.Handshake(context.Background(), triggerprotocol.HandshakeParams{
		Protocol: triggerprotocol.ProtocolVersion,
	})
	if err == nil {
		t.Fatal("handshake error is nil")
	}
	if got := err.Error(); got != "validation_failed: repo is required" {
		t.Fatalf("handshake error = %q", got)
	}
}

func TestProviderPollSkipsIssuesAtCursor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{
				"number":     42,
				"html_url":   "https://github.com/acme/widgets/issues/42",
				"title":      "Already delivered",
				"state":      "open",
				"labels":     []map[string]string{},
				"user":       map[string]string{"login": "octo"},
				"created_at": "2026-06-01T10:00:00Z",
				"updated_at": "2026-06-02T10:00:00Z",
			},
		})
	}))
	defer server.Close()

	provider := New(server.Client())
	_, err := provider.Handshake(context.Background(), triggerprotocol.HandshakeParams{
		Protocol: triggerprotocol.ProtocolVersion,
		Config: map[string]string{
			"repo":    "acme/widgets",
			"api_url": server.URL,
		},
	})
	if err != nil {
		t.Fatalf("handshake error = %v", err)
	}

	result, err := provider.Poll(context.Background(), triggerprotocol.PollParams{
		Cursor: "2026-06-02T10:00:00Z",
	})
	if err != nil {
		t.Fatalf("poll error = %v", err)
	}
	if result.Cursor != "2026-06-02T10:00:00Z" {
		t.Fatalf("cursor = %q", result.Cursor)
	}
	if len(result.Items) != 0 {
		t.Fatalf("items len = %d", len(result.Items))
	}
}

func TestProviderPollReportsGitHubStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "bad credentials", http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := New(server.Client())
	_, err := provider.Handshake(context.Background(), triggerprotocol.HandshakeParams{
		Protocol: triggerprotocol.ProtocolVersion,
		Config: map[string]string{
			"repo":    "acme/widgets",
			"api_url": server.URL,
		},
	})
	if err != nil {
		t.Fatalf("handshake error = %v", err)
	}
	_, err = provider.Poll(context.Background(), triggerprotocol.PollParams{})
	if err == nil {
		t.Fatal("poll error is nil")
	}
	if got := err.Error(); got != "internal: github issues request failed with 401 Unauthorized: bad credentials" {
		t.Fatalf("poll error = %q", got)
	}
}

func TestProviderPollRedactsTokenFromGitHubStatus(t *testing.T) {
	t.Setenv("BACH_TEST_GITHUB_TOKEN", "secret-token")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "reflected secret-token", http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := New(server.Client())
	_, err := provider.Handshake(context.Background(), triggerprotocol.HandshakeParams{
		Protocol: triggerprotocol.ProtocolVersion,
		Config: map[string]string{
			"repo":      "acme/widgets",
			"api_url":   server.URL,
			"token_env": "BACH_TEST_GITHUB_TOKEN",
		},
	})
	if err != nil {
		t.Fatalf("handshake error = %v", err)
	}
	_, err = provider.Poll(context.Background(), triggerprotocol.PollParams{})
	if err == nil {
		t.Fatal("poll error is nil")
	}
	if got := err.Error(); got != "internal: github issues request failed with 401 Unauthorized: reflected [REDACTED]" {
		t.Fatalf("poll error = %q", got)
	}
}
