package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hiiamtrong/smart-code-review/internal/config"
	"github.com/hiiamtrong/smart-code-review/internal/git"
)

// ─── parseSSEStream unit tests ─────────────────────────────────────────────

func TestParseSSEStream_ProgressIgnored(t *testing.T) {
	stream := "event: progress\ndata: {\"chunk\":1,\"total\":5}\n\n"
	result, err := parseSSEStream(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("want 0 diagnostics, got %d", len(result.Diagnostics))
	}
}

func TestParseSSEStream_TextAccumulates(t *testing.T) {
	stream := "" +
		"event: text\ndata: {\"content\":\"Hello \"}\n\n" +
		"event: text\ndata: {\"content\":\"World\"}\n\n"

	result, err := parseSSEStream(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Overview != "Hello World" {
		t.Errorf("overview: got %q, want %q", result.Overview, "Hello World")
	}
}

func TestParseSSEStream_DiagnosticCallsCallback(t *testing.T) {
	diagJSON := `{"message":"use err != nil","location":{"path":"main.go","range":{"start":{"line":10,"column":1},"end":{"line":10,"column":20}}},"severity":"WARNING"}`
	stream := fmt.Sprintf("event: diagnostic\ndata: %s\n\n", diagJSON)

	var received []Diagnostic
	result, err := parseSSEStream(strings.NewReader(stream), func(d Diagnostic) {
		received = append(received, d)
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("callback called %d times, want 1", len(received))
	}
	if received[0].Message != "use err != nil" {
		t.Errorf("message: got %q", received[0].Message)
	}
	if len(result.Diagnostics) != 1 {
		t.Errorf("result diagnostics: got %d, want 1", len(result.Diagnostics))
	}
	if result.Diagnostics[0].Severity != "WARNING" {
		t.Errorf("severity: got %q, want WARNING", result.Diagnostics[0].Severity)
	}
}

func TestParseSSEStream_CompleteOverridesTextOverview(t *testing.T) {
	completeJSON := `{"overview":"Final overview","max_severity":"ERROR","source":{"name":"ai-review","url":"https://example.com"}}`
	stream := "" +
		"event: text\ndata: {\"content\":\"partial \"}\n\n" +
		fmt.Sprintf("event: complete\ndata: %s\n\n", completeJSON)

	result, err := parseSSEStream(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Overview != "Final overview" {
		t.Errorf("overview: got %q, want %q", result.Overview, "Final overview")
	}
	if result.MaxSeverity != "ERROR" {
		t.Errorf("max_severity: got %q, want ERROR", result.MaxSeverity)
	}
	if result.Source.Name != "ai-review" {
		t.Errorf("source.name: got %q, want ai-review", result.Source.Name)
	}
}

func TestParseSSEStream_ErrorReturnsError(t *testing.T) {
	stream := "event: error\ndata: {\"message\":\"model overloaded\"}\n\n"
	_, err := parseSSEStream(strings.NewReader(stream), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "model overloaded") {
		t.Errorf("error message: got %q, want to contain 'model overloaded'", err.Error())
	}
}

func TestParseSSEStream_ErrorPlainText(t *testing.T) {
	// Error data that isn't valid JSON should still propagate the raw text.
	stream := "event: error\ndata: internal server error\n\n"
	_, err := parseSSEStream(strings.NewReader(stream), nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "internal server error") {
		t.Errorf("error message: got %q", err.Error())
	}
}

func TestParseSSEStream_FullStream(t *testing.T) {
	diagJSON := `{"message":"nil pointer","location":{"path":"foo.go","range":{"start":{"line":5,"column":1},"end":{"line":5,"column":10}}},"severity":"ERROR"}`
	completeJSON := `{"overview":"One error found","max_severity":"ERROR"}`

	stream := "" +
		"event: progress\ndata: {\"chunk\":1,\"total\":3}\n\n" +
		fmt.Sprintf("event: diagnostic\ndata: %s\n\n", diagJSON) +
		"event: text\ndata: {\"content\":\"Reviewing...\"}\n\n" +
		fmt.Sprintf("event: complete\ndata: %s\n\n", completeJSON)

	var callbackCount int
	result, err := parseSSEStream(strings.NewReader(stream), func(d Diagnostic) {
		callbackCount++
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if callbackCount != 1 {
		t.Errorf("callback count: got %d, want 1", callbackCount)
	}
	if len(result.Diagnostics) != 1 {
		t.Errorf("diagnostics: got %d, want 1", len(result.Diagnostics))
	}
	if result.Diagnostics[0].Severity != "ERROR" {
		t.Errorf("severity: got %q, want ERROR", result.Diagnostics[0].Severity)
	}
	if result.Overview != "One error found" {
		t.Errorf("overview: got %q, want 'One error found'", result.Overview)
	}
	if result.MaxSeverity != "ERROR" {
		t.Errorf("max_severity: got %q, want ERROR", result.MaxSeverity)
	}
}

func TestParseSSEStream_NoTrailingNewline(t *testing.T) {
	// Trailing event without a final blank line should still be flushed.
	completeJSON := `{"overview":"done","max_severity":"INFO"}`
	stream := fmt.Sprintf("event: complete\ndata: %s", completeJSON) // no trailing \n\n

	result, err := parseSSEStream(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Overview != "done" {
		t.Errorf("overview: got %q, want 'done'", result.Overview)
	}
}

func TestParseSSEStream_UnknownEventIgnored(t *testing.T) {
	stream := "event: unknown_future_type\ndata: {\"foo\":\"bar\"}\n\n"
	result, err := parseSSEStream(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("want 0 diagnostics, got %d", len(result.Diagnostics))
	}
}

func TestParseSSEStream_NilCallbackOK(t *testing.T) {
	diagJSON := `{"message":"test","location":{"path":"f.go","range":{"start":{"line":1,"column":1},"end":{"line":1,"column":5}}},"severity":"INFO"}`
	stream := fmt.Sprintf("event: diagnostic\ndata: %s\n\n", diagJSON)

	// Must not panic with nil callback.
	result, err := parseSSEStream(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Diagnostics) != 1 {
		t.Errorf("diagnostics: got %d, want 1", len(result.Diagnostics))
	}
}

func TestParseSSEStream_MultipleLines(t *testing.T) {
	// Some SSE implementations split data across multiple data: lines.
	stream := "" +
		"event: diagnostic\n" +
		"data: {\"message\":\"long msg\",\"location\":{\"path\":\"x.go\",\"range\":{\"start\":{\"line\":1,\"column\":1},\"end\":{\"line\":1,\"column\":5}}}," +
		"\"severity\":\"INFO\"}\n\n"

	result, err := parseSSEStream(strings.NewReader(stream), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Diagnostics) != 1 {
		t.Errorf("diagnostics: got %d, want 1", len(result.Diagnostics))
	}
}

// ─── HTTP integration tests ────────────────────────────────────────────────

func testPayload() ReviewPayload {
	return ReviewPayload{
		Diff:       "diff --git a/main.go b/main.go\n+fmt.Println(\"hello\")\n",
		Language:   "go",
		AIModel:    "gemini-2.0-flash",
		AIProvider: "google",
		GitInfo: git.GitInfo{
			CommitHash: "abc123",
			BranchName: "main",
			RepoURL:    "https://github.com/example/repo",
		},
	}
}

func testConfig(serverURL string) *config.Config {
	cfg := config.Defaults()
	cfg.AIGatewayURL = serverURL
	cfg.AIGatewayAPIKey = "test-api-key"
	cfg.GatewayTimeoutSec = 10
	return cfg
}

func TestSyncReview_Success(t *testing.T) {
	want := ReviewResult{
		Source:      Source{Name: "ai-review"},
		MaxSeverity: "WARNING",
		Diagnostics: []Diagnostic{
			{
				Message:  "test issue",
				Severity: "WARNING",
				Location: Location{Path: "main.go", Range: Range{
					Start: Position{Line: 1, Column: 1},
					End:   Position{Line: 1, Column: 10},
				}},
			},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %q, want POST", r.Method)
		}
		if r.Header.Get("X-API-Key") != "test-api-key" {
			t.Errorf("X-API-Key header missing or wrong")
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(want); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	result, err := SyncReview(t.Context(), testConfig(srv.URL), testPayload())
	if err != nil {
		t.Fatalf("SyncReview: %v", err)
	}
	if result.MaxSeverity != "WARNING" {
		t.Errorf("max_severity: got %q, want WARNING", result.MaxSeverity)
	}
	if len(result.Diagnostics) != 1 {
		t.Errorf("diagnostics: got %d, want 1", len(result.Diagnostics))
	}
}

func TestSyncReview_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := SyncReview(t.Context(), testConfig(srv.URL), testPayload())
	if err == nil {
		t.Fatal("expected error for HTTP 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error: got %q, want to contain '400'", err.Error())
	}
}

func TestStreamingReview_Success(t *testing.T) {
	diagJSON := `{"message":"found issue","location":{"path":"main.go","range":{"start":{"line":3,"column":1},"end":{"line":3,"column":5}}},"severity":"WARNING"}`
	completeJSON := `{"overview":"One warning","max_severity":"WARNING","source":{"name":"ai-review"}}`

	sseBody := "" +
		"event: progress\ndata: {\"chunk\":1}\n\n" +
		fmt.Sprintf("event: diagnostic\ndata: %s\n\n", diagJSON) +
		fmt.Sprintf("event: complete\ndata: %s\n\n", completeJSON)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		fmt.Fprint(w, sseBody)
	}))
	defer srv.Close()

	var received []Diagnostic
	result, err := StreamingReview(t.Context(), testConfig(srv.URL), testPayload(), func(d Diagnostic) {
		received = append(received, d)
	})
	if err != nil {
		t.Fatalf("StreamingReview: %v", err)
	}
	if len(received) != 1 {
		t.Errorf("streaming callback count: got %d, want 1", len(received))
	}
	if result.Overview != "One warning" {
		t.Errorf("overview: got %q, want 'One warning'", result.Overview)
	}
	if result.MaxSeverity != "WARNING" {
		t.Errorf("max_severity: got %q, want WARNING", result.MaxSeverity)
	}
}

func TestStreamingReview_FallsBackToSync(t *testing.T) {
	// Server returns SSE that is unparseable (error event), then SyncReview
	// would re-POST. We serve a valid JSON response on both calls so the
	// second call (sync fallback) succeeds.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First call: respond with SSE containing an error event.
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "event: error\ndata: {\"message\":\"oops\"}\n\n")
			return
		}
		// Second call (sync fallback): return valid JSON.
		result := ReviewResult{MaxSeverity: "INFO", Source: Source{Name: "ai-review"}}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(result); err != nil {
			t.Errorf("encode: %v", err)
		}
	}))
	defer srv.Close()

	result, err := StreamingReview(t.Context(), testConfig(srv.URL), testPayload(), nil)
	if err != nil {
		t.Fatalf("StreamingReview (with fallback): %v", err)
	}
	if callCount != 2 {
		t.Errorf("server call count: got %d, want 2 (streaming + sync fallback)", callCount)
	}
	if result.MaxSeverity != "INFO" {
		t.Errorf("max_severity from sync fallback: got %q, want INFO", result.MaxSeverity)
	}
}

func TestStreamingReview_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := StreamingReview(t.Context(), testConfig(srv.URL), testPayload(), nil)
	if err == nil {
		t.Fatal("expected error for HTTP 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error: got %q, want to contain '401'", err.Error())
	}
}
