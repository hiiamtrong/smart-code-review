package display

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/fatih/color"
)

func init() {
	// Disable ANSI color codes so output is plain text in tests.
	color.NoColor = true
}

// captureColorOutput redirects color.Output and returns whatever was written.
func captureColorOutput(fn func()) string {
	var buf bytes.Buffer
	old := color.Output
	color.Output = &buf
	fn()
	color.Output = old
	return buf.String()
}

// captureStderr redirects os.Stderr and returns whatever was written.
func captureStderr(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stderr
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	io.Copy(&buf, r) //nolint:errcheck
	return buf.String()
}

// captureStdout redirects os.Stdout (for raw fmt.Printf calls) and color.Output.
func captureAll(fn func()) string {
	// Redirect os.Stdout for fmt.Printf calls.
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	// Redirect color.Output for color.Println calls.
	var colBuf bytes.Buffer
	oldColor := color.Output
	color.Output = &colBuf

	fn()

	w.Close()
	os.Stdout = oldStdout
	color.Output = oldColor

	var stdBuf bytes.Buffer
	io.Copy(&stdBuf, r) //nolint:errcheck
	return colBuf.String() + stdBuf.String()
}

func TestLogError(t *testing.T) {
	out := captureStderr(func() { LogError("something failed") })
	if !strings.Contains(out, "[ERROR]") {
		t.Errorf("LogError missing [ERROR] prefix; got: %q", out)
	}
	if !strings.Contains(out, "something failed") {
		t.Errorf("LogError missing message; got: %q", out)
	}
}

func TestLogWarn(t *testing.T) {
	out := captureColorOutput(func() { LogWarn("be careful") })
	if !strings.Contains(out, "[WARN]") {
		t.Errorf("LogWarn missing [WARN] prefix; got: %q", out)
	}
	if !strings.Contains(out, "be careful") {
		t.Errorf("LogWarn missing message; got: %q", out)
	}
}

func TestLogInfo(t *testing.T) {
	out := captureColorOutput(func() { LogInfo("processing") })
	if !strings.Contains(out, "[INFO]") {
		t.Errorf("LogInfo missing [INFO] prefix; got: %q", out)
	}
	if !strings.Contains(out, "processing") {
		t.Errorf("LogInfo missing message; got: %q", out)
	}
}

func TestLogSuccess(t *testing.T) {
	out := captureColorOutput(func() { LogSuccess("done") })
	if !strings.Contains(out, "[OK]") {
		t.Errorf("LogSuccess missing [OK] prefix; got: %q", out)
	}
	if !strings.Contains(out, "done") {
		t.Errorf("LogSuccess missing message; got: %q", out)
	}
}

func TestPrintSeparator(t *testing.T) {
	out := captureColorOutput(func() { PrintSeparator() })
	if !strings.Contains(out, "─") {
		t.Errorf("PrintSeparator missing separator character; got: %q", out)
	}
}

func TestDivider(t *testing.T) {
	out := captureAll(func() { Divider() })
	if !strings.Contains(out, "━") {
		t.Errorf("Divider missing separator character; got: %q", out)
	}
}

func TestPrintIssue_Error(t *testing.T) {
	out := captureColorOutput(func() { PrintIssue("ERROR", "main.go", 42, "null pointer dereference") })
	if !strings.Contains(out, "[ERROR]") {
		t.Errorf("PrintIssue ERROR missing prefix; got: %q", out)
	}
	if !strings.Contains(out, "null pointer dereference") {
		t.Errorf("PrintIssue ERROR missing message; got: %q", out)
	}
	if !strings.Contains(out, "main.go:42") {
		t.Errorf("PrintIssue ERROR missing file:line; got: %q", out)
	}
}

func TestPrintIssue_Warning(t *testing.T) {
	out := captureColorOutput(func() { PrintIssue("WARNING", "util.go", 10, "unused variable") })
	if !strings.Contains(out, "[WARN]") {
		t.Errorf("PrintIssue WARNING missing prefix; got: %q", out)
	}
	if !strings.Contains(out, "util.go:10") {
		t.Errorf("PrintIssue WARNING missing file:line; got: %q", out)
	}
}

func TestPrintIssue_Info(t *testing.T) {
	out := captureColorOutput(func() { PrintIssue("INFO", "foo.go", 1, "consider refactoring") })
	if !strings.Contains(out, "[INFO]") {
		t.Errorf("PrintIssue INFO missing prefix; got: %q", out)
	}
	if !strings.Contains(out, "foo.go:1") {
		t.Errorf("PrintIssue INFO missing file:line; got: %q", out)
	}
}

func TestPrintIssue_Default(t *testing.T) {
	// Any severity other than ERROR/WARNING falls through to the default [INFO] branch.
	out := captureColorOutput(func() { PrintIssue("UNKNOWN", "bar.go", 5, "misc message") })
	if !strings.Contains(out, "[INFO]") {
		t.Errorf("PrintIssue UNKNOWN should use [INFO] default; got: %q", out)
	}
}

func TestPrintSummary(t *testing.T) {
	out := captureAll(func() { PrintSummary(3, 2, 1) })
	if !strings.Contains(out, "3 errors") {
		t.Errorf("PrintSummary missing errors count; got: %q", out)
	}
	if !strings.Contains(out, "2 warnings") {
		t.Errorf("PrintSummary missing warnings count; got: %q", out)
	}
	if !strings.Contains(out, "1 info") {
		t.Errorf("PrintSummary missing info count; got: %q", out)
	}
}

func TestPrintHeader(t *testing.T) {
	out := captureAll(func() { PrintHeader("1.2.3") })
	if !strings.Contains(out, "v1.2.3") {
		t.Errorf("PrintHeader missing version; got: %q", out)
	}
}

func TestPrintStageHeader(t *testing.T) {
	out := captureAll(func() { PrintStageHeader("Lint") })
	if !strings.Contains(out, "Lint") {
		t.Errorf("PrintStageHeader missing stage name; got: %q", out)
	}
	if !strings.Contains(out, "━") {
		t.Errorf("PrintStageHeader missing divider; got: %q", out)
	}
}

func TestPrintIssueWithSourcePrefix(t *testing.T) {
	out := captureAll(func() {
		PrintIssueWithSource("golint", "ERROR", "main.go", 10, "exported func missing doc")
	})
	if !strings.Contains(out, "golint:") {
		t.Errorf("missing source prefix; got: %q", out)
	}
	if !strings.Contains(out, "exported func missing doc") {
		t.Errorf("missing message; got: %q", out)
	}
	if !strings.Contains(out, "main.go:10") {
		t.Errorf("missing file:line; got: %q", out)
	}
}

func TestPrintIssueWithSourceNoSource(t *testing.T) {
	out := captureAll(func() {
		PrintIssueWithSource("", "WARNING", "util.go", 5, "shadow variable")
	})
	if strings.Contains(out, ": : ") {
		t.Errorf("empty source should not produce double colon; got: %q", out)
	}
	if !strings.Contains(out, "[WARN]") {
		t.Errorf("WARNING missing prefix; got: %q", out)
	}
	if !strings.Contains(out, "shadow variable") {
		t.Errorf("missing message; got: %q", out)
	}
	if !strings.Contains(out, "util.go:5") {
		t.Errorf("missing file:line; got: %q", out)
	}
}

func TestPrintIssueWithSourceInfo(t *testing.T) {
	out := captureAll(func() {
		PrintIssueWithSource("mycheck", "INFO", "foo.go", 99, "consider refactoring")
	})
	if !strings.Contains(out, "[INFO]") {
		t.Errorf("INFO missing prefix; got: %q", out)
	}
	if !strings.Contains(out, "mycheck:") {
		t.Errorf("missing source; got: %q", out)
	}
}

func TestPrintStageSummary(t *testing.T) {
	out := captureAll(func() {
		PrintStageSummary(StageSummary{Name: "Lint", Errors: 2, Warnings: 3, Infos: 1})
	})
	if !strings.Contains(out, "Lint") {
		t.Errorf("PrintStageSummary missing stage name; got: %q", out)
	}
	if !strings.Contains(out, "2 errors") {
		t.Errorf("PrintStageSummary missing errors; got: %q", out)
	}
	if !strings.Contains(out, "3 warnings") {
		t.Errorf("PrintStageSummary missing warnings; got: %q", out)
	}
	if !strings.Contains(out, "1 info") {
		t.Errorf("PrintStageSummary missing info; got: %q", out)
	}
}

func TestPrintStageSummaries(t *testing.T) {
	stages := []StageSummary{
		{Name: "Lint", Errors: 1, Warnings: 2, Infos: 0},
		{Name: "Security", Errors: 0, Warnings: 1, Infos: 3},
	}
	out := captureAll(func() {
		PrintStageSummaries(stages, 1, 3, 3)
	})
	if !strings.Contains(out, "Lint:") {
		t.Errorf("PrintStageSummaries missing Lint stage; got: %q", out)
	}
	if !strings.Contains(out, "Security:") {
		t.Errorf("PrintStageSummaries missing Security stage; got: %q", out)
	}
	if !strings.Contains(out, "1 errors, 3 warnings, 3 info") {
		t.Errorf("PrintStageSummaries missing total summary; got: %q", out)
	}
	if !strings.Contains(out, "Review Summary") {
		t.Errorf("PrintStageSummaries missing Review Summary label; got: %q", out)
	}
}
