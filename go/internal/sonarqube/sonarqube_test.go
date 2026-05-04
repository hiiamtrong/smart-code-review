package sonarqube

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/hiiamtrong/smart-code-review/internal/gateway"
)

const (
	propsFile          = "sonar-project.properties"
	scannerDir         = ".scannerwork"
	errFmtString       = "error: %v"
	testFileFooGo      = "src/foo.go"
	testFileBarGo      = "src/bar.go"
	testFileMainGo     = "main.go"
	testProjectKey     = "my-project"
	testSonarURL       = "http://sonar.example.com"
	reportTaskFile     = "report-task.txt"
	errExpect1Diag     = "expected 1 diagnostic, got %d"
	headerContentType  = "Content-Type"
	mimeJSON           = "application/json"
	apiIssuesSearch    = "/api/issues/search"
	errFetchResults    = "FetchResults error: %v"
)

// ─── ParseStagedLineRanges ────────────────────────────────────────────────────

func TestParseStagedLineRanges_basic(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/src/foo.go b/src/foo.go",
		"@@ -10,3 +10,5 @@",
	}, "\n")

	ranges := ParseStagedLineRanges(diff)
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	r := ranges[0]
	if r.File != testFileFooGo {
		t.Errorf("File = %q, want %q", r.File, testFileFooGo)
	}
	if r.Start != 10 || r.End != 14 {
		t.Errorf("range = [%d,%d], want [10,14]", r.Start, r.End)
	}
}

func TestParseStagedLineRanges_deletionHunk(t *testing.T) {
	// count=0 means deletion-only; should be skipped
	diff := strings.Join([]string{
		"diff --git a/foo.go b/foo.go",
		"@@ -5,3 +5,0 @@",
	}, "\n")
	ranges := ParseStagedLineRanges(diff)
	if len(ranges) != 0 {
		t.Errorf("expected 0 ranges for deletion-only hunk, got %d", len(ranges))
	}
}

func TestParseStagedLineRanges_noCountField(t *testing.T) {
	// @@ -1 +1 @@ — no count means count=1
	diff := strings.Join([]string{
		"diff --git a/bar.py b/bar.py",
		"@@ -1 +1 @@",
	}, "\n")
	ranges := ParseStagedLineRanges(diff)
	if len(ranges) != 1 {
		t.Fatalf("expected 1 range, got %d", len(ranges))
	}
	if ranges[0].Start != 1 || ranges[0].End != 1 {
		t.Errorf("range = [%d,%d], want [1,1]", ranges[0].Start, ranges[0].End)
	}
}

func TestParseStagedLineRanges_multipleFiles(t *testing.T) {
	diff := strings.Join([]string{
		"diff --git a/a.go b/a.go",
		"@@ -1,2 +1,2 @@",
		"diff --git a/b.go b/b.go",
		"@@ -5,3 +5,3 @@",
	}, "\n")
	ranges := ParseStagedLineRanges(diff)
	if len(ranges) != 2 {
		t.Fatalf("expected 2 ranges, got %d", len(ranges))
	}
	if ranges[0].File != "a.go" || ranges[1].File != "b.go" {
		t.Errorf("files = %q, %q", ranges[0].File, ranges[1].File)
	}
}

func TestParseStagedLineRanges_empty(t *testing.T) {
	ranges := ParseStagedLineRanges("")
	if len(ranges) != 0 {
		t.Errorf("expected 0 ranges on empty diff")
	}
}

// ─── dedupedDirs ──────────────────────────────────────────────────────────────

func TestDedupedDirs_basic(t *testing.T) {
	files := []string{"src/a.go", "src/b.go", "cmd/main.go"}
	result := dedupedDirs(files)
	dirs := strings.Split(result, ",")
	if len(dirs) != 2 {
		t.Fatalf("expected 2 dirs, got %d: %v", len(dirs), dirs)
	}
}

func TestDedupedDirs_removesSubdir(t *testing.T) {
	// src is parent of src/utils — only src should remain
	files := []string{testFileFooGo, "src/utils/bar.go"}
	result := dedupedDirs(files)
	if strings.Contains(result, "src/utils") {
		t.Errorf("dedupedDirs should drop subdirectory: got %q", result)
	}
	if !strings.Contains(result, "src") {
		t.Errorf("dedupedDirs should keep parent: got %q", result)
	}
}

func TestDedupedDirs_rootFiles(t *testing.T) {
	files := []string{testFileMainGo, "go.mod"}
	result := dedupedDirs(files)
	if result != "." {
		t.Errorf("expected \".\", got %q", result)
	}
}

func TestDedupedDirs_rootFileMixedWithSubdirs(t *testing.T) {
	// Root-level files (dir=".") should be skipped to avoid sonar.sources="."
	// which causes SonarQube to traverse node_modules, vendor, etc.
	// Root files are already covered by sonar.inclusions.
	files := []string{"package.json", "docs/API_DOCUMENTATION.md", "src/lib/api.ts"}
	result := dedupedDirs(files)
	// dedupedDirs emits forward slashes for Sonar (see sonarSlashDir).
	srcLib := filepath.ToSlash(filepath.Join("src", "lib"))
	if !strings.Contains(result, "docs") || !strings.Contains(result, srcLib) {
		t.Errorf("expected docs and %s, got %q", srcLib, result)
	}
}

// ─── sanitizeKey ──────────────────────────────────────────────────────────────

func TestSanitizeKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{testProjectKey, testProjectKey},
		{"my project", "my_project"},
		{"foo/bar.baz", "foo_bar_baz"},
		{"valid_key-123", "valid_key-123"},
	}
	for _, tc := range cases {
		got := sanitizeKey(tc.in)
		if got != tc.want {
			t.Errorf("sanitizeKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ─── sonarToSeverity ──────────────────────────────────────────────────────────

func TestSonarToSeverity(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"BLOCKER", "ERROR"},
		{"CRITICAL", "ERROR"},
		{"MAJOR", "ERROR"},
		{"MINOR", "WARNING"},
		{"INFO", "INFO"},
		{"unknown", "INFO"},
	}
	for _, tc := range cases {
		got := sonarToSeverity(tc.in)
		if got != tc.want {
			t.Errorf("sonarToSeverity(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ─── filterByChangedLines ─────────────────────────────────────────────────────

func TestFilterByChangedLines_keeps(t *testing.T) {
	diags := []gateway.Diagnostic{
		{
			Location: gateway.Location{
				Path:  testFileFooGo,
				Range: gateway.Range{Start: gateway.Position{Line: 10}},
			},
		},
	}
	ranges := []lineRange{{File: testFileFooGo, Start: 5, End: 15}}
	out := filterByChangedLines(diags, ranges)
	if len(out) != 1 {
		t.Errorf(errExpect1Diag, len(out))
	}
}

func TestFilterByChangedLines_keepsAnyLineInChangedFile(t *testing.T) {
	// Issue is on line 100, change is on lines 5-15, but same file
	// → should be kept (filter is file-based, not line-based)
	diags := []gateway.Diagnostic{
		{
			Location: gateway.Location{
				Path:  testFileFooGo,
				Range: gateway.Range{Start: gateway.Position{Line: 100}},
			},
		},
	}
	ranges := []lineRange{{File: testFileFooGo, Start: 5, End: 15}}
	out := filterByChangedLines(diags, ranges)
	if len(out) != 1 {
		t.Errorf("expected 1 diagnostic (same file), got %d", len(out))
	}
}

func TestFilterByChangedLines_wrongFile(t *testing.T) {
	diags := []gateway.Diagnostic{
		{
			Location: gateway.Location{
				Path:  "other.go",
				Range: gateway.Range{Start: gateway.Position{Line: 10}},
			},
		},
	}
	ranges := []lineRange{{File: testFileFooGo, Start: 5, End: 15}}
	out := filterByChangedLines(diags, ranges)
	if len(out) != 0 {
		t.Errorf("expected 0 diagnostics (wrong file), got %d", len(out))
	}
}

// ─── convertIssues ────────────────────────────────────────────────────────────

func TestConvertIssues_stripsProjectKeyPrefix(t *testing.T) {
	issues := []sonarIssue{
		{
			Message:   "null deref",
			Rule:      "java:S1234",
			Severity:  "MAJOR",
			Component: "myproject:src/Main.java",
			Line:      42,
		},
	}
	diags := convertIssues(issues, testSonarURL)
	if len(diags) != 1 {
		t.Fatalf("expected 1 diag, got %d", len(diags))
	}
	if diags[0].Location.Path != "src/Main.java" {
		t.Errorf("Path = %q, want %q", diags[0].Location.Path, "src/Main.java")
	}
	if diags[0].Severity != "ERROR" {
		t.Errorf("Severity = %q, want ERROR", diags[0].Severity)
	}
}

func TestConvertIssues_zeroLineBecomesOne(t *testing.T) {
	issues := []sonarIssue{
		{Message: "msg", Rule: "r", Severity: "INFO", Component: "proj:file.go", Line: 0},
	}
	diags := convertIssues(issues, testSonarURL)
	if diags[0].Location.Range.Start.Line != 1 {
		t.Errorf("Line = %d, want 1", diags[0].Location.Range.Start.Line)
	}
}

// ─── Cleanup ──────────────────────────────────────────────────────────────────

func TestCleanup_removesAutoGeneratedProps(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, propsFile), []byte("test"), 0644)
	os.MkdirAll(filepath.Join(dir, scannerDir), 0755)
	os.WriteFile(filepath.Join(dir, ".sonar_lock"), []byte(""), 0644)

	Cleanup(dir, true)

	if _, err := os.Stat(filepath.Join(dir, propsFile)); err == nil {
		t.Error("sonar-project.properties should be removed when propsCreated=true")
	}
	if _, err := os.Stat(filepath.Join(dir, scannerDir)); err == nil {
		t.Error(".scannerwork should be removed")
	}
	if _, err := os.Stat(filepath.Join(dir, ".sonar_lock")); err == nil {
		t.Error(".sonar_lock should be removed")
	}
}

func TestCleanup_keepsExistingProps(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, propsFile), []byte("keep"), 0644)
	os.MkdirAll(filepath.Join(dir, scannerDir), 0755)

	Cleanup(dir, false)

	if _, err := os.Stat(filepath.Join(dir, propsFile)); err != nil {
		t.Error("sonar-project.properties should be kept when propsCreated=false")
	}
	if _, err := os.Stat(filepath.Join(dir, scannerDir)); err == nil {
		t.Error(".scannerwork should still be removed")
	}
}

// ─── AutoGenerateProperties ───────────────────────────────────────────────────

func TestAutoGenerateProperties_createsFile(t *testing.T) {
	dir := t.TempDir()
	path, created, err := AutoGenerateProperties(dir, testProjectKey)
	if err != nil {
		t.Fatalf("AutoGenerateProperties error: %v", err)
	}
	if !created {
		t.Error("expected created=true for new file")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("properties file not created: %v", err)
	}
	data, _ := os.ReadFile(path)
	content := string(data)
	if !strings.Contains(content, "sonar.projectKey=my-project") {
		t.Errorf("missing projectKey in: %q", content)
	}
	if !strings.Contains(content, "sonar.sources=.") {
		t.Errorf("missing sources in: %q", content)
	}
}

func TestAutoGenerateProperties_existingFileNotOverwritten(t *testing.T) {
	dir := t.TempDir()
	propsPath := filepath.Join(dir, propsFile)
	original := "sonar.projectKey=existing\n"
	os.WriteFile(propsPath, []byte(original), 0644)

	path, created, err := AutoGenerateProperties(dir, "new-key")
	if err != nil {
		t.Fatalf(errFmtString, err)
	}
	if created {
		t.Error("expected created=false for existing file")
	}
	data, _ := os.ReadFile(path)
	if string(data) != original {
		t.Errorf("existing file was overwritten: %q", string(data))
	}
}

func TestAutoGenerateProperties_sanitizesEmptyKey(t *testing.T) {
	dir := t.TempDir()
	// When projectKey is empty, it derives from dir basename
	path, _, err := AutoGenerateProperties(dir, "")
	if err != nil {
		t.Fatalf(errFmtString, err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "sonar.projectKey=") {
		t.Errorf("missing projectKey: %q", string(data))
	}
}

func TestAutoGenerateProperties_jsProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{}`), 0644)
	_, _, err := AutoGenerateProperties(dir, "js-proj")
	if err != nil {
		t.Fatalf(errFmtString, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, propsFile))
	if !strings.Contains(string(data), "lcov.info") {
		t.Errorf("JS project should include lcov path: %q", string(data))
	}
}

func TestAutoGenerateProperties_goProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/foo\n"), 0644)
	_, _, err := AutoGenerateProperties(dir, "go-proj")
	if err != nil {
		t.Fatalf(errFmtString, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, propsFile))
	if !strings.Contains(string(data), "coverage.out") {
		t.Errorf("Go project should include coverage.out: %q", string(data))
	}
}

// ─── readTaskID ───────────────────────────────────────────────────────────────

func TestReadTaskID_found(t *testing.T) {
	dir := t.TempDir()
	reportFile := filepath.Join(dir, reportTaskFile)
	content := "projectKey=my-project\nceTaskId=abc-123\nserverUrl=http://sonar\n"
	os.WriteFile(reportFile, []byte(content), 0644)

	id := readTaskID(reportFile)
	if id != "abc-123" {
		t.Errorf("readTaskID = %q, want %q", id, "abc-123")
	}
}

func TestReadTaskID_missingFile(t *testing.T) {
	id := readTaskID("/nonexistent/report-task.txt")
	if id != "" {
		t.Errorf("readTaskID missing file: got %q, want empty", id)
	}
}

func TestReadTaskID_noTaskIDLine(t *testing.T) {
	dir := t.TempDir()
	reportFile := filepath.Join(dir, reportTaskFile)
	os.WriteFile(reportFile, []byte("projectKey=foo\nserverUrl=http://sonar\n"), 0644)

	id := readTaskID(reportFile)
	if id != "" {
		t.Errorf("readTaskID without ceTaskId: got %q", id)
	}
}

// ─── fetchTaskStatus (HTTP mock) ──────────────────────────────────────────────

func TestFetchTaskStatus_success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"task": map[string]string{"status": "SUCCESS"},
		})
	}))
	defer srv.Close()

	status, err := fetchTaskStatus(srv.URL+"/api/ce/task?id=x", "tok")
	if err != nil {
		t.Fatalf(errFmtString, err)
	}
	if status != "SUCCESS" {
		t.Errorf("status = %q, want SUCCESS", status)
	}
}

func TestFetchTaskStatus_networkError(t *testing.T) {
	_, err := fetchTaskStatus("http://127.0.0.1:1/api/ce/task?id=x", "tok")
	if err == nil {
		t.Error("expected error on connection refused")
	}
}

// ─── WaitForTask (no taskID path) ────────────────────────────────────────────

func TestWaitForTask_noTaskID(t *testing.T) {
	// When there's no report-task.txt, WaitForTask sleeps briefly and returns nil.
	dir := t.TempDir()
	// We pass a very short timeout by having no scannerwork dir.
	// The function sleeps 3s in the no-taskID path — skip timing, just verify no error.
	// Use a non-existent dir so readTaskID returns "".
	err := WaitForTask(testSonarURL, "tok", dir, false)
	if err != nil {
		t.Errorf("WaitForTask no taskID: got error %v", err)
	}
}

func TestWaitForTask_taskSucceeds(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		json.NewEncoder(w).Encode(map[string]interface{}{
			"task": map[string]string{"status": "SUCCESS"},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	// Create .scannerwork/report-task.txt with a task ID
	scannerPath := filepath.Join(dir, scannerDir)
	os.MkdirAll(scannerPath, 0755)
	os.WriteFile(filepath.Join(scannerPath, reportTaskFile),
		[]byte("ceTaskId=task-xyz\n"), 0644)

	err := WaitForTask(srv.URL, "tok", dir, false)
	if err != nil {
		t.Errorf("WaitForTask: expected nil, got %v", err)
	}
	if callCount == 0 {
		t.Error("expected at least one poll call")
	}
}

// ─── FetchResults (HTTP mock) ─────────────────────────────────────────────────

func TestFetchResults_http(t *testing.T) {
	issuesPayload := map[string]interface{}{
		"issues": []map[string]interface{}{
			{
				"message":   "test issue",
				"rule":      "go:S001",
				"severity":  "MINOR",
				"component": "proj:main.go",
				"line":      5,
			},
		},
	}
	hotspotsPayload := map[string]interface{}{
		"hotspots": []map[string]interface{}{},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerContentType, mimeJSON)
		if strings.Contains(r.URL.Path, apiIssuesSearch) {
			json.NewEncoder(w).Encode(issuesPayload)
		} else {
			json.NewEncoder(w).Encode(hotspotsPayload)
		}
	}))
	defer srv.Close()

	cfg := SonarConfig{
		HostURL:    srv.URL,
		Token:      "test-token",
		ProjectKey: "proj",
	}

	result, err := FetchResults(cfg, nil)
	if err != nil {
		t.Fatalf(errFetchResults, err)
	}
	if len(result.Diagnostics) != 1 {
		t.Errorf(errExpect1Diag, len(result.Diagnostics))
	}
	if result.Diagnostics[0].Severity != "WARNING" {
		t.Errorf("Severity = %q, want WARNING (MINOR)", result.Diagnostics[0].Severity)
	}
	if result.HotspotCount != 0 {
		t.Errorf("HotspotCount = %d, want 0", result.HotspotCount)
	}
	if result.Truncated {
		t.Error("Truncated should be false for 1 issue")
	}
}

func TestFetchResults_truncationFlagWhenPageLimitHit(t *testing.T) {
	// Return exactly 500 issues — should set Truncated=true.
	issues := make([]map[string]interface{}, 500)
	for i := range issues {
		issues[i] = map[string]interface{}{
			"message": "issue", "rule": "r", "severity": "INFO",
			"component": "proj:f.go", "line": i + 1,
		}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerContentType, mimeJSON)
		if strings.Contains(r.URL.Path, apiIssuesSearch) {
			json.NewEncoder(w).Encode(map[string]interface{}{"issues": issues})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"hotspots": []interface{}{}})
		}
	}))
	defer srv.Close()

	cfg := SonarConfig{HostURL: srv.URL, Token: "tok", ProjectKey: "proj"}
	result, err := FetchResults(cfg, nil)
	if err != nil {
		t.Fatalf(errFetchResults, err)
	}
	if !result.Truncated {
		t.Error("expected Truncated=true when 500 issues returned")
	}
}

// ─── changedFilesFromRanges ───────────────────────────────────────────────────

func TestChangedFilesFromRanges_basic(t *testing.T) {
	ranges := []lineRange{
		{File: "src/foo.go", Start: 1, End: 10},
		{File: testFileBarGo, Start: 5, End: 20},
		{File: "src/foo.go", Start: 30, End: 40}, // duplicate file
	}
	files := changedFilesFromRanges(ranges)
	if len(files) != 2 {
		t.Fatalf("expected 2 unique files, got %d: %v", len(files), files)
	}
	if files[0] != "src/foo.go" || files[1] != testFileBarGo {
		t.Errorf("unexpected files: %v", files)
	}
}

func TestChangedFilesFromRanges_empty(t *testing.T) {
	files := changedFilesFromRanges(nil)
	if len(files) != 0 {
		t.Errorf("expected 0 files for nil input, got %d", len(files))
	}
}

func TestChangedFilesFromRanges_singleFile(t *testing.T) {
	ranges := []lineRange{
		{File: testFileMainGo, Start: 1, End: 5},
	}
	files := changedFilesFromRanges(ranges)
	if len(files) != 1 || files[0] != testFileMainGo {
		t.Errorf("expected [main.go], got %v", files)
	}
}

// ─── FindScanner ──────────────────────────────────────────────────────────────

func TestFindScanner_foundInPATH(t *testing.T) {
	dir := t.TempDir()

	// Create a fake sonar-scanner binary
	name := scannerName
	if runtime.GOOS == "windows" {
		name = scannerName + ".bat"
	}
	fakeBin := filepath.Join(dir, name)
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// Prepend our temp dir to PATH
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+origPath)

	path, err := FindScanner()
	if err != nil {
		t.Fatalf("FindScanner error: %v", err)
	}
	if path == "" {
		t.Fatal("FindScanner returned empty path")
	}
	// The returned path should point into our temp dir
	if !strings.HasPrefix(path, dir) {
		t.Errorf("FindScanner returned %q, expected path under %q", path, dir)
	}
}

func TestFindScanner_notFound(t *testing.T) {
	// Set PATH to an empty temp dir so scanner won't be found
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	// Also override HOME so ~/.sonar fallback doesn't find anything
	t.Setenv("HOME", dir)
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	}

	_, err := FindScanner()
	if err == nil {
		t.Fatal("expected error when scanner not in PATH")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found': %v", err)
	}
}

func TestFindScanner_foundInSonarHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping ~/.sonar test on Windows")
	}
	dir := t.TempDir()
	// Create ~/.sonar/sonar-scanner/bin/sonar-scanner
	binDir := filepath.Join(dir, ".sonar", scannerName, "bin")
	os.MkdirAll(binDir, 0755)
	fakeBin := filepath.Join(binDir, scannerName)
	os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0755)

	// Set PATH to empty dir and HOME to our temp dir
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)
	t.Setenv("HOME", dir)

	path, err := FindScanner()
	if err != nil {
		t.Fatalf("FindScanner error: %v", err)
	}
	if path != fakeBin {
		t.Errorf("FindScanner = %q, want %q", path, fakeBin)
	}
}

// ─── FetchResults with FilterChanged ──────────────────────────────────────────

func TestFetchResults_filterChanged(t *testing.T) {
	issuesPayload := map[string]interface{}{
		"issues": []map[string]interface{}{
			{
				"message":   "filtered issue",
				"rule":      "go:S002",
				"severity":  "CRITICAL",
				"component": "proj:src/changed.go",
				"line":      10,
			},
		},
	}
	hotspotsPayload := map[string]interface{}{
		"hotspots": []map[string]interface{}{},
	}

	var capturedIssuesURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerContentType, mimeJSON)
		if strings.Contains(r.URL.Path, apiIssuesSearch) {
			capturedIssuesURL = r.URL.String()
			json.NewEncoder(w).Encode(issuesPayload)
		} else {
			json.NewEncoder(w).Encode(hotspotsPayload)
		}
	}))
	defer srv.Close()

	cfg := SonarConfig{
		HostURL:       srv.URL,
		Token:         "tok",
		ProjectKey:    "proj",
		FilterChanged: true,
	}
	ranges := []lineRange{
		{File: "src/changed.go", Start: 5, End: 15},
		{File: "src/other.go", Start: 1, End: 3},
	}

	result, err := FetchResults(cfg, ranges)
	if err != nil {
		t.Fatalf(errFetchResults, err)
	}
	// Verify the URL was narrowed to specific components
	if !strings.Contains(capturedIssuesURL, "proj:src/changed.go") {
		t.Errorf("expected component filter in URL, got %q", capturedIssuesURL)
	}
	if !strings.Contains(capturedIssuesURL, "proj:src/other.go") {
		t.Errorf("expected second component in URL, got %q", capturedIssuesURL)
	}
	if len(result.Diagnostics) != 1 {
		t.Errorf(errExpect1Diag, len(result.Diagnostics))
	}
	// With FilterChanged, Truncated should be false even for many issues
	if result.Truncated {
		t.Error("Truncated should be false when FilterChanged=true")
	}
}

func TestFetchResults_filterChangedNoRanges(t *testing.T) {
	// FilterChanged=true but no ranges → should fall back to project-wide query
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerContentType, mimeJSON)
		if strings.Contains(r.URL.Path, apiIssuesSearch) {
			json.NewEncoder(w).Encode(map[string]interface{}{"issues": []interface{}{}})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"hotspots": []interface{}{}})
		}
	}))
	defer srv.Close()

	cfg := SonarConfig{
		HostURL:       srv.URL,
		Token:         "tok",
		ProjectKey:    "proj",
		FilterChanged: true,
	}
	result, err := FetchResults(cfg, nil)
	if err != nil {
		t.Fatalf(errFetchResults, err)
	}
	if len(result.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics, got %d", len(result.Diagnostics))
	}
}

// ─── WaitForTask additional statuses ──────────────────────────────────────────

func TestWaitForTask_taskFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"task": map[string]string{"status": "FAILED"},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	scannerPath := filepath.Join(dir, scannerDir)
	os.MkdirAll(scannerPath, 0755)
	os.WriteFile(filepath.Join(scannerPath, reportTaskFile),
		[]byte("ceTaskId=task-fail\n"), 0644)

	err := WaitForTask(srv.URL, "tok", dir, false)
	if err == nil {
		t.Fatal("expected error for FAILED task")
	}
	if !strings.Contains(err.Error(), "failed") {
		t.Errorf("error should mention failed: %v", err)
	}
}

func TestWaitForTask_taskCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"task": map[string]string{"status": "CANCELED"},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	scannerPath := filepath.Join(dir, scannerDir)
	os.MkdirAll(scannerPath, 0755)
	os.WriteFile(filepath.Join(scannerPath, reportTaskFile),
		[]byte("ceTaskId=task-cancel\n"), 0644)

	err := WaitForTask(srv.URL, "tok", dir, false)
	if err == nil {
		t.Fatal("expected error for CANCELED task")
	}
	if !strings.Contains(err.Error(), "canceled") {
		t.Errorf("error should mention canceled: %v", err)
	}
}

// ─── RunAnalysis ──────────────────────────────────────────────────────────────

func TestRunAnalysis_binaryNotFound(t *testing.T) {
	cfg := SonarConfig{
		HostURL: "http://localhost:9000",
		Token:   "tok",
	}
	err := RunAnalysis("/nonexistent/sonar-scanner", cfg, nil)
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
	if !strings.Contains(err.Error(), "sonar-scanner failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunAnalysis_ciMode(t *testing.T) {
	cfg := SonarConfig{
		HostURL:      "http://localhost:9000",
		Token:        "tok",
		IsCI:         true,
		PRNumber:     "42",
		PRHeadBranch: "feature",
		BaseBranch:   "main",
	}
	// Will fail because binary doesn't exist, but exercises the CI arg-building path
	err := RunAnalysis("/nonexistent/sonar-scanner", cfg, nil)
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

func TestRunAnalysis_localWithStagedFiles(t *testing.T) {
	cfg := SonarConfig{
		HostURL: "http://localhost:9000",
		Token:   "tok",
		IsCI:    false,
	}
	stagedFiles := []string{"src/foo.go", testFileBarGo}
	// Will fail because binary doesn't exist, but exercises the local arg-building path
	err := RunAnalysis("/nonexistent/sonar-scanner", cfg, stagedFiles)
	if err == nil {
		t.Fatal("expected error for missing binary")
	}
}

// ─── FetchResults error handling ──────────────────────────────────────────────

func TestFetchResults_issuesAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, apiIssuesSearch) {
			w.Write([]byte("not json"))
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"hotspots": []interface{}{}})
		}
	}))
	defer srv.Close()

	cfg := SonarConfig{HostURL: srv.URL, Token: "tok", ProjectKey: "proj"}
	_, err := FetchResults(cfg, nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
}

func TestFetchResults_hotspotsTruncated(t *testing.T) {
	// Return 500 hotspots to trigger truncation
	hotspots := make([]map[string]interface{}, 500)
	for i := range hotspots {
		hotspots[i] = map[string]interface{}{"key": i}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerContentType, mimeJSON)
		if strings.Contains(r.URL.Path, apiIssuesSearch) {
			json.NewEncoder(w).Encode(map[string]interface{}{"issues": []interface{}{}})
		} else {
			json.NewEncoder(w).Encode(map[string]interface{}{"hotspots": hotspots})
		}
	}))
	defer srv.Close()

	cfg := SonarConfig{HostURL: srv.URL, Token: "tok", ProjectKey: "proj"}
	result, err := FetchResults(cfg, nil)
	if err != nil {
		t.Fatalf(errFetchResults, err)
	}
	if !result.Truncated {
		t.Error("expected Truncated=true when 500 hotspots returned")
	}
	if result.HotspotCount != 500 {
		t.Errorf("HotspotCount = %d, want 500", result.HotspotCount)
	}
}
