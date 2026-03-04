package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/hiiamtrong/smart-code-review/internal/config"
	"github.com/hiiamtrong/smart-code-review/internal/display"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure AI Gateway credentials",
	Long:  "Interactively configure your AI Gateway URL, API key, and model settings.",
	RunE:  runSetup,
}

func init() {
	rootCmd.AddCommand(setupCmd)
}

func runSetup(cmd *cobra.Command, args []string) error {
	cfg, _ := config.LoadMerged()
	if cfg == nil {
		cfg = config.Defaults()
	}

	reader := bufio.NewReader(os.Stdin)

	display.Bold.Println("AI Review Setup")
	display.PrintSeparator()
	fmt.Println()

	// AI Gateway URL
	cfg.AIGatewayURL = promptString(reader, "AI Gateway URL", cfg.AIGatewayURL, true)

	// AI Gateway API Key (masked input)
	fmt.Print("AI Gateway API Key")
	if cfg.AIGatewayAPIKey != "" {
		fmt.Print(" [****]: ")
	} else {
		fmt.Print(" (required): ")
	}
	key, err := readPassword()
	if err != nil || strings.TrimSpace(key) == "" {
		if cfg.AIGatewayAPIKey == "" {
			return fmt.Errorf("AI_GATEWAY_API_KEY is required")
		}
		// Keep existing key
	} else {
		cfg.AIGatewayAPIKey = strings.TrimSpace(key)
	}
	fmt.Println()

	// AI Model
	cfg.AIModel = promptString(reader, "AI Model", cfg.AIModel, false)
	if cfg.AIModel == "" {
		cfg.AIModel = "gemini-2.0-flash"
	}

	// AI Provider
	cfg.AIProvider = promptString(reader, "AI Provider", cfg.AIProvider, false)
	if cfg.AIProvider == "" {
		cfg.AIProvider = "google"
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Println()
	display.LogSuccess("Configuration saved to " + config.FilePath())
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  Navigate to a git repo and run: ai-review install")
	return nil
}

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

func readPassword() (string, error) {
	b, err := term.ReadPassword(int(syscall.Stdin))
	return string(b), err
}
