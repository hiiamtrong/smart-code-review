// Package display provides colored terminal output functions.
// Colors are automatically disabled when NO_COLOR is set or stdout is not a tty.
package display

import (
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
)

var (
	Bold    = color.New(color.Bold)
	red     = color.New(color.FgRed)
	green   = color.New(color.FgGreen)
	yellow  = color.New(color.FgYellow)
	blue    = color.New(color.FgBlue)
	cyan    = color.New(color.FgCyan)
)

func LogError(msg string) { red.Fprintln(os.Stderr, "[ERROR]", msg) }
func LogWarn(msg string)  { yellow.Println("[WARN]", msg) }
func LogInfo(msg string)  { blue.Println("[INFO]", msg) }
func LogSuccess(msg string) { green.Println("[OK]", msg) }

func PrintSeparator() {
	cyan.Println(strings.Repeat("─", 40))
}

// PrintStageHeader prints a consistent section header for each review stage.
func PrintStageHeader(name string) {
	Divider()
	Bold.Printf("── %s ", name)
	fmt.Println()
}

// PrintIssue prints a single diagnostic issue in the same format as the bash version.
func PrintIssue(severity, file string, line int, message string) {
	printIssueWithSource("", severity, file, line, message)
}

// PrintIssueWithSource prints a diagnostic with a tool source label.
func PrintIssueWithSource(source, severity, file string, line int, message string) {
	printIssueWithSource(source, severity, file, line, message)
}

func printIssueWithSource(source, severity, file string, line int, message string) {
	prefix := ""
	if source != "" {
		prefix = source + ": "
	}
	switch severity {
	case "ERROR":
		red.Printf("[ERROR] %s%s\n", prefix, message)
	case "WARNING":
		yellow.Printf("[WARN] %s%s\n", prefix, message)
	default:
		blue.Printf("[INFO] %s%s\n", prefix, message)
	}
	Bold.Printf("        %s:%d\n", file, line)
}

// StageSummary holds counts for a single review stage.
type StageSummary struct {
	Name                           string
	Errors, Warnings, Infos        int
}

// PrintStageSummary prints the summary line for a single stage.
func PrintStageSummary(s StageSummary) {
	fmt.Printf("  %s: %d errors, %d warnings, %d info\n", s.Name, s.Errors, s.Warnings, s.Infos)
}

// PrintStageSummaries prints per-tool breakdowns followed by a total.
func PrintStageSummaries(stages []StageSummary, totalErr, totalWarn, totalInfo int) {
	for _, s := range stages {
		fmt.Printf("  %-20s %d errors, %d warnings, %d info\n", s.Name+":", s.Errors, s.Warnings, s.Infos)
	}
	fmt.Println()
	Bold.Printf("Review Summary: ")
	fmt.Printf("%d errors, %d warnings, %d info\n", totalErr, totalWarn, totalInfo)
}

// PrintSummary prints the final review summary line.
func PrintSummary(errors, warnings, infos int) {
	Bold.Printf("Review Summary: ")
	fmt.Printf("%d errors, %d warnings, %d info\n", errors, warnings, infos)
}

// PrintHeader prints the "AI Review vX.Y.Z" banner.
func PrintHeader(version string) {
	fmt.Println()
	Bold.Printf("AI Review")
	fmt.Printf(" v%s - Pre-commit code review\n", version)
	fmt.Println()
}

// Divider prints a wide separator bar.
func Divider() {
	fmt.Println(strings.Repeat("━", 46))
}
