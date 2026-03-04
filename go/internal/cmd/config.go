package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/hiiamtrong/smart-code-review/internal/config"
	"github.com/hiiamtrong/smart-code-review/internal/display"
	"github.com/spf13/cobra"
)

const errLoadConfig = "load config: %w"

var (
	configGlobalFlag  bool
	configProjectFlag bool
)

var configCmd = &cobra.Command{
	Use:   "config [get|set|list-projects|remove-project] [KEY] [VALUE]",
	Short: "View or modify configuration",
	Long: `View or modify ai-review configuration.

  ai-review config                          Print all config values (merged)
  ai-review config get KEY                  Print a single merged value
  ai-review config get KEY --global         Print global value only
  ai-review config set KEY VAL              Set a value (project if exists, else global)
  ai-review config set KEY VAL --global     Set in global config
  ai-review config set KEY VAL --project    Set in project config (requires git repo)
  ai-review config list-projects            List all projects with overrides
  ai-review config remove-project [ID]      Remove a project config`,
	RunE: runConfig,
}

func init() {
	configCmd.Flags().BoolVar(&configGlobalFlag, "global", false, "target global config only")
	configCmd.Flags().BoolVar(&configProjectFlag, "project", false, "target project config only (requires git repo)")
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	switch {
	case len(args) == 0:
		return runConfigShow()

	case args[0] == "get" && len(args) == 2:
		return runConfigGet(args[1])

	case args[0] == "set" && len(args) == 3:
		return runConfigSet(args[1], args[2])

	case args[0] == "list-projects":
		return runConfigListProjects()

	case args[0] == "remove-project":
		id := ""
		if len(args) >= 2 {
			id = args[1]
		}
		return runConfigRemoveProject(id)

	default:
		return fmt.Errorf("usage: ai-review config [get KEY | set KEY VALUE | list-projects | remove-project [ID]]")
	}
}

// ── config (no args) — show merged config with source annotations ───────────

var sensitiveKeys = map[string]bool{
	"AI_GATEWAY_API_KEY": true,
	"SONAR_TOKEN":        true,
}

func formatConfigValue(key, val string) string {
	if sensitiveKeys[key] {
		return maskIfSet(val)
	}
	return orNotSet(val)
}

func runConfigShow() error {
	sources, err := config.LoadMergedWithSources()
	if err != nil {
		return fmt.Errorf(errLoadConfig, err)
	}

	display.PrintSeparator()
	for _, key := range config.AllConfigKeys() {
		src := sources[key]
		fmt.Printf("  %-35s %-30s (%s)\n", key, formatConfigValue(key, src.Value), src.Source)
	}
	display.PrintSeparator()
	fmt.Println()
	fmt.Println("Global config: ", config.FilePath())
	printProjectInfo()
	return nil
}

func printProjectInfo() {
	projDir, _ := config.ProjectConfigDir()
	if projDir == "" {
		return
	}
	if _, err := os.Stat(projDir); err != nil {
		return
	}
	fmt.Println("Project config:", projDir+"/config")
	if b, err := os.ReadFile(projDir + "/repo-path"); err == nil {
		fmt.Println("Project repo:  ", strings.TrimSpace(string(b)))
	}
}

// ── config get KEY ──────────────────────────────────────────────────────────

func runConfigGet(key string) error {
	if configGlobalFlag {
		// Read global only.
		upperKey := strings.ToUpper(key)
		defaults := config.DefaultsAsMap()
		if _, known := defaults[upperKey]; !known {
			return fmt.Errorf("unknown key: %s", key)
		}
		val := defaults[upperKey]
		globalRaw, err := config.LoadGlobalRaw()
		if err != nil {
			return fmt.Errorf("load global config: %w", err)
		}
		if globalRaw != nil {
			if v, ok := globalRaw[upperKey]; ok {
				val = v
			}
		}
		fmt.Println(val)
		return nil
	}

	// Merged value.
	upperKey := strings.ToUpper(key)
	defaults := config.DefaultsAsMap()
	if _, known := defaults[upperKey]; !known {
		return fmt.Errorf("unknown key: %s", key)
	}
	cfg, err := config.LoadMerged()
	if err != nil {
		return fmt.Errorf(errLoadConfig, err)
	}
	fmt.Println(config.GetField(cfg, key))
	return nil
}

// ── config set KEY VALUE ────────────────────────────────────────────────────

func runConfigSet(key, value string) error {
	// Validate key/value.
	testCfg := config.Defaults()
	if err := config.SetField(testCfg, key, value); err != nil {
		return err
	}

	if configProjectFlag {
		// Always write to project config.
		if err := config.SaveProjectField(key, value); err != nil {
			return fmt.Errorf("save project config: %w", err)
		}
		display.LogSuccess(fmt.Sprintf("Set %s (project)", key))
		return nil
	}

	if configGlobalFlag {
		return saveToGlobal(key, value)
	}

	// Auto-detect: if project config already exists, update it.
	projRaw, _ := config.LoadProjectRaw()
	if projRaw != nil {
		if err := config.SaveProjectField(key, value); err != nil {
			return fmt.Errorf("save project config: %w", err)
		}
		display.LogSuccess(fmt.Sprintf("Set %s (project)", key))
		return nil
	}

	// Fallback: update global config.
	return saveToGlobal(key, value)
}

func saveToGlobal(key, value string) error {
	cfg, err := config.LoadMerged()
	if err != nil {
		return fmt.Errorf(errLoadConfig, err)
	}
	if err := config.SetField(cfg, key, value); err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	display.LogSuccess(fmt.Sprintf("Set %s (global)", key))
	return nil
}

// ── config list-projects ────────────────────────────────────────────────────

func runConfigListProjects() error {
	projects, err := config.ListProjects()
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}
	if len(projects) == 0 {
		display.LogInfo("No project configs found")
		return nil
	}

	display.PrintSeparator()
	for _, p := range projects {
		repoPath := p.RepoPath
		if repoPath == "" {
			repoPath = "(unknown)"
		}
		fmt.Printf("  %-14s %s\n", p.ID, repoPath)
	}
	display.PrintSeparator()
	return nil
}

// ── config remove-project ───────────────────────────────────────────────────

// ── display helpers (used in status.go and tests) ────────────────────────────

func orNotSet(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}

func maskIfSet(s string) string {
	if s == "" {
		return "(not set)"
	}
	return "****"
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func runConfigRemoveProject(id string) error {
	if err := config.RemoveProject(id); err != nil {
		return err
	}
	if id == "" {
		display.LogSuccess("Removed project config for current repo")
	} else {
		display.LogSuccess(fmt.Sprintf("Removed project config: %s", id))
	}
	return nil
}
