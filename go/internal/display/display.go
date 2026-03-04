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

// PrintIssue prints a single diagnostic issue in the same format as the bash version.
func PrintIssue(severity, file string, line int, message string) {
	switch severity {
	case "ERROR":
		red.Printf("[ERROR] %s\n", message)
	case "WARNING":
		yellow.Printf("[WARN] %s\n", message)
	default:
		blue.Printf("[INFO] %s\n", message)
	}
	Bold.Printf("        %s:%d\n", file, line)
}

// PrintSummary prints the final review summary line.
func PrintSummary(errors, warnings, infos int) {
	Bold.Printf("AI Review Summary: ")
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
