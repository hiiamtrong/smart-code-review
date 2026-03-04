// Package git provides helpers for interacting with the local git repository.
package git

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitInfo holds metadata about the current commit and repository.
type GitInfo struct {
	CommitHash string `json:"commit_hash"`
	BranchName string `json:"branch_name"`
	PRNumber   string `json:"pr_number,omitempty"`
	RepoURL    string `json:"repo_url"`
	Author     Person `json:"author"`
	Committer  Person `json:"committer,omitempty"`
}

// Person holds a git user's name and email.
type Person struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// GetStagedDiff returns the output of `git diff --cached`.
func GetStagedDiff() (string, error) {
	return run("git", "diff", "--cached")
}

// GetPRDiff returns the diff between HEAD and the base branch.
// Falls back through: origin/<base> → origin/main → origin/master → HEAD~1 → staged.
func GetPRDiff(baseBranch string) (string, error) {
	candidates := []string{}
	if baseBranch != "" {
		candidates = append(candidates, "origin/"+baseBranch)
	}
	candidates = append(candidates, "origin/main", "origin/master")

	for _, ref := range candidates {
		if refExists(ref) {
			return run("git", "diff", ref+"...HEAD")
		}
	}

	// Fallback: previous commit
	if refExists("HEAD~1") {
		return run("git", "diff", "HEAD~1")
	}

	// Final fallback: staged changes
	return GetStagedDiff()
}

// GetGitInfo collects author, committer, branch, commit hash, and repo URL.
func GetGitInfo() (GitInfo, error) {
	var info GitInfo
	var err error

	if info.CommitHash, err = run("git", "rev-parse", "HEAD"); err != nil {
		info.CommitHash = "staged"
	}
	if info.BranchName, err = GetCurrentBranch(); err != nil {
		info.BranchName = os.Getenv("GITHUB_REF_NAME")
	}

	info.Author.Name, _ = run("git", "log", "-1", "--format=%an")
	info.Author.Email, _ = run("git", "log", "-1", "--format=%ae")
	info.Committer.Name, _ = run("git", "log", "-1", "--format=%cn")
	info.Committer.Email, _ = run("git", "log", "-1", "--format=%ce")

	repoURL, _ := run("git", "remote", "get-url", "origin")
	if repoURL == "" {
		// GitHub Actions fallback
		server := os.Getenv("GITHUB_SERVER_URL")
		repo := os.Getenv("GITHUB_REPOSITORY")
		if server != "" && repo != "" {
			repoURL = server + "/" + repo
		} else {
			repoURL = "local"
		}
	}
	info.RepoURL = repoURL

	// PR number from GitHub Actions environment
	info.PRNumber = extractPRNumber()

	return info, nil
}

// GetCurrentBranch returns the current branch name.
func GetCurrentBranch() (string, error) {
	return run("git", "rev-parse", "--abbrev-ref", "HEAD")
}

// GetRemoteTrackingBranch returns the upstream tracking branch (e.g. origin/main).
func GetRemoteTrackingBranch() (string, error) {
	return run("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
}

// GetRepoRoot returns the absolute path to the git repository root.
func GetRepoRoot() (string, error) {
	root, err := run("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("not a git repository")
	}
	return root, nil
}

// GetLocalConfig reads a per-repo git config value.
// Returns ("", nil) if the key is not set.
func GetLocalConfig(key string) (string, error) {
	val, err := run("git", "config", "--local", key)
	if err != nil {
		return "", nil // key not set is not an error
	}
	return val, nil
}

// AnnotateLineNumbers adds `+NNN:` prefixes to added lines in a unified diff,
// porting the showlinenum.awk logic to pure Go (show_header=0 show_path=1).
func AnnotateLineNumbers(diff string) string {
	var out strings.Builder
	newLineNum := 0

	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "@@"):
			// Parse the @@ -old,n +new,m @@ header to get the new-file starting line
			newLineNum = parseHunkStart(line)
			out.WriteString(line + "\n")
		case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
			out.WriteString(fmt.Sprintf("+%d:%s\n", newLineNum, line[1:]))
			newLineNum++
		case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
			out.WriteString(line + "\n")
			// context lines (no +/-) advance the new file line counter
		case !strings.HasPrefix(line, "\\"):
			out.WriteString(line + "\n")
			if !strings.HasPrefix(line, "diff") &&
				!strings.HasPrefix(line, "index") &&
				!strings.HasPrefix(line, "---") &&
				!strings.HasPrefix(line, "+++") &&
				!strings.HasPrefix(line, "@@") {
				newLineNum++
			}
		}
	}
	return out.String()
}

// ─── internal helpers ─────────────────────────────────────────────────────────

// run executes a git command and returns trimmed stdout.
func run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func refExists(ref string) bool {
	err := exec.Command("git", "rev-parse", "--verify", ref).Run()
	return err == nil
}

// parseHunkStart extracts the starting line number of the new file from a
// unified diff @@ header: "@@ -old,n +new,m @@" → new.
func parseHunkStart(hunk string) int {
	// Find "+NNN" after the first space
	idx := strings.Index(hunk, " +")
	if idx < 0 {
		return 0
	}
	rest := hunk[idx+2:]
	end := strings.IndexAny(rest, ", @")
	if end < 0 {
		end = len(rest)
	}
	n := 0
	fmt.Sscan(rest[:end], &n)
	return n
}

func extractPRNumber() string {
	// From GITHUB_REF: refs/pull/123/merge
	ref := os.Getenv("GITHUB_REF")
	if strings.Contains(ref, "/pull/") {
		parts := strings.Split(ref, "/")
		for i, p := range parts {
			if p == "pull" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	return ""
}
