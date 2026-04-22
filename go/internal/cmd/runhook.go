package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hiiamtrong/smart-code-review/internal/config"
	"github.com/hiiamtrong/smart-code-review/internal/display"
	"github.com/hiiamtrong/smart-code-review/internal/filter"
	"github.com/hiiamtrong/smart-code-review/internal/gateway"
	"github.com/hiiamtrong/smart-code-review/internal/git"
	"github.com/hiiamtrong/smart-code-review/internal/language"
	"github.com/hiiamtrong/smart-code-review/internal/semgrep"
	"github.com/hiiamtrong/smart-code-review/internal/sonarqube"
)

// errBlocked is returned when the commit should be blocked.
// return errBlocked bypasses defer, so we return this error instead
// to let deferred cleanup (e.g. SonarQube artifacts) run.
var errBlocked = errors.New("")

var runHookCmd = &cobra.Command{
	Use:          "run-hook",
	Short:        "Execute pre-commit review logic (called by git hook)",
	Hidden:       true,
	SilenceUsage: true,
	RunE:         runHook,
}

func init() {
	rootCmd.AddCommand(runHookCmd)
}

// hookCounts tracks diagnostic counts across review stages.
type hookCounts struct {
	errCount, warnCount, infoCount int
	blocked                        bool
}

func runHook(cmd *cobra.Command, args []string) error {
	display.PrintHeader(Version)

	cfg, err := config.LoadMerged()
	if err != nil {
		display.LogWarn("config not found — run 'ai-review setup' to configure")
		return nil
	}

	diff, annotatedDiff, lang, repoRoot, gitInfo := hookPrepareDiff()
	if diff == "" {
		return nil
	}

	var counts hookCounts
	var stages []display.StageSummary
	var blocked bool
	var result *gateway.ReviewResult

	addCounts := func(name string, c hookCounts) {
		counts.errCount += c.errCount
		counts.warnCount += c.warnCount
		counts.infoCount += c.infoCount
		stage := display.StageSummary{Name: name, Errors: c.errCount, Warnings: c.warnCount, Infos: c.infoCount}
		stages = append(stages, stage)
		display.PrintStageSummary(stage)
		if c.blocked {
			blocked = true
			counts.blocked = true
		}
	}

	// Stage 1: Semgrep (local static analysis — fastest)
	if cfg.EnableSemgrep {
		addCounts("Semgrep", hookRunSemgrep(cfg, repoRoot, diff))
	}

	// Stage 2: SonarQube (server-based analysis)
	if !blocked && cfg.EnableSonarQube && cfg.SonarToken != "" {
		addCounts("SonarQube", hookRunSonarQube(cfg, repoRoot, diff))
	}

	// Stage 3: AI code review
	if !blocked && cfg.EnableAIReview {
		if cfg.AIGatewayURL == "" || cfg.AIGatewayAPIKey == "" {
			display.LogWarn("AI Gateway not configured — skipping review (run 'ai-review setup')")
		} else {
			var aiCounts hookCounts
			aiCounts, result = hookRunAIReview(cfg, annotatedDiff, lang, gitInfo)
			addCounts("AI", aiCounts)
		}
	}

	return hookFinalize(result, counts, stages)
}

// hookPrepareDiff gets the staged diff, filters ignored files, annotates line
// numbers, detects language, and collects git metadata. Returns empty rawDiff
// when there is nothing to review.
func hookPrepareDiff() (rawDiff, annotated, lang, repoRoot string, gitInfo git.GitInfo) {
	diff, err := git.GetStagedDiff()
	if err != nil {
		display.LogWarn(fmt.Sprintf("could not get staged diff: %v", err))
		return
	}
	if strings.TrimSpace(diff) == "" {
		display.LogInfo("No staged changes to review")
		return
	}

	repoRoot, err = git.GetRepoRoot()
	if err != nil {
		repoRoot = "."
	}

	ignorePath := filepath.Join(repoRoot, ".aireviewignore")
	patterns, _ := filter.LoadIgnorePatterns(ignorePath)
	filteredDiff, ignoredCount := filter.FilterDiff(diff, patterns)

	if ignoredCount > 0 {
		display.LogInfo(fmt.Sprintf("Skipped %d ignored file(s)", ignoredCount))
	}

	if strings.TrimSpace(filteredDiff) == "" {
		display.LogInfo("All changed files are ignored — skipping review")
		return
	}

	annotated = git.AnnotateLineNumbers(filteredDiff)
	lang = language.DetectFromDiff(annotated)
	if lang == "unknown" {
		lang = language.DetectFromProject(repoRoot)
	}
	display.LogInfo(fmt.Sprintf("Detected language: %s", lang))

	gitInfo, err = git.GetGitInfo()
	if err != nil {
		gitInfo = git.GitInfo{CommitHash: "staged"}
	}

	rawDiff = diff
	return
}

// hookRunSonarQube runs the full SonarQube analysis pipeline and returns
// diagnostic counts and whether the commit should be blocked.
func hookRunSonarQube(cfg *config.Config, repoRoot, diff string) hookCounts {
	display.PrintStageHeader("SonarQube Analysis")

	var counts hookCounts

	scannerBin, err := sonarqube.FindScanner()
	if err != nil {
		display.LogWarn(fmt.Sprintf("SonarQube scanner not found: %v", err))
		return counts
	}

	projectKey := cfg.SonarProjectKey
	if projectKey == "" {
		projectKey, _ = git.GetLocalConfig("aireview.sonarProjectKey")
	}
	_, propsCreated, propErr := sonarqube.AutoGenerateProperties(repoRoot, projectKey)
	if propErr != nil {
		display.LogWarn(fmt.Sprintf("sonar-project.properties: %v", propErr))
	}
	defer sonarqube.Cleanup(repoRoot, propsCreated)

	stagedFiles := extractStagedFiles(diff)
	sonarCfg := sonarqube.SonarConfig{
		HostURL:       cfg.SonarHostURL,
		Token:         cfg.SonarToken,
		ProjectKey:    projectKey,
		RepoRoot:      repoRoot,
		FilterChanged: cfg.SonarFilterChanged,
		BlockHotspots: cfg.SonarBlockHotspots,
	}

	if runErr := sonarqube.RunAnalysis(scannerBin, sonarCfg, stagedFiles); runErr != nil {
		display.LogWarn(fmt.Sprintf("SonarQube analysis failed: %v", runErr))
		return counts
	}

	_ = sonarqube.WaitForTask(cfg.SonarHostURL, cfg.SonarToken, repoRoot, false)

	changedRanges := sonarqube.ParseStagedLineRanges(diff)
	sonarRes, fetchErr := sonarqube.FetchResults(sonarCfg, changedRanges)
	if fetchErr != nil {
		display.LogWarn(fmt.Sprintf("fetch SonarQube results: %v", fetchErr))
		return counts
	}

	if sonarRes.Truncated {
		display.LogWarn("SonarQube: result set may be incomplete (over 500 issues); review SonarQube dashboard for full list")
	}

	for _, d := range sonarRes.Diagnostics {
		display.PrintIssueWithSource("SonarQube", d.Severity, d.Location.Path, d.Location.Range.Start.Line, d.Message)
		switch d.Severity {
		case "ERROR":
			counts.errCount++
		case "WARNING":
			counts.warnCount++
		default:
			counts.infoCount++
		}
	}

	if sonarRes.HotspotCount > 0 {
		display.LogWarn(fmt.Sprintf("SonarQube: %d security hotspot(s) require review", sonarRes.HotspotCount))
		if cfg.SonarBlockHotspots {
			display.LogError("Commit blocked: review security hotspots in SonarQube dashboard")
			counts.blocked = true
			return counts
		}
	}

	if counts.errCount > 0 {
		display.LogError("Commit blocked by SonarQube errors")
		counts.blocked = true
	}

	return counts
}

// hookRunSemgrep runs Semgrep on staged files and returns diagnostic counts.
func hookRunSemgrep(cfg *config.Config, repoRoot, diff string) hookCounts {
	display.PrintStageHeader("Semgrep Analysis")

	var counts hookCounts

	bin, err := semgrep.FindSemgrep()
	if err != nil {
		display.LogWarn(fmt.Sprintf("Semgrep not found: %v", err))
		return counts
	}

	stagedFiles := extractStagedFiles(diff)
	if len(stagedFiles) == 0 {
		return counts
	}

	sgCfg := semgrep.SemgrepConfig{
		Rules: cfg.SemgrepRules,
	}

	res, scanErr := semgrep.ScanFiles(bin, sgCfg, stagedFiles, repoRoot)
	if scanErr != nil {
		display.LogWarn(fmt.Sprintf("Semgrep scan failed: %v", scanErr))
		return counts
	}

	for _, d := range res.Diagnostics {
		display.PrintIssueWithSource("Semgrep", d.Severity, d.Location.Path, d.Location.Range.Start.Line, d.Message)
		switch d.Severity {
		case "ERROR":
			counts.errCount++
		case "WARNING":
			counts.warnCount++
		default:
			counts.infoCount++
		}
	}

	if counts.errCount > 0 {
		display.LogError("Commit blocked by Semgrep errors")
		counts.blocked = true
	}

	return counts
}

// hookRunAIReview calls the AI gateway for code review and returns diagnostic
// counts plus the result. A nil result means a non-blocking gateway error.
func hookRunAIReview(cfg *config.Config, annotatedDiff, lang string, gitInfo git.GitInfo) (hookCounts, *gateway.ReviewResult) {
	display.PrintStageHeader("AI Analysis")

	var counts hookCounts

	payload := gateway.ReviewPayload{
		Diff:       annotatedDiff,
		Language:   lang,
		GitInfo:    gitInfo,
		AIModel:    cfg.AIModel,
		AIProvider: cfg.AIProvider,
	}

	onDiagnostic := func(d gateway.Diagnostic) {
		display.PrintIssueWithSource("AI", d.Severity, d.Location.Path, d.Location.Range.Start.Line, d.Message)
		switch d.Severity {
		case "ERROR":
			counts.errCount++
		case "WARNING":
			counts.warnCount++
		default:
			counts.infoCount++
		}
	}

	result, reviewErr := gateway.StreamingReview(context.Background(), cfg, payload, onDiagnostic)

	if reviewErr != nil {
		display.LogWarn(fmt.Sprintf("AI Gateway error: %v", reviewErr))
		if cfg.BlockOnGatewayError {
			display.LogError("Blocking commit due to gateway error (set BLOCK_ON_GATEWAY_ERROR=false to skip)")
			counts.blocked = true
		}
		return counts, nil
	}

	// Recount from result if streaming callback did not fire (sync fallback path).
	if counts.errCount+counts.warnCount+counts.infoCount == 0 && result != nil {
		for _, d := range result.Diagnostics {
			display.PrintIssueWithSource("AI", d.Severity, d.Location.Path, d.Location.Range.Start.Line, d.Message)
			switch d.Severity {
			case "ERROR":
				counts.errCount++
			case "WARNING":
				counts.warnCount++
			default:
				counts.infoCount++
			}
		}
	}

	return counts, result
}

// hookFinalize displays the overview and summary, then decides whether to
// block the commit.
func hookFinalize(result *gateway.ReviewResult, counts hookCounts, stages []display.StageSummary) error {
	if result != nil && result.Overview != "" {
		display.Divider()
		fmt.Println(result.Overview)
	}

	display.Divider()
	display.PrintStageSummaries(stages, counts.errCount, counts.warnCount, counts.infoCount)
	display.Divider()

	if counts.blocked || counts.errCount > 0 {
		display.LogError("Commit blocked: fix the errors above before committing")
		return errBlocked
	}

	if counts.errCount == 0 && counts.warnCount == 0 && counts.infoCount == 0 {
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
