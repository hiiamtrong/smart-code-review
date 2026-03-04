package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/hiiamtrong/smart-code-review/internal/config"
	"github.com/hiiamtrong/smart-code-review/internal/display"
	"github.com/hiiamtrong/smart-code-review/internal/filter"
	"github.com/hiiamtrong/smart-code-review/internal/gateway"
	"github.com/hiiamtrong/smart-code-review/internal/git"
	"github.com/hiiamtrong/smart-code-review/internal/language"
	"github.com/hiiamtrong/smart-code-review/internal/reviewdog"
)

var (
	ciOutputFile   string
	ciOverviewFile string
	ciReporter     string
)

var ciReviewCmd = &cobra.Command{
	Use:   "ci-review",
	Short: "Execute CI/PR review logic (GitHub Actions)",
	RunE:  runCIReview,
}

func init() {
	ciReviewCmd.Flags().StringVar(&ciOutputFile, "output", "ai-output.jsonl", "rdjson output file path")
	ciReviewCmd.Flags().StringVar(&ciOverviewFile, "overview", "ai-overview.txt", "overview text output file path")
	ciReviewCmd.Flags().StringVar(&ciReporter, "reporter", "github-pr-review", "reviewdog reporter (github-pr-review|github-check|local)")
	rootCmd.AddCommand(ciReviewCmd)
}

func runCIReview(cmd *cobra.Command, args []string) error {
	cfg, err := config.LoadMerged()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if !cfg.EnableAIReview {
		display.LogInfo("AI review disabled (ENABLE_AI_REVIEW=false)")
		return nil
	}

	if cfg.AIGatewayURL == "" || cfg.AIGatewayAPIKey == "" {
		return fmt.Errorf("AI Gateway not configured — run 'ai-review setup'")
	}

	// ── 1. Determine PR diff ──────────────────────────────────────────────────

	baseBranch := os.Getenv("GITHUB_BASE_REF") // set for PR events
	display.LogInfo(fmt.Sprintf("Base branch: %q", baseBranch))

	diff, err := git.GetPRDiff(baseBranch)
	if err != nil {
		return fmt.Errorf("get PR diff: %w", err)
	}
	if diff == "" {
		display.LogInfo("No diff to review")
		return nil
	}

	// ── 2. Filter ignored files ───────────────────────────────────────────────

	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		repoRoot = "."
	}

	patterns, _ := filter.LoadIgnorePatterns(repoRoot + "/.aireviewignore")
	filteredDiff, ignoredCount := filter.FilterDiff(diff, patterns)
	if ignoredCount > 0 {
		display.LogInfo(fmt.Sprintf("Skipped %d ignored file(s)", ignoredCount))
	}
	if filteredDiff == "" {
		display.LogInfo("All changed files are ignored — skipping review")
		return nil
	}

	// ── 3. Annotate line numbers ──────────────────────────────────────────────

	annotatedDiff := git.AnnotateLineNumbers(filteredDiff)

	// ── 4. Detect language ────────────────────────────────────────────────────

	lang := language.DetectFromDiff(annotatedDiff)
	if lang == "unknown" {
		lang = language.DetectFromProject(repoRoot)
	}
	display.LogInfo(fmt.Sprintf("Language: %s", lang))

	// ── 5. Git metadata ───────────────────────────────────────────────────────

	gitInfo, _ := git.GetGitInfo()

	// ── 6. Call AI Gateway (sync — streaming not needed in CI) ────────────────

	display.LogInfo("Running AI code review (sync)...")

	payload := gateway.ReviewPayload{
		Diff:       annotatedDiff,
		Language:   lang,
		GitInfo:    gitInfo,
		AIModel:    cfg.AIModel,
		AIProvider: cfg.AIProvider,
	}

	result, err := gateway.SyncReview(context.Background(), cfg, payload)
	if err != nil {
		return fmt.Errorf("AI Gateway: %w", err)
	}

	display.LogInfo(fmt.Sprintf("Review complete: %d diagnostic(s)", len(result.Diagnostics)))

	// ── 7-9. Write outputs, post PR comment, invoke reviewdog ────────────────

	if err := ciWriteOutputs(result, ciOutputFile, ciOverviewFile); err != nil {
		return err
	}
	ciPostPRComment(result, gitInfo)
	ciRunReviewdog(result, ciOutputFile, ciReporter)

	return nil
}

func ciWriteOutputs(result *gateway.ReviewResult, outputFile, overviewFile string) error {
	if err := reviewdog.WriteRDJSON(result, outputFile); err != nil {
		return fmt.Errorf("write rdjson: %w", err)
	}
	display.LogSuccess(fmt.Sprintf("Wrote rdjson to %s", outputFile))

	if result.Overview != "" {
		if err := reviewdog.WriteOverview(result, overviewFile); err != nil {
			return fmt.Errorf("write overview: %w", err)
		}
		display.LogSuccess(fmt.Sprintf("Wrote overview to %s", overviewFile))
	}
	return nil
}

func ciPostPRComment(result *gateway.ReviewResult, gitInfo git.GitInfo) {
	ghToken := os.Getenv("GITHUB_TOKEN")
	ghRepo := os.Getenv("GITHUB_REPOSITORY")
	prNumber := gitInfo.PRNumber

	if ghToken == "" || ghRepo == "" || prNumber == "" || result.Overview == "" {
		return
	}

	display.LogInfo("Posting overview comment on PR...")
	if err := reviewdog.PostOverviewComment(ghToken, ghRepo, prNumber, result.Overview); err != nil {
		display.LogWarn(fmt.Sprintf("Could not post PR comment: %v", err))
	} else {
		display.LogSuccess("Posted overview comment")
	}
}

func ciRunReviewdog(result *gateway.ReviewResult, outputFile, reporter string) {
	if err := reviewdog.InvokeReviewdog(outputFile, reporter); err != nil {
		display.LogWarn(fmt.Sprintf("reviewdog: %v", err))
		for _, d := range result.Diagnostics {
			if d.Severity == "ERROR" {
				os.Exit(1)
			}
		}
	}
}
