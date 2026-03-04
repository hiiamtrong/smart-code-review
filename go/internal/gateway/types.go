// Package gateway provides the HTTP client for the AI Gateway review endpoint.
package gateway

import (
	"github.com/hiiamtrong/smart-code-review/internal/git"
)

// ReviewPayload contains everything needed to submit a review request.
type ReviewPayload struct {
	Diff       string
	Language   string
	GitInfo    git.GitInfo
	AIModel    string
	AIProvider string
	Stream     bool
}

// ReviewResult is the final output of a completed review.
type ReviewResult struct {
	Source      Source       `json:"source"`
	Diagnostics []Diagnostic `json:"diagnostics"`
	Overview    string       `json:"overview,omitempty"`
	MaxSeverity string       `json:"max_severity,omitempty"`
}

// Diagnostic represents a single code review finding in rdjson format.
type Diagnostic struct {
	Message  string   `json:"message"`
	Location Location `json:"location"`
	Severity string   `json:"severity"` // "ERROR" | "WARNING" | "INFO"
	Code     Code     `json:"code,omitempty"`
}

// Location identifies where in the code the diagnostic applies.
type Location struct {
	Path  string `json:"path"`
	Range Range  `json:"range"`
}

// Range specifies a span of lines/columns in a file.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position is a line + column pair (1-based).
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// Code is an optional code/URL attached to a diagnostic.
type Code struct {
	Value string `json:"value"`
	URL   string `json:"url,omitempty"`
}

// Source identifies the tool that produced the diagnostics.
type Source struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// metadataPayload is the JSON sent in the "metadata" multipart field.
type metadataPayload struct {
	AIModel    string      `json:"ai_model"`
	AIProvider string      `json:"ai_provider"`
	Language   string      `json:"language"`
	ReviewMode string      `json:"review_mode"`
	Stream     bool        `json:"stream,omitempty"`
	GitInfo    gitInfoJSON `json:"git_info"`
}

type gitInfoJSON struct {
	CommitHash string     `json:"commit_hash"`
	BranchName string     `json:"branch_name"`
	PRNumber   string     `json:"pr_number,omitempty"`
	RepoURL    string     `json:"repo_url"`
	Author     personJSON `json:"author"`
	Committer  personJSON `json:"committer,omitempty"`
}

type personJSON struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}
