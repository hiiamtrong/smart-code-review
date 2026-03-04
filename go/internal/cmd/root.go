package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

var rootCmd = &cobra.Command{
	Use:     "ai-review",
	Short:   "AI-powered code review tool",
	Long:    "ai-review provides AI-powered code review as a pre-commit hook and CI integration.",
	Version: Version,
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		if errors.Is(err, errBlocked) {
			// Message already printed by display helpers; just exit.
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetVersionTemplate("ai-review version {{.Version}}\n")
}
