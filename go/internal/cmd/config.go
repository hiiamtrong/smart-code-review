package cmd

import (
	"fmt"
	"strings"

	"github.com/hiiamtrong/smart-code-review/internal/config"
	"github.com/hiiamtrong/smart-code-review/internal/display"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config [get|set] [KEY] [VALUE]",
	Short: "View or modify configuration",
	Long: `View or modify ai-review configuration.

  ai-review config              Print all config values
  ai-review config get KEY      Print a single value
  ai-review config set KEY VAL  Set a value`,
	RunE: runConfig,
}

func init() {
	rootCmd.AddCommand(configCmd)
}

func runConfig(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	switch {
	case len(args) == 0:
		printAllConfig(cfg)
	case len(args) == 2 && args[0] == "get":
		val := config.GetField(cfg, args[1])
		if val == "" {
			return fmt.Errorf("unknown key: %s", args[1])
		}
		fmt.Println(val)
	case len(args) == 3 && args[0] == "set":
		if err := config.SetField(cfg, args[1], args[2]); err != nil {
			return err
		}
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		display.LogSuccess(fmt.Sprintf("Set %s", args[1]))
	default:
		return fmt.Errorf("usage: ai-review config [get KEY | set KEY VALUE]")
	}
	return nil
}

func printAllConfig(cfg *config.Config) {
	maskedKey := "****"
	if cfg.AIGatewayAPIKey == "" {
		maskedKey = "(not set)"
	}

	rows := []struct{ k, v string }{
		{"AI_GATEWAY_URL", orNotSet(cfg.AIGatewayURL)},
		{"AI_GATEWAY_API_KEY", maskedKey},
		{"AI_MODEL", cfg.AIModel},
		{"AI_PROVIDER", cfg.AIProvider},
		{"ENABLE_AI_REVIEW", boolStr(cfg.EnableAIReview)},
		{"ENABLE_SONARQUBE_LOCAL", boolStr(cfg.EnableSonarQube)},
		{"BLOCK_ON_GATEWAY_ERROR", boolStr(cfg.BlockOnGatewayError)},
		{"GATEWAY_TIMEOUT_SEC", fmt.Sprintf("%d", cfg.GatewayTimeoutSec)},
		{"SONAR_HOST_URL", orNotSet(cfg.SonarHostURL)},
		{"SONAR_TOKEN", maskIfSet(cfg.SonarToken)},
		{"SONAR_PROJECT_KEY", orNotSet(cfg.SonarProjectKey)},
	}

	display.PrintSeparator()
	for _, r := range rows {
		fmt.Printf("  %-30s %s\n", r.k, r.v)
	}
	display.PrintSeparator()
	fmt.Println()
	fmt.Println("Config file:", config.FilePath())
}

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
	return strings.ToLower(fmt.Sprintf("%t", b))
}
