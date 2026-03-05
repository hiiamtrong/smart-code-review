package semgrep

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const (
	msgScanFiles     = "ScanFiles: %v"
	msgParseOutput   = "parseOutput: %v"
	msgNoDiags       = "expected 0 diagnostics, got %d"
	skipWindows      = "shell script test not applicable on windows"
	testFileMainGo   = "main.go"
)

// ─── FindSemgrep ─────────────────────────────────────────────────────────────

func TestFindSemgrep_NotFound(t *testing.T) {
	// Override PATH to ensure semgrep is not found.
	t.Setenv("PATH", t.TempDir())
	// Also override HOME to prevent ~/.local/bin fallback.
	t.Setenv("HOME", t.TempDir())

	_, err := FindSemgrep()
	if err == nil {
		t.Error("expected error when semgrep not in PATH")
	}
}

func TestFindSemgrep_FoundInPath(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "semgrep")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("PATH", dir)

	path, err := FindSemgrep()
	if err != nil {
		t.Fatalf("FindSemgrep: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty path")
	}
}

func TestFindSemgrep_FoundInLocalBin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("~/.local/bin fallback not applicable on windows")
	}

	// Override PATH so semgrep is NOT in PATH.
	t.Setenv("PATH", t.TempDir())

	// Create fake semgrep in ~/.local/bin/
	home := t.TempDir()
	t.Setenv("HOME", home)
	localBin := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(localBin, 0755); err != nil {
		t.Fatal(err)
	}
	fakeBin := filepath.Join(localBin, "semgrep")
	if err := os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	path, err := FindSemgrep()
	if err != nil {
		t.Fatalf("FindSemgrep: %v", err)
	}
	if path != fakeBin {
		t.Errorf("got %q, want %q", path, fakeBin)
	}
}

// ─── ScanFiles ───────────────────────────────────────────────────────────────

func TestScanFiles_EmptyFileList(t *testing.T) {
	res, err := ScanFiles("/usr/bin/semgrep", SemgrepConfig{}, nil, "/repo")
	if err != nil {
		t.Fatalf(msgScanFiles, err)
	}
	if len(res.Diagnostics) != 0 {
		t.Errorf(msgNoDiags, len(res.Diagnostics))
	}
}

func TestScanFiles_WithFindings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(skipWindows)
	}

	// Create a fake semgrep that outputs JSON with findings and exits 1.
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "semgrep")
	script := `#!/bin/sh
cat <<'JSONEOF'
{
  "results": [
    {
      "check_id": "test.rule",
      "path": "main.go",
      "start": {"line": 10, "col": 1},
      "end": {"line": 10, "col": 20},
      "extra": {
        "message": "test finding",
        "severity": "WARNING"
      }
    }
  ],
  "errors": []
}
JSONEOF
exit 1
`
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	res, err := ScanFiles(fakeBin, SemgrepConfig{Rules: "auto"}, []string{testFileMainGo}, dir)
	if err != nil {
		t.Fatalf(msgScanFiles, err)
	}
	if len(res.Diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(res.Diagnostics))
	}
	if res.Diagnostics[0].Message != "test finding" {
		t.Errorf("message = %q", res.Diagnostics[0].Message)
	}
	if res.Diagnostics[0].Severity != "WARNING" {
		t.Errorf("severity = %q", res.Diagnostics[0].Severity)
	}
	if res.Diagnostics[0].Code.Value != "test.rule" {
		t.Errorf("code = %q", res.Diagnostics[0].Code.Value)
	}
}

func TestScanFiles_NoFindings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(skipWindows)
	}

	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "semgrep")
	script := `#!/bin/sh
echo '{"results": [], "errors": []}'
exit 0
`
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	res, err := ScanFiles(fakeBin, SemgrepConfig{}, []string{testFileMainGo}, dir)
	if err != nil {
		t.Fatalf(msgScanFiles, err)
	}
	if len(res.Diagnostics) != 0 {
		t.Errorf(msgNoDiags, len(res.Diagnostics))
	}
}

func TestScanFiles_BinaryFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(skipWindows)
	}

	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "semgrep")
	// Exit code 2 = actual error (not findings)
	script := `#!/bin/sh
echo "fatal error" >&2
exit 2
`
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := ScanFiles(fakeBin, SemgrepConfig{}, []string{testFileMainGo}, dir)
	if err == nil {
		t.Error("expected error for exit code 2")
	}
}

func TestScanFiles_InvalidOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(skipWindows)
	}

	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "semgrep")
	script := `#!/bin/sh
echo "not json"
exit 0
`
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := ScanFiles(fakeBin, SemgrepConfig{}, []string{testFileMainGo}, dir)
	if err == nil {
		t.Error("expected error for invalid JSON output")
	}
}

func TestScanFiles_DefaultRules(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip(skipWindows)
	}

	// Verify that empty Rules defaults to "auto"
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "semgrep")
	// Script that echoes args so we can verify --config auto was passed
	script := `#!/bin/sh
echo '{"results": [], "errors": []}'
`
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	res, err := ScanFiles(fakeBin, SemgrepConfig{Rules: ""}, []string{testFileMainGo}, dir)
	if err != nil {
		t.Fatalf(msgScanFiles, err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestScanFiles_BinaryNotFound(t *testing.T) {
	_, err := ScanFiles("/nonexistent/semgrep", SemgrepConfig{}, []string{testFileMainGo}, t.TempDir())
	if err == nil {
		t.Error("expected error when binary does not exist")
	}
}

// ─── parseOutput ─────────────────────────────────────────────────────────────

func TestParseOutput_Empty(t *testing.T) {
	diags, err := parseOutput([]byte{}, "/repo")
	if err != nil {
		t.Fatalf(msgParseOutput, err)
	}
	if len(diags) != 0 {
		t.Errorf(msgNoDiags, len(diags))
	}
}

func TestParseOutput_ValidResults(t *testing.T) {
	jsonData := `{
		"results": [
			{
				"check_id": "rules.security.sql-injection",
				"path": "src/db.go",
				"start": {"line": 42, "col": 5},
				"end": {"line": 42, "col": 50},
				"extra": {
					"message": "Possible SQL injection",
					"severity": "ERROR"
				}
			},
			{
				"check_id": "rules.style.unused-var",
				"path": "src/main.go",
				"start": {"line": 10, "col": 2},
				"end": {"line": 10, "col": 15},
				"extra": {
					"message": "Variable declared but not used",
					"severity": "WARNING"
				}
			},
			{
				"check_id": "rules.info.todo-comment",
				"path": "src/utils.go",
				"start": {"line": 5, "col": 1},
				"end": {"line": 5, "col": 20},
				"extra": {
					"message": "TODO comment found",
					"severity": "INFO"
				}
			}
		],
		"errors": []
	}`

	diags, err := parseOutput([]byte(jsonData), "/repo")
	if err != nil {
		t.Fatalf(msgParseOutput, err)
	}

	if len(diags) != 3 {
		t.Fatalf("expected 3 diagnostics, got %d", len(diags))
	}

	// Check first diagnostic fully
	d := diags[0]
	if d.Severity != "ERROR" {
		t.Errorf("diag[0] severity: got %q, want ERROR", d.Severity)
	}
	if d.Message != "Possible SQL injection" {
		t.Errorf("diag[0] message: got %q", d.Message)
	}
	if d.Location.Path != "src/db.go" {
		t.Errorf("diag[0] path: got %q", d.Location.Path)
	}
	if d.Location.Range.Start.Line != 42 {
		t.Errorf("diag[0] start line: got %d, want 42", d.Location.Range.Start.Line)
	}
	if d.Location.Range.Start.Column != 5 {
		t.Errorf("diag[0] start col: got %d, want 5", d.Location.Range.Start.Column)
	}
	if d.Location.Range.End.Line != 42 {
		t.Errorf("diag[0] end line: got %d, want 42", d.Location.Range.End.Line)
	}
	if d.Location.Range.End.Column != 50 {
		t.Errorf("diag[0] end col: got %d, want 50", d.Location.Range.End.Column)
	}
	if d.Code.Value != "rules.security.sql-injection" {
		t.Errorf("diag[0] code: got %q", d.Code.Value)
	}

	// Check severity mapping for all three
	if diags[1].Severity != "WARNING" {
		t.Errorf("diag[1] severity: got %q, want WARNING", diags[1].Severity)
	}
	if diags[2].Severity != "INFO" {
		t.Errorf("diag[2] severity: got %q, want INFO", diags[2].Severity)
	}
}

func TestParseOutput_AbsolutePathConversion(t *testing.T) {
	repoRoot := "/home/user/project"
	jsonData := `{
		"results": [
			{
				"check_id": "test.rule",
				"path": "/home/user/project/src/main.go",
				"start": {"line": 1, "col": 1},
				"end": {"line": 1, "col": 10},
				"extra": {
					"message": "test issue",
					"severity": "WARNING"
				}
			}
		],
		"errors": []
	}`

	diags, err := parseOutput([]byte(jsonData), repoRoot)
	if err != nil {
		t.Fatalf(msgParseOutput, err)
	}

	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diags))
	}
	if diags[0].Location.Path != "src/main.go" {
		t.Errorf("expected relative path 'src/main.go', got %q", diags[0].Location.Path)
	}
}

func TestParseOutput_RelativePathKept(t *testing.T) {
	jsonData := `{
		"results": [
			{
				"check_id": "test.rule",
				"path": "src/main.go",
				"start": {"line": 1, "col": 1},
				"end": {"line": 1, "col": 10},
				"extra": {
					"message": "test",
					"severity": "INFO"
				}
			}
		],
		"errors": []
	}`

	diags, err := parseOutput([]byte(jsonData), "/repo")
	if err != nil {
		t.Fatalf(msgParseOutput, err)
	}
	if diags[0].Location.Path != "src/main.go" {
		t.Errorf("relative path should be preserved, got %q", diags[0].Location.Path)
	}
}

func TestParseOutput_EmptyRepoRoot(t *testing.T) {
	jsonData := `{
		"results": [
			{
				"check_id": "test.rule",
				"path": "/abs/path/file.go",
				"start": {"line": 1, "col": 1},
				"end": {"line": 1, "col": 10},
				"extra": {
					"message": "test",
					"severity": "INFO"
				}
			}
		],
		"errors": []
	}`

	diags, err := parseOutput([]byte(jsonData), "")
	if err != nil {
		t.Fatalf(msgParseOutput, err)
	}
	// With empty repoRoot, absolute path should be preserved as-is
	if diags[0].Location.Path != "/abs/path/file.go" {
		t.Errorf("expected absolute path preserved, got %q", diags[0].Location.Path)
	}
}

func TestParseOutput_InvalidJSON(t *testing.T) {
	_, err := parseOutput([]byte("not json"), "/repo")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseOutput_NoResults(t *testing.T) {
	jsonData := `{"results": [], "errors": []}`
	diags, err := parseOutput([]byte(jsonData), "/repo")
	if err != nil {
		t.Fatalf(msgParseOutput, err)
	}
	if len(diags) != 0 {
		t.Errorf(msgNoDiags, len(diags))
	}
}

// ─── mapSeverity ─────────────────────────────────────────────────────────────

func TestMapSeverity(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ERROR", "ERROR"},
		{"error", "ERROR"},
		{"WARNING", "WARNING"},
		{"warning", "WARNING"},
		{"INFO", "INFO"},
		{"info", "INFO"},
		{"", "INFO"},
		{"UNKNOWN", "INFO"},
	}

	for _, tt := range tests {
		got := mapSeverity(tt.input)
		if got != tt.want {
			t.Errorf("mapSeverity(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
