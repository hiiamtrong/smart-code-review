package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hiiamtrong/smart-code-review/internal/config"
	"github.com/hiiamtrong/smart-code-review/internal/display"
	"github.com/hiiamtrong/smart-code-review/internal/filter"
	"github.com/hiiamtrong/smart-code-review/internal/gateway"
	"github.com/hiiamtrong/smart-code-review/internal/git"
	"github.com/hiiamtrong/smart-code-review/internal/language"
	"github.com/hiiamtrong/smart-code-review/internal/sonarqube"
)

var runHookCmd = &cobra.Command{
	Use:    "run-hook",
	Short:  "Execute pre-commit review logic (called by git hook)",
	Hidden: true,
	RunE:   runHook,
}

func init() {
	rootCmd.AddCommand(runHookCmd)
}

func runHook(cmd *cobra.Command, args []string) error {
	display.PrintHeader(Version)

	cfg, err := config.LoadWithRepoOverrides()
	if err != nil {
		display.LogWarn("config not found — run 'ai-review setup' to configure")
		return nil
	}

	if !cfg.EnableAIReview {
		display.LogInfo("AI review disabled (ENABLE_AI_REVIEW=false)")
		return nil
	}

	if cfg.AIGatewayURL == "" || cfg.AIGatewayAPIKey == "" {
		display.LogWarn("AI Gateway not configured — skipping review (run 'ai-review setup')")
		return nil
	}

	// ── 1. Get staged diff ────────────────────────────────────────────────────

	diff, err := git.GetStagedDiff()
	if err != nil {
		display.LogWarn(fmt.Sprintf("could not get staged diff: %v", err))
		return nil
	}
	if strings.TrimSpace(diff) == "" {
		display.LogInfo("No staged changes to review")
		return nil
	}

	// ── 2. Filter ignored files ───────────────────────────────────────────────

	repoRoot, err := git.GetRepoRoot()
	if err != nil {
		repoRoot = "."
	}

	ignorePath := filepath.Join(repoRoot, ".aireviewignore")
	patterns, _ := filter.LoadIgnorePatterns(ignorePath) // missing file is fine
	filteredDiff, ignoredCount := filter.FilterDiff(diff, patterns)

	if ignoredCount > 0 {
		display.LogInfo(fmt.Sprintf("Skipped %d ignored file(s)", ignoredCount))
	}

	if strings.TrimSpace(filteredDiff) == "" {
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
	display.LogInfo(fmt.Sprintf("Detected language: %s", lang))

	// ── 5. Collect git metadata ───────────────────────────────────────────────

	gitInfo, err := git.GetGitInfo()
	if err != nil {
		gitInfo = git.GitInfo{CommitHash: "staged"}
	}

	// Counters shared across SonarQube and AI review sections.
	var (
		errCount  int
		warnCount int
		infoCount int
	)

	// ── 6. SonarQube analysis (optional, runs before AI review) ──────────────

	if cfg.EnableSonarQube && cfg.SonarToken != "" {
		display.Divider()
		display.LogInfo("Running SonarQube analysis...")

		scannerBin, sonarErr := sonarqube.FindScanner()
		if sonarErr != nil {
			display.LogWarn(fmt.Sprintf("SonarQube scanner not found: %v", sonarErr))
		} else {
			projectKey := cfg.SonarProjectKey
			if projectKey == "" {
				projectKey, _ = git.GetLocalConfig("aireview.sonarProjectKey")
			}
			if _, propErr := sonarqube.AutoGenerateProperties(repoRoot, projectKey); propErr != nil {
				display.LogWarn(fmt.Sprintf("sonar-project.properties: %v", propErr))
			}

			// Build list of staged files for narrowed scanning.
			stagedFiles := extractStagedFiles(diff)

			sonarCfg := sonarqube.SonarConfig{
				HostURL:       cfg.SonarHostURL,
				Token:         cfg.SonarToken,
				ProjectKey:    projectKey,
				FilterChanged: cfg.SonarFilterChanged,
				BlockHotspots: cfg.SonarBlockHotspots,
			}

			if runErr := sonarqube.RunAnalysis(scannerBin, sonarCfg, stagedFiles); runErr != nil {
				display.LogWarn(fmt.Sprintf("SonarQube analysis failed: %v", runErr))
			} else {
				_ = sonarqube.WaitForTask(cfg.SonarHostURL, cfg.SonarToken, repoRoot, false)

				changedRanges := sonarqube.ParseStagedLineRanges(diff)
				sonarResult, fetchErr := sonarqube.FetchResults(sonarCfg, changedRanges)
				if fetchErr != nil {
					display.LogWarn(fmt.Sprintf("fetch SonarQube results: %v", fetchErr))
				} else {
					if sonarResult.Truncated {
						display.LogWarn("SonarQube: result set may be incomplete (over 500 issues); review SonarQube dashboard for full list")
					}
					for _, d := range sonarResult.Diagnostics {
						display.PrintIssue(d.Severity, d.Location.Path, d.Location.Range.Start.Line, d.Message)
						switch d.Severity {
						case "ERROR":
							errCount++
						case "WARNING":
							warnCount++
						default:
							infoCount++
						}
					}
					if sonarResult.HotspotCount > 0 {
						display.LogWarn(fmt.Sprintf("SonarQube: %d security hotspot(s) require review", sonarResult.HotspotCount))
						if cfg.SonarBlockHotspots {
							display.LogError("Commit blocked: review security hotspots in SonarQube dashboard")
							os.Exit(1)
						}
					}
					if errCount > 0 {
						display.LogError("Commit blocked by SonarQube errors")
						os.Exit(1)
					}
				}
			}
		}
	}

	// ── 7. Call AI Gateway (streaming) ────────────────────────────────────────

	display.Divider()
	display.LogInfo("Running AI code review...")
	display.Divider()

	payload := gateway.ReviewPayload{
		Diff:       annotatedDiff,
		Language:   lang,
		GitInfo:    gitInfo,
		AIModel:    cfg.AIModel,
		AIProvider: cfg.AIProvider,
	}

	onDiagnostic := func(d gateway.Diagnostic) {
		display.PrintIssue(d.Severity, d.Location.Path, d.Location.Range.Start.Line, d.Message)
		switch d.Severity {
		case "ERROR":
			errCount++
		case "WARNING":
			warnCount++
		default:
			infoCount++
		}
	}

	result, reviewErr := gateway.StreamingReview(context.Background(), cfg, payload, onDiagnostic)

	if reviewErr != nil {
		display.LogWarn(fmt.Sprintf("AI Gateway error: %v", reviewErr))
		if cfg.BlockOnGatewayError {
			display.LogError("Blocking commit due to gateway error (set BLOCK_ON_GATEWAY_ERROR=false to skip)")
			os.Exit(1)
		}
		return nil
	}

	// Recount from result if streaming callback did not fire (sync fallback path).
	if errCount+warnCount+infoCount == 0 && result != nil {
		for _, d := range result.Diagnostics {
			display.PrintIssue(d.Severity, d.Location.Path, d.Location.Range.Start.Line, d.Message)
			switch d.Severity {
			case "ERROR":
				errCount++
			case "WARNING":
				warnCount++
			default:
				infoCount++
			}
		}
	}

	// ── 8. Display overview ───────────────────────────────────────────────────

	if result != nil && result.Overview != "" {
		display.Divider()
		fmt.Println(result.Overview)
	}

	// ── 9. Summary & exit code ────────────────────────────────────────────────

	display.Divider()
	display.PrintSummary(errCount, warnCount, infoCount)
	display.Divider()

	if errCount > 0 {
		display.LogError("Commit blocked: fix the errors above before committing")
		os.Exit(1)
	}

	if errCount == 0 && warnCount == 0 && infoCount == 0 {
		display.LogSuccess("No issues found")
	} else {
		display.LogSuccess("Review complete")
	}

	return nil
}

// extractStagedFiles parses a unified diff and returns the list of new/modified filenames.
func extractStagedFiles(diff string) []string {
	var files []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			parts := strings.Fields(line)
			if len(parts) >= 4 {
				f := strings.TrimPrefix(parts[3], "b/")
				if !seen[f] {
					seen[f] = true
					files = append(files, f)
				}
			}
		}
	}
	return files
}
