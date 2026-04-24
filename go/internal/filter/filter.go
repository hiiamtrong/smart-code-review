// Package filter implements .aireviewignore pattern matching and diff filtering.
// Patterns use gitignore-style glob syntax (*, **, ?, character classes).
package filter

import (
	"bufio"
	"os"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// binaryExts are file extensions that should always be excluded from review.
var binaryExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
	".ico": true, ".webp": true, ".bmp": true, ".tif": true, ".tiff": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true, ".otf": true,
	".pdf": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true,
	".jar": true, ".war": true, ".nar": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true,
	".wasm": true, ".class": true, ".o": true, ".a": true, ".lib": true,
	".mp3": true, ".mp4": true, ".wav": true, ".avi": true, ".mov": true,
	".webm": true, ".flac": true, ".ogg": true,
	".db": true, ".sqlite": true, ".lock": true,
	".out": true, ".bin": true, ".prof": true,
	".min.js": true, ".min.css": true,
	".bundle.js": true, ".bundle.css": true, ".chunk.js": true,
}

// LoadIgnorePatterns reads and parses a .aireviewignore file.
// Returns an empty slice (not an error) if the file does not exist.
// Lines starting with # and blank lines are ignored.
func LoadIgnorePatterns(path string) ([]string, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Strip Windows CRLF
		line = strings.TrimRight(line, "\r")
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, scanner.Err()
}

// FilterDiff removes diff file blocks whose paths match any of the given patterns.
// Returns the filtered diff string and the count of ignored files.
// An empty patterns slice returns the original diff unchanged.
func FilterDiff(diff string, patterns []string) (string, int) {
	if len(patterns) == 0 || diff == "" {
		return diff, 0
	}

	type block struct {
		file    string
		content strings.Builder
	}

	var blocks []block
	var current *block

	for _, line := range strings.Split(diff, "\n") {
		// Normalise Windows line endings
		line = strings.TrimRight(line, "\r")

		if file, ok := parseDiffGitHeader(line); ok {
			// Flush previous block
			if current != nil {
				blocks = append(blocks, *current)
			}
			current = &block{file: file}
			current.content.WriteString(line + "\n")
		} else if current != nil {
			current.content.WriteString(line + "\n")
		}
	}
	// Flush last block
	if current != nil {
		blocks = append(blocks, *current)
	}

	var out strings.Builder
	ignoredCount := 0
	for _, b := range blocks {
		if isBinaryExt(b.file) || matchesAny(b.file, patterns) {
			ignoredCount++
		} else {
			out.WriteString(b.content.String())
		}
	}
	return out.String(), ignoredCount
}

// ─── internal helpers ─────────────────────────────────────────────────────────

// parseDiffGitHeader detects a "diff --git a/X b/Y" line and returns Y (the new filename).
func parseDiffGitHeader(line string) (string, bool) {
	if !strings.HasPrefix(line, "diff --git ") {
		return "", false
	}
	// Format: diff --git a/<path> b/<path>
	// Split on " b/" from the right to handle spaces in filenames
	const bSep = " b/"
	idx := strings.LastIndex(line, bSep)
	if idx < 0 {
		return "", false
	}
	return line[idx+len(bSep):], true
}

// matchesAny reports whether file matches any of the given glob patterns.
// Each pattern is tried both as-is and with a "**/" prefix, matching bash's
// behaviour where "*.lock" matches "dir/package.lock".
func matchesAny(file string, patterns []string) bool {
	for _, p := range patterns {
		if match(file, p) {
			return true
		}
		// Also try anchored prefix so "*.lock" matches "some/dir/package.lock"
		if !strings.Contains(p, "/") {
			if match(file, "**/"+p) {
				return true
			}
		}
	}
	return false
}

func match(file, pattern string) bool {
	ok, _ := doublestar.Match(pattern, file)
	return ok
}

// isBinaryExt reports whether the file has a known binary extension.
func isBinaryExt(file string) bool {
	return binaryExts[strings.ToLower(strExt(file))]
}

// strExt returns the extension of a filename (including dot).
func strExt(file string) string {
	if i := strings.LastIndex(file, "."); i >= 0 {
		return file[i:]
	}
	return ""
}
