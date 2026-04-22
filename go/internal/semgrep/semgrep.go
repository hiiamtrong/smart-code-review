// Package semgrep provides helpers for running Semgrep static analysis and
// converting the results into gateway.Diagnostic values.
package semgrep

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/hiiamtrong/smart-code-review/internal/gateway"
)

// SemgrepConfig holds the parameters needed to run a Semgrep scan.
type SemgrepConfig struct {
	Rules string // e.g. "auto", "p/default", ".semgrep.yml"
}

// Result is the structured outcome of a completed Semgrep scan.
type Result struct {
	Diagnostics []gateway.Diagnostic
}

// ─── binary discovery ─────────────────────────────────────────────────────────

// FindSemgrep returns the path to the semgrep binary.
// Looks in PATH, then common pip/brew install locations.
func FindSemgrep() (string, error) {
	name := "semgrep"
	if runtime.GOOS == "windows" {
		name = "semgrep.exe"
	}

	// 1. PATH
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}

	// 2. Common pip install location: ~/.local/bin/semgrep
	home, _ := os.UserHomeDir()
	if home != "" {
		candidate := filepath.Join(home, ".local", "bin", name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("semgrep not found in PATH; install with: pip install semgrep")
}

// ─── scan execution ───────────────────────────────────────────────────────────

// ScanFiles runs semgrep on the given files and returns diagnostics.
// The repoRoot is used to make output paths relative.
// filterExistingFiles returns only files that exist on disk.
// Semgrep exits with code 2 if any target path is missing.
func filterExistingFiles(files []string, repoRoot string) []string {
	var out []string
	for _, f := range files {
		path := f
		if !filepath.IsAbs(path) && repoRoot != "" {
			path = filepath.Join(repoRoot, f)
		}
		if _, err := os.Stat(path); err == nil {
			out = append(out, f)
		}
	}
	return out
}

func ScanFiles(bin string, cfg SemgrepConfig, files []string, repoRoot string) (*Result, error) {
	if len(files) == 0 {
		return &Result{}, nil
	}

	existing := filterExistingFiles(files, repoRoot)
	if len(existing) == 0 {
		return &Result{}, nil
	}

	rules := cfg.Rules
	if rules == "" {
		rules = "auto"
	}

	args := []string{
		"--json",
		"--config", rules,
		"--skip-unknown-extensions",
		"--quiet",
	}
	args = append(args, existing...)

	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.Command(bin, args...) //nolint:gosec
	cmd.Dir = repoRoot

	var stderr strings.Builder
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return nil, fmt.Errorf("semgrep failed: %w", err)
		}
		switch exitErr.ExitCode() {
		case 1:
			// Findings present — continue parsing.
		case 2:
			// Semgrep scan error (bad config, unreachable rules, parse error).
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = exitErr.Error()
			}
			return nil, fmt.Errorf("semgrep scan error: %s", msg)
		default:
			return nil, fmt.Errorf("semgrep failed (exit %d): %s", exitErr.ExitCode(), strings.TrimSpace(stderr.String()))
		}
	}

	diagnostics, parseErr := parseOutput(out, repoRoot)
	if parseErr != nil {
		return nil, fmt.Errorf("parse semgrep output: %w", parseErr)
	}

	return &Result{Diagnostics: diagnostics}, nil
}

// ─── output parsing ───────────────────────────────────────────────────────────

type semgrepOutput struct {
	Results []semgrepResult `json:"results"`
	Errors  []semgrepError  `json:"errors"`
}

type semgrepResult struct {
	CheckID string     `json:"check_id"`
	Path    string     `json:"path"`
	Start   semgrepPos `json:"start"`
	End     semgrepPos `json:"end"`
	Extra   struct {
		Message  string `json:"message"`
		Severity string `json:"severity"` // ERROR, WARNING, INFO
	} `json:"extra"`
}

type semgrepPos struct {
	Line int `json:"line"`
	Col  int `json:"col"`
}

type semgrepError struct {
	Message string `json:"message"`
	Level   string `json:"level"`
}

func parseOutput(data []byte, repoRoot string) ([]gateway.Diagnostic, error) {
	if len(data) == 0 {
		return nil, nil
	}

	var output semgrepOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	diags := make([]gateway.Diagnostic, 0, len(output.Results))
	for _, r := range output.Results {
		path := r.Path
		// Convert absolute paths to repo-relative.
		if filepath.IsAbs(path) && repoRoot != "" {
			if rel, err := filepath.Rel(repoRoot, path); err == nil {
				path = rel
			}
		}

		severity := mapSeverity(r.Extra.Severity)

		diags = append(diags, gateway.Diagnostic{
			Message:  r.Extra.Message,
			Severity: severity,
			Location: gateway.Location{
				Path: path,
				Range: gateway.Range{
					Start: gateway.Position{Line: r.Start.Line, Column: r.Start.Col},
					End:   gateway.Position{Line: r.End.Line, Column: r.End.Col},
				},
			},
			Code: gateway.Code{
				Value: r.CheckID,
			},
		})
	}

	return diags, nil
}

func mapSeverity(s string) string {
	switch strings.ToUpper(s) {
	case "ERROR":
		return "ERROR"
	case "WARNING":
		return "WARNING"
	default:
		return "INFO"
	}
}
