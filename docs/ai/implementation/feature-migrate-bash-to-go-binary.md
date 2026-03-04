---
phase: implementation
title: Implementation Guide — Migrate Bash Scripts to Go Binary
description: Technical implementation notes for the ai-review Go binary
---

# Implementation Guide

## Development Setup

**Prerequisites:**
- Go 1.22+ (`go version`)
- goreleaser (optional, for local release builds): `brew install goreleaser`
- Access to AI Gateway endpoint for integration testing

**Project layout:**
```
smart-code-review/
├── go/                          # NEW: Go binary source
│   ├── cmd/
│   │   └── ai-review/
│   │       └── main.go          # Entry point
│   ├── internal/
│   │   ├── cmd/                 # Cobra subcommands
│   │   │   ├── root.go
│   │   │   ├── setup.go
│   │   │   ├── install.go
│   │   │   ├── uninstall.go
│   │   │   ├── config.go
│   │   │   ├── status.go
│   │   │   ├── update.go
│   │   │   ├── runhook.go       # run-hook subcommand
│   │   │   └── cireview.go      # ci-review subcommand
│   │   ├── config/
│   │   │   ├── config.go        # Config struct + read/write
│   │   │   └── config_test.go
│   │   ├── git/
│   │   │   ├── git.go
│   │   │   └── git_test.go
│   │   ├── filter/
│   │   │   ├── filter.go
│   │   │   └── filter_test.go
│   │   ├── language/
│   │   │   ├── language.go
│   │   │   └── language_test.go
│   │   ├── gateway/
│   │   │   ├── client.go        # HTTP client, streaming SSE
│   │   │   ├── sse.go           # SSE parser
│   │   │   └── client_test.go
│   │   ├── display/
│   │   │   ├── display.go
│   │   │   └── display_test.go
│   │   ├── sonarqube/
│   │   │   ├── sonarqube.go
│   │   │   └── sonarqube_test.go
│   │   ├── reviewdog/
│   │   │   ├── reviewdog.go
│   │   │   └── reviewdog_test.go
│   │   └── installer/
│   │       ├── installer.go
│   │       └── installer_test.go
│   ├── go.mod
│   ├── go.sum
│   └── .goreleaser.yml
├── scripts/                     # KEEP: bash scripts (deprecated over time)
│   └── ...
```

**Bootstrap commands:**
```bash
mkdir -p go/cmd/ai-review go/internal/{cmd,config,git,filter,language,gateway,display,sonarqube,reviewdog,installer}
cd go
go mod init github.com/hiiamtrong/smart-code-review
go get github.com/spf13/cobra@latest
go get github.com/fatih/color@latest
go get github.com/bmatcuk/doublestar/v4@latest   # for gitignore-style glob
```

## Code Structure & Key Implementation Notes

### pkg/config — Shell-format config parser

The existing config file uses bash syntax: `KEY="VALUE"`. Go must read and write this without breaking existing user configs.

```go
// ParseShellConfig parses KEY="VALUE" format, stripping quotes.
func ParseShellConfig(r io.Reader) (map[string]string, error) {
    result := make(map[string]string)
    scanner := bufio.NewScanner(r)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        parts := strings.SplitN(line, "=", 2)
        if len(parts) != 2 {
            continue
        }
        key := strings.TrimSpace(parts[0])
        val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
        result[key] = val
    }
    return result, scanner.Err()
}

// WriteShellConfig writes KEY="VALUE" format compatible with bash source.
func WriteShellConfig(path string, cfg map[string]string) error {
    var buf bytes.Buffer
    buf.WriteString("# AI Review Configuration\n")
    for k, v := range cfg {
        fmt.Fprintf(&buf, "%s=%q\n", k, v)
    }
    return os.WriteFile(path, buf.Bytes(), 0600)
}
```

**Config directory:**
```go
func ConfigDir() string {
    if runtime.GOOS == "windows" {
        if appData := os.Getenv("APPDATA"); appData != "" {
            return filepath.Join(appData, "ai-review")
        }
    }
    home, _ := os.UserHomeDir()
    return filepath.Join(home, ".config", "ai-review")
}
```

### pkg/filter — .aireviewignore implementation

Port the bash glob-to-regex logic. Use `bmatcuk/doublestar` for `**` support which bash version approximates poorly.

```go
// FilterDiff removes diff blocks for files matching any ignore pattern.
func FilterDiff(diff string, patterns []string) (string, int) {
    type block struct {
        file    string
        content strings.Builder
    }

    var blocks []block
    var current *block

    for _, line := range strings.Split(diff, "\n") {
        if m := diffGitRe.FindStringSubmatch(line); m != nil {
            // Save previous block
            if current != nil {
                blocks = append(blocks, *current)
            }
            current = &block{file: m[2]}
            current.content.WriteString(line + "\n")
        } else if current != nil {
            current.content.WriteString(line + "\n")
        }
    }
    if current != nil {
        blocks = append(blocks, *current)
    }

    var out strings.Builder
    ignoredCount := 0
    for _, b := range blocks {
        if matchesAnyPattern(b.file, patterns) {
            ignoredCount++
        } else {
            out.WriteString(b.content.String())
        }
    }
    return out.String(), ignoredCount
}

func matchesAnyPattern(file string, patterns []string) bool {
    for _, p := range patterns {
        // doublestar.Match handles **, *, ? gitignore-style patterns
        if ok, _ := doublestar.Match(p, file); ok {
            return true
        }
        // Also try matching as suffix (e.g. "*.lock" should match "dir/package.lock")
        if ok, _ := doublestar.Match("**/"+p, file); ok {
            return true
        }
    }
    return false
}
```

### pkg/gateway — SSE Streaming

SSE is `text/event-stream`: lines of `event: TYPE\ndata: JSON\n\n`. Port the bash `while IFS= read -r` loop exactly.

```go
func StreamingReview(ctx context.Context, cfg *Config, payload ReviewPayload, onDiagnostic func(Diagnostic)) (*ReviewResult, error) {
    // Build multipart body
    body := &bytes.Buffer{}
    w := multipart.NewWriter(body)
    _ = w.WriteField("metadata", mustMarshal(payload.toMetadata()))
    fw, _ := w.CreateFormFile("git_diff", "diff.txt")
    _, _ = fw.Write([]byte(payload.Diff))
    w.Close()

    req, _ := http.NewRequestWithContext(ctx, "POST", cfg.AIGatewayURL+"/review", body)
    req.Header.Set("Content-Type", w.FormDataContentType())
    req.Header.Set("X-API-Key", cfg.AIGatewayAPIKey)
    req.Header.Set("Accept", "text/event-stream")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    return parseSSEStream(resp.Body, onDiagnostic)
}

func parseSSEStream(r io.Reader, onDiagnostic func(Diagnostic)) (*ReviewResult, error) {
    scanner := bufio.NewScanner(r)
    scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1MB buffer for large SSE lines

    var currentEvent string
    var textBuffer strings.Builder
    var allDiagnostics []Diagnostic
    var result *ReviewResult

    for scanner.Scan() {
        line := strings.TrimRight(scanner.Text(), "\r")

        if strings.HasPrefix(line, "event: ") {
            currentEvent = strings.TrimPrefix(line, "event: ")
            continue
        }
        if !strings.HasPrefix(line, "data: ") {
            continue
        }
        data := strings.TrimPrefix(line, "data: ")
        if data == "" {
            continue
        }

        switch currentEvent {
        case "diagnostic":
            var d Diagnostic
            if err := json.Unmarshal([]byte(data), &d); err == nil && d.Message != "" {
                allDiagnostics = append(allDiagnostics, d)
                if onDiagnostic != nil {
                    onDiagnostic(d)
                }
            }
        case "text":
            var t struct{ Text string `json:"text"` }
            if err := json.Unmarshal([]byte(data), &t); err == nil {
                textBuffer.WriteString(t.Text)
            }
        case "complete":
            var c struct {
                Overview        string `json:"overview"`
                TotalDiagnostics int   `json:"total_diagnostics"`
                Severity        string `json:"severity"`
            }
            if err := json.Unmarshal([]byte(data), &c); err == nil {
                result = &ReviewResult{
                    Source:      Source{Name: "ai-review"},
                    Diagnostics: allDiagnostics,
                    Overview:    c.Overview,
                    MaxSeverity: c.Severity,
                }
            }
        case "error":
            var e struct{ Message string `json:"message"` }
            json.Unmarshal([]byte(data), &e)
            return nil, fmt.Errorf("AI gateway error: %s", e.Message)
        }
    }
    if result == nil && len(allDiagnostics) > 0 {
        result = &ReviewResult{Diagnostics: allDiagnostics}
    }
    return result, scanner.Err()
}
```

### pkg/display — Cross-platform colors

```go
// Use fatih/color which auto-detects Windows ANSI support via go-isatty.
var (
    Red    = color.New(color.FgRed)
    Green  = color.New(color.FgGreen)
    Yellow = color.New(color.FgYellow)
    Blue   = color.New(color.FgBlue)
    Cyan   = color.New(color.FgCyan)
    Bold   = color.New(color.Bold)
)

func LogError(msg string) { Red.Fprintf(os.Stderr, "[ERROR] %s\n", msg) }
func LogWarn(msg string)  { Yellow.Printf("[WARN] %s\n", msg) }
func LogInfo(msg string)  { Blue.Printf("[INFO] %s\n", msg) }
func LogSuccess(msg string) { Green.Printf("[OK] %s\n", msg) }

func PrintIssue(severity, file string, line int, message string) {
    switch severity {
    case "ERROR":
        Red.Printf("[ERROR] %s\n", message)
    case "WARNING":
        Yellow.Printf("[WARN] %s\n", message)
    default:
        Blue.Printf("[INFO] %s\n", message)
    }
    Bold.Printf("        %s:%d\n", file, line)
}
```

### cmd/run-hook — Pre-commit orchestration

```go
func runHook(cmd *cobra.Command, args []string) error {
    cfg, err := config.Load()
    if err != nil { return err }

    if !cfg.EnableAIReview && !cfg.EnableSonarQube {
        display.LogInfo("Both AI Review and SonarQube are disabled. Nothing to check.")
        return nil
    }

    // SonarQube first (if enabled)
    if cfg.EnableSonarQube {
        if err := sonarqube.RunAndCheck(cfg); err != nil {
            return err // exits with code 1, blocking commit
        }
    }

    if !cfg.EnableAIReview {
        return nil
    }

    // Get staged diff
    diff, err := git.GetStagedDiff()
    if err != nil { return err }
    if diff == "" {
        display.LogSuccess("No staged changes to review")
        return nil
    }

    // Filter ignored files
    ignorePatterns, _ := filter.LoadIgnorePatterns(repoIgnoreFile())
    filteredDiff, ignoredCount := filter.FilterDiff(diff, ignorePatterns)
    if ignoredCount > 0 {
        display.LogInfo(fmt.Sprintf("Ignored %d file(s) from .aireviewignore", ignoredCount))
    }
    if filteredDiff == "" {
        display.LogSuccess("All changes are in ignored files, skipping review")
        return nil
    }

    lang := language.DetectFromDiff(filteredDiff)
    gitInfo, _ := git.GetGitInfo()

    // Stream from AI Gateway
    display.PrintSeparator()
    display.LogInfo("AI is reviewing your code...")

    result, err := gateway.StreamingReview(context.Background(), cfg, gateway.ReviewPayload{
        Diff: filteredDiff, Language: lang, GitInfo: gitInfo,
        AIModel: cfg.AIModel, AIProvider: cfg.AIProvider, Stream: true,
    }, func(d gateway.Diagnostic) {
        display.PrintIssue(d.Severity, d.Location.Path, d.Location.Range.Start.Line, d.Message)
    })
    if err != nil {
        display.LogError(fmt.Sprintf("AI review error: %v", err))
        display.LogError("Commit blocked - AI review service error")
        fmt.Println("        Use 'git commit --no-verify' to bypass")
        os.Exit(1)
    }

    return display.ShowResultsAndExit(result)
}
```

### Installer — Hook file template

```go
const hookTemplate = `#!/usr/bin/env sh
# AI-REVIEW-HOOK
# Managed by ai-review (https://github.com/hiiamtrong/smart-code-review)
# Do not edit manually. Run 'ai-review uninstall' to remove.

exec ai-review run-hook "$@"
`
```

## Integration Points

- **AI Gateway**: POST `$AI_GATEWAY_URL/review` with `multipart/form-data` (fields: `metadata` JSON, `git_diff` file)
- **reviewdog**: exec `$HOME/bin/reviewdog -f=rdjson -name=ai-review -reporter=github-pr-review`
- **sonar-scanner**: exec from PATH or configured location
- **GitHub API**: `POST /repos/{owner}/{repo}/issues/{pr_number}/comments` for overview comment

## Error Handling

- All subprocess invocations use `exec.Command` with argument slices (never `sh -c`), preventing shell injection
- Git errors: wrap with context (`fmt.Errorf("get staged diff: %w", err)`)
- Gateway timeout: `context.WithTimeout(ctx, cfg.Timeout)` — default 120s
- SSE parse error → log warning → fall back to `SyncReview`
- Config missing → exit 1 with "Run `ai-review setup`" message
- Temp files: use `os.CreateTemp` + `defer os.Remove(f.Name())`

## Performance Considerations

- Go binary cold start: ~10-20ms vs bash script overhead of ~50-200ms per subprocess
- Avoid reading entire diff into memory multiple times: pass `io.Reader` where possible
- SSE scanner uses 1MB buffer to handle large single-line JSON events
- `os.MkdirAll` once at startup, not repeatedly

## Security Notes

- Config file: `os.WriteFile(path, data, 0600)` — readable only by owner
- API key: never printed in full; masked as `****` in `config` command output
- No shell expansion: all `exec.Command` calls use `[]string` arguments
- Temp files: use `os.CreateTemp` (random suffix), not predictable filenames
- GitHub tokens: read from env var `GITHUB_TOKEN` only, never written to disk
