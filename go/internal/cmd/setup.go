package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/hiiamtrong/smart-code-review/internal/config"
	"github.com/hiiamtrong/smart-code-review/internal/display"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var setupProjectFlag bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Interactive configuration wizard",
	Long:  "Interactively configure all AI Review settings including AI Gateway, SonarQube, and feature flags.",
	RunE:  runSetup,
}

func init() {
	setupCmd.Flags().BoolVar(&setupProjectFlag, "project", false, "save to project config (requires git repo)")
	rootCmd.AddCommand(setupCmd)
}

// readPasswordFn is a package-level var so tests can override it.
var readPasswordFn = func() (string, error) {
	b, err := term.ReadPassword(int(syscall.Stdin))
	return string(b), err
}

func runSetup(cmd *cobra.Command, args []string) error {
	cfg, _ := config.LoadMerged()
	if cfg == nil {
		cfg = config.Defaults()
	}

	reader := bufio.NewReader(os.Stdin)

	display.Bold.Println("AI Review Setup")
	display.PrintSeparator()

	// ── Step 1: Feature Flags ──
	fmt.Println()
	display.Bold.Println("── Step 1: Feature Flags ──")
	cfg.EnableAIReview = promptBool(reader, "Enable AI Review?", cfg.EnableAIReview)
	cfg.EnableSonarQube = promptBool(reader, "Enable SonarQube Review?", cfg.EnableSonarQube)
	cfg.EnableSemgrep = promptBool(reader, "Enable Semgrep Analysis?", cfg.EnableSemgrep)

	// ── Step 2: AI Gateway (conditional) ──
	if cfg.EnableAIReview {
		fmt.Println()
		display.Bold.Println("── Step 2: AI Gateway ──")
		cfg.AIGatewayURL = promptStringRequired(reader, "AI Gateway URL", cfg.AIGatewayURL)
		cfg.AIGatewayAPIKey = promptPasswordRequired("AI Gateway API Key", cfg.AIGatewayAPIKey)
		cfg.AIModel = promptString(reader, "AI Model", cfg.AIModel, false)
		if cfg.AIModel == "" {
			cfg.AIModel = "gemini-2.0-flash"
		}
		cfg.AIProvider = promptString(reader, "AI Provider", cfg.AIProvider, false)
		if cfg.AIProvider == "" {
			cfg.AIProvider = "google"
		}

		}

	// ── Step 3: SonarQube Settings (conditional) ──
	if cfg.EnableSonarQube {
		fmt.Println()
		display.Bold.Println("── Step 3: SonarQube Settings ──")
		cfg.SonarHostURL = promptStringRequired(reader, "SonarQube Host URL", cfg.SonarHostURL)
		cfg.SonarToken = promptPasswordRequired("SonarQube Token", cfg.SonarToken)
		if cfg.SonarProjectKey == "" {
			cfg.SonarProjectKey = detectRepoName()
		}
		cfg.SonarProjectKey = promptStringRequired(reader, "SonarQube Project Key", cfg.SonarProjectKey)
	}

	// ── Step 4: Semgrep Settings (conditional) ──
	if cfg.EnableSemgrep {
		fmt.Println()
		display.Bold.Println("── Step 4: Semgrep Settings ──")
		fmt.Println("  Rules config: 'auto' (auto-detect), 'p/default', or path to .semgrep.yml")
		cfg.SemgrepRules = promptString(reader, "Semgrep Rules", cfg.SemgrepRules, false)
		if cfg.SemgrepRules == "" {
			cfg.SemgrepRules = "auto"
		}
	}

	// ── Summary ──
	fmt.Println()
	display.Bold.Println("── Summary ──")
	printSetupSummary(cfg)
	fmt.Println()

	if !promptBool(reader, "Save configuration?", true) {
		display.LogInfo("Setup cancelled — no changes saved.")
		return nil
	}

	// Save
	if setupProjectFlag {
		for _, key := range config.AllConfigKeys() {
			val := config.GetField(cfg, key)
			if err := config.SaveProjectField(key, val); err != nil {
				return fmt.Errorf("save project config: %w", err)
			}
		}
		display.LogSuccess("Configuration saved to project config")
	} else {
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		display.LogSuccess("Configuration saved to " + config.FilePath())
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  Navigate to a git repo and run: ai-review install")
	return nil
}

// ── prompt helpers ──────────────────────────────────────────────────────────

func promptString(r *bufio.Reader, label, current string, required bool) string {
	if current != "" {
		fmt.Printf("%s [%s]: ", label, current)
	} else if required {
		fmt.Printf("%s (required): ", label)
	} else {
		fmt.Printf("%s: ", label)
	}
	val, _ := r.ReadString('\n')
	val = strings.TrimSpace(val)
	if val == "" {
		return current
	}
	return val
}

func promptStringRequired(r *bufio.Reader, label, current string) string {
	for {
		val := promptString(r, label, current, true)
		if val != "" {
			return val
		}
		fmt.Printf("  %s is required. Please enter a value.\n", label)
	}
}

func promptBool(r *bufio.Reader, label string, defaultVal bool) bool {
	if defaultVal {
		fmt.Printf("%s (default: yes) [Y/n]: ", label)
	} else {
		fmt.Printf("%s (default: no) [y/N]: ", label)
	}
	val, _ := r.ReadString('\n')
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "" {
		return defaultVal
	}
	return val == "y" || val == "yes"
}

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

func promptPassword(label, current string) string {
	if current != "" {
		fmt.Printf("%s [****]: ", label)
	} else {
		fmt.Printf("%s (required): ", label)
	}
	raw, err := readPasswordFn()
	fmt.Println() // newline after masked input
	val := strings.TrimSpace(raw)
	if err != nil || val == "" {
		return current
	}
	return val
}

func promptPasswordRequired(label, current string) string {
	for {
		val := promptPassword(label, current)
		if val != "" {
			return val
		}
		fmt.Printf("  %s is required. Please enter a value.\n", label)
	}
}

// ── summary display ─────────────────────────────────────────────────────────

func printSetupSummary(cfg *config.Config) {
	display.PrintSeparator()
	fmt.Printf("  %-35s %s\n", "ENABLE_AI_REVIEW", boolStr(cfg.EnableAIReview))
	fmt.Printf("  %-35s %s\n", "ENABLE_SONARQUBE_LOCAL", boolStr(cfg.EnableSonarQube))
	fmt.Printf("  %-35s %s\n", "ENABLE_SEMGREP", boolStr(cfg.EnableSemgrep))
	if cfg.EnableAIReview {
		fmt.Printf("  %-35s %s\n", "AI_GATEWAY_URL", orNotSet(cfg.AIGatewayURL))
		fmt.Printf("  %-35s %s\n", "AI_GATEWAY_API_KEY", maskIfSet(cfg.AIGatewayAPIKey))
		fmt.Printf("  %-35s %s\n", "AI_MODEL", orNotSet(cfg.AIModel))
		fmt.Printf("  %-35s %s\n", "AI_PROVIDER", orNotSet(cfg.AIProvider))
	}
	if cfg.EnableSonarQube {
		fmt.Printf("  %-35s %s\n", "SONAR_HOST_URL", orNotSet(cfg.SonarHostURL))
		fmt.Printf("  %-35s %s\n", "SONAR_TOKEN", maskIfSet(cfg.SonarToken))
		fmt.Printf("  %-35s %s\n", "SONAR_PROJECT_KEY", orNotSet(cfg.SonarProjectKey))
	}
	if cfg.EnableSemgrep {
		fmt.Printf("  %-35s %s\n", "SEMGREP_RULES", orNotSet(cfg.SemgrepRules))
	}
	display.PrintSeparator()
}

// detectRepoName returns the current git repo's directory name,
// falling back to the current working directory name.
func detectRepoName() string {
	if out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output(); err == nil {
		return filepath.Base(strings.TrimSpace(string(out)))
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Base(wd)
	}
	return ""
}
