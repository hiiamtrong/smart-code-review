package reviewdog

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hiiamtrong/smart-code-review/internal/gateway"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

func makeResult(diags ...gateway.Diagnostic) *gateway.ReviewResult {
	return &gateway.ReviewResult{
		Diagnostics: diags,
		Source:      gateway.Source{Name: "ai-review"},
	}
}

func makeDiag(path string, line int, severity, message string) gateway.Diagnostic {
	return gateway.Diagnostic{
		Message:  message,
		Severity: severity,
		Location: gateway.Location{
			Path:  path,
			Range: gateway.Range{Start: gateway.Position{Line: line, Column: 1}, End: gateway.Position{Line: line, Column: 80}},
		},
		Code: gateway.Code{Value: "rule-001"},
	}
}

// ─── WriteRDJSON ──────────────────────────────────────────────────────────────

func TestWriteRDJSON_empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.jsonl")

	result := makeResult()
	if err := WriteRDJSON(result, path); err != nil {
		t.Fatalf("WriteRDJSON error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.TrimSpace(string(data))) != 0 {
		t.Errorf("expected empty file, got %q", string(data))
	}
}

func TestWriteRDJSON_singleDiagnostic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.jsonl")

	result := makeResult(makeDiag("src/main.go", 10, "ERROR", "null pointer dereference"))
	if err := WriteRDJSON(result, path); err != nil {
		t.Fatalf("WriteRDJSON error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	var rd rdDiagnostic
	if err := json.Unmarshal([]byte(lines[0]), &rd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rd.Message != "null pointer dereference" {
		t.Errorf("Message = %q", rd.Message)
	}
	if rd.Location.Path != "src/main.go" {
		t.Errorf("Path = %q", rd.Location.Path)
	}
	if rd.Location.Range.Start.Line != 10 {
		t.Errorf("Line = %d", rd.Location.Range.Start.Line)
	}
	if rd.Severity != "ERROR" {
		t.Errorf("Severity = %q", rd.Severity)
	}
	if rd.Source == nil || rd.Source.Name != "ai-review" {
		t.Errorf("Source.Name = %v", rd.Source)
	}
	if rd.Code == nil || rd.Code.Value != "rule-001" {
		t.Errorf("Code = %v", rd.Code)
	}
}

func TestWriteRDJSON_multipleDiagnostics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.jsonl")

	result := makeResult(
		makeDiag("a.go", 1, "ERROR", "error 1"),
		makeDiag("b.go", 2, "WARNING", "warning 2"),
		makeDiag("c.go", 3, "INFO", "info 3"),
	)
	if err := WriteRDJSON(result, path); err != nil {
		t.Fatalf("WriteRDJSON error: %v", err)
	}

	data, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
}

func TestWriteRDJSON_defaultSourceName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.jsonl")

	// Source name is empty — should default to "ai-review"
	result := &gateway.ReviewResult{
		Diagnostics: []gateway.Diagnostic{makeDiag("f.go", 1, "INFO", "msg")},
		Source:      gateway.Source{},
	}
	if err := WriteRDJSON(result, path); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(path)
	var rd rdDiagnostic
	json.Unmarshal([]byte(strings.TrimRight(string(data), "\n")), &rd)
	if rd.Source == nil || rd.Source.Name != "ai-review" {
		t.Errorf("default source name: got %v", rd.Source)
	}
}

func TestWriteRDJSON_createsDirectory(t *testing.T) {
	dir := t.TempDir()
	// Deep nested path that doesn't exist yet
	path := filepath.Join(dir, "sub", "dir", "out.jsonl")
	if err := WriteRDJSON(makeResult(), path); err != nil {
		t.Fatalf("WriteRDJSON should create parent dirs: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("output file not created: %v", err)
	}
}

// ─── WriteOverview ────────────────────────────────────────────────────────────

func TestWriteOverview_basic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overview.txt")
	result := &gateway.ReviewResult{Overview: "Found 2 issues: 1 error, 1 warning"}

	if err := WriteOverview(result, path); err != nil {
		t.Fatalf("WriteOverview error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != result.Overview {
		t.Errorf("overview = %q, want %q", string(data), result.Overview)
	}
}

func TestWriteOverview_empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overview.txt")
	result := &gateway.ReviewResult{Overview: ""}

	if err := WriteOverview(result, path); err != nil {
		t.Fatalf("WriteOverview error: %v", err)
	}
	data, _ := os.ReadFile(path)
	if len(data) != 0 {
		t.Errorf("expected empty file")
	}
}

// ─── DeleteExistingOverviewComments ──────────────────────────────────────────

func TestDeleteExistingOverviewComments_deletesMatchingComments(t *testing.T) {
	deletedIDs := []int64{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/comments"):
			// Return two comments: one with sentinel, one without
			comments := []map[string]interface{}{
				{"id": 101, "body": overviewSentinel + "\n\nsome overview"},
				{"id": 102, "body": "regular comment"},
			}
			json.NewEncoder(w).Encode(comments)
		case r.Method == http.MethodDelete:
			// Track deleted IDs
			var id int64
			fmt.Sscanf(r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:], "%d", &id)
			deletedIDs = append(deletedIDs, id)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	// Override the URL by using the test server directly.
	// We need to temporarily monkey-patch the URL — instead, let's just call the
	// internal function by checking the HTTP calls made through a transport.
	// Since functions use http.DefaultClient we use a custom RoundTripper.
	origTransport := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = origTransport }()
	http.DefaultClient.Transport = &urlRewriteTransport{base: srv.URL}

	if err := DeleteExistingOverviewComments("tok", "owner/repo", "42"); err != nil {
		t.Fatalf("error: %v", err)
	}
	// Only comment 101 (with sentinel) should be deleted.
	if len(deletedIDs) != 1 || deletedIDs[0] != 101 {
		t.Errorf("deleted IDs = %v, want [101]", deletedIDs)
	}
}

func TestDeleteExistingOverviewComments_noMatchingComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		comments := []map[string]interface{}{
			{"id": 5, "body": "a normal comment"},
		}
		json.NewEncoder(w).Encode(comments)
	}))
	defer srv.Close()

	origTransport := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = origTransport }()
	http.DefaultClient.Transport = &urlRewriteTransport{base: srv.URL}

	if err := DeleteExistingOverviewComments("tok", "owner/repo", "1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ─── PostOverviewComment ──────────────────────────────────────────────────────

func TestPostOverviewComment_success(t *testing.T) {
	var postedBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			// list comments — return empty
			json.NewEncoder(w).Encode([]interface{}{})
		case r.Method == http.MethodPost:
			var payload map[string]string
			json.NewDecoder(r.Body).Decode(&payload)
			postedBody = payload["body"]
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 999})
		}
	}))
	defer srv.Close()

	origTransport := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = origTransport }()
	http.DefaultClient.Transport = &urlRewriteTransport{base: srv.URL}

	if err := PostOverviewComment("tok", "owner/repo", "7", "2 errors found"); err != nil {
		t.Fatalf("PostOverviewComment error: %v", err)
	}
	if !strings.Contains(postedBody, overviewSentinel) {
		t.Errorf("posted body missing sentinel: %q", postedBody)
	}
	if !strings.Contains(postedBody, "2 errors found") {
		t.Errorf("posted body missing overview text: %q", postedBody)
	}
}

func TestPostOverviewComment_httpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]interface{}{})
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	origTransport := http.DefaultClient.Transport
	defer func() { http.DefaultClient.Transport = origTransport }()
	http.DefaultClient.Transport = &urlRewriteTransport{base: srv.URL}

	err := PostOverviewComment("bad-tok", "owner/repo", "1", "overview")
	if err == nil {
		t.Error("expected error on HTTP 401")
	}
}

// ─── InvokeReviewdog ─────────────────────────────────────────────────────────

func TestInvokeReviewdog_BinaryNotFound(t *testing.T) {
	// Pass a non-existent input file path — InvokeReviewdog handles os.Open
	// failure by leaving cmd.Stdin as nil, which avoids holding a file handle
	// (important on Windows where open handles block TempDir cleanup).
	inputFile := "/nonexistent/rdjson-input.json"

	// Point HOME to a temp dir that has no ~/bin/reviewdog, and ensure
	// "reviewdog" is not on PATH either, so the exec fails with "not found".
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir()) // empty dir, no reviewdog binary

	err := InvokeReviewdog(inputFile, "local")
	if err == nil {
		t.Error("expected error when reviewdog binary is not found")
	}
}

// ─── urlRewriteTransport ─────────────────────────────────────────────────────

// urlRewriteTransport redirects api.github.com calls to a test server.
type urlRewriteTransport struct {
	base string
}

func (t *urlRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = strings.TrimPrefix(t.base, "http://")
	return http.DefaultTransport.RoundTrip(req2)
}
