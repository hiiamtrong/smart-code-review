---
phase: implementation
title: Setup Wizard - Implementation Guide
description: Technical details for implementing the setup wizard
---

# Implementation: Setup Wizard

## Files to Modify

| File | Change |
|---|---|
| `go/internal/cmd/setup.go` | Rewrite `runSetup`, add prompt helpers, add `--project` flag |
| `go/internal/cmd/setup_test.go` | New file: unit + integration tests for wizard |

## Prompt Helpers

### `promptBool`

```go
func promptBool(r *bufio.Reader, label string, defaultVal bool) bool {
    if defaultVal {
        fmt.Printf("%s [Y/n]: ", label)
    } else {
        fmt.Printf("%s [y/N]: ", label)
    }
    val, _ := r.ReadString('\n')
    val = strings.TrimSpace(strings.ToLower(val))
    if val == "" {
        return defaultVal
    }
    return val == "y" || val == "yes"
}
```

### `promptInt`

```go
func promptInt(r *bufio.Reader, label string, defaultVal int) int {
    fmt.Printf("%s [%d]: ", label, defaultVal)
    val, _ := r.ReadString('\n')
    val = strings.TrimSpace(val)
    if val == "" {
        return defaultVal
    }
    n, err := strconv.Atoi(val)
    if err != nil {
        fmt.Println("  Invalid number, using default:", defaultVal)
        return defaultVal
    }
    return n
}
```

### `promptPassword`

```go
func promptPassword(label, current string) string {
    if current != "" {
        fmt.Printf("%s [****]: ", label)
    } else {
        fmt.Printf("%s: ", label)
    }
    b, err := term.ReadPassword(int(syscall.Stdin))
    fmt.Println() // newline after masked input
    val := strings.TrimSpace(string(b))
    if err != nil || val == "" {
        return current
    }
    return val
}
```

## Wizard Step Structure

Each step prints a header and groups related prompts. Steps 2-3 are conditional on AI Review; step 4 is conditional on SonarQube.

```go
// Step 1: Feature Flags (always shown)
display.Bold.Println("── Step 1: Feature Flags ──")
cfg.EnableAIReview = promptBool(reader, "Enable AI Review?", cfg.EnableAIReview)
cfg.EnableSonarQube = promptBool(reader, "Enable SonarQube Review?", cfg.EnableSonarQube)

// Step 2 & 3: only if AI Review enabled
if cfg.EnableAIReview {
    display.Bold.Println("── Step 2: AI Gateway ──")
    cfg.AIGatewayURL = promptStringRequired(reader, "AI Gateway URL", cfg.AIGatewayURL)
    cfg.AIGatewayAPIKey = promptPasswordRequired("AI Gateway API Key", cfg.AIGatewayAPIKey)
    cfg.AIModel = promptString(reader, "AI Model", cfg.AIModel, false)
    cfg.AIProvider = promptString(reader, "AI Provider", cfg.AIProvider, false)

    display.Bold.Println("── Step 3: Gateway Behaviour ──")
    cfg.BlockOnGatewayError = promptBool(reader, "Block commit on gateway error?", cfg.BlockOnGatewayError)
    cfg.GatewayTimeoutSec = promptInt(reader, "Gateway timeout (seconds)", cfg.GatewayTimeoutSec)
}

// Step 4: only if SonarQube enabled
if cfg.EnableSonarQube {
    display.Bold.Println("── Step 4: SonarQube Settings ──")
    cfg.SonarHostURL = promptStringRequired(reader, "SonarQube Host URL", cfg.SonarHostURL)
    cfg.SonarToken = promptPasswordRequired("SonarQube Token", cfg.SonarToken)
    cfg.SonarProjectKey = promptStringRequired(reader, "SonarQube Project Key", cfg.SonarProjectKey)
    cfg.SonarBlockHotspots = promptBool(reader, "Block commit on security hotspots?", cfg.SonarBlockHotspots)
    cfg.SonarFilterChanged = promptBool(reader, "Filter changed lines only?", cfg.SonarFilterChanged)
}
```

### `promptStringRequired` (re-prompts until non-empty)

```go
func promptStringRequired(r *bufio.Reader, label, current string) string {
    for {
        val := promptString(r, label, current, true)
        if val != "" {
            return val
        }
        fmt.Printf("  %s is required. Please enter a value.\n", label)
    }
}
```

### `promptPasswordRequired` (re-prompts until non-empty)

```go
func promptPasswordRequired(label, current string) string {
    for {
        val := promptPassword(label, current)
        if val != "" {
            return val
        }
        fmt.Printf("  %s is required. Please enter a value.\n", label)
    }
}
```

## Summary Display

```go
func printSummary(cfg *config.Config) {
    display.PrintSeparator()
    fmt.Printf("  %-35s %s\n", "ENABLE_AI_REVIEW", boolStr(cfg.EnableAIReview))
    fmt.Printf("  %-35s %s\n", "ENABLE_SONARQUBE_LOCAL", boolStr(cfg.EnableSonarQube))
    fmt.Printf("  %-35s %s\n", "AI_GATEWAY_URL", orNotSet(cfg.AIGatewayURL))
    fmt.Printf("  %-35s %s\n", "AI_GATEWAY_API_KEY", maskIfSet(cfg.AIGatewayAPIKey))
    fmt.Printf("  %-35s %s\n", "AI_MODEL", orNotSet(cfg.AIModel))
    fmt.Printf("  %-35s %s\n", "AI_PROVIDER", orNotSet(cfg.AIProvider))
    fmt.Printf("  %-35s %s\n", "BLOCK_ON_GATEWAY_ERROR", boolStr(cfg.BlockOnGatewayError))
    fmt.Printf("  %-35s %d\n", "GATEWAY_TIMEOUT_SEC", cfg.GatewayTimeoutSec)
    if cfg.EnableSonarQube {
        fmt.Printf("  %-35s %s\n", "SONAR_HOST_URL", orNotSet(cfg.SonarHostURL))
        fmt.Printf("  %-35s %s\n", "SONAR_TOKEN", maskIfSet(cfg.SonarToken))
        fmt.Printf("  %-35s %s\n", "SONAR_PROJECT_KEY", orNotSet(cfg.SonarProjectKey))
        fmt.Printf("  %-35s %s\n", "SONAR_BLOCK_ON_HOTSPOTS", boolStr(cfg.SonarBlockHotspots))
        fmt.Printf("  %-35s %s\n", "SONAR_FILTER_CHANGED_LINES_ONLY", boolStr(cfg.SonarFilterChanged))
    }
    display.PrintSeparator()
}
```

## `--project` Flag

Declare a separate flag variable (do NOT reuse `configProjectFlag` from config.go):

```go
var setupProjectFlag bool

func init() {
    setupCmd.Flags().BoolVar(&setupProjectFlag, "project", false, "save to project config (requires git repo)")
    rootCmd.AddCommand(setupCmd)
}
```

Save logic at the end of `runSetup`:

```go
if setupProjectFlag {
    // Save each field to project config
    for _, key := range config.AllConfigKeys() {
        val := config.GetField(cfg, key)
        if err := config.SaveProjectField(key, val); err != nil {
            return fmt.Errorf("save project config: %w", err)
        }
    }
} else {
    if err := config.Save(cfg); err != nil {
        return fmt.Errorf("save config: %w", err)
    }
}
```

## Testing Strategy

Tests use a `strings.NewReader` to simulate stdin input. The `readPassword` function should be extracted as a package-level var so tests can override it:

```go
var readPasswordFn = func() (string, error) {
    return term.ReadPassword(int(syscall.Stdin))
}
```

Tests override: `readPasswordFn = func() (string, error) { return "test-key", nil }`
