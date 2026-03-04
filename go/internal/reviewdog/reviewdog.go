// Package reviewdog provides helpers for writing rdjson output, invoking the
// reviewdog binary, and posting overview comments via the GitHub API.
package reviewdog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hiiamtrong/smart-code-review/internal/gateway"
)

const overviewSentinel = "<!-- ai-review-overview -->"

// ─── rdjson output ────────────────────────────────────────────────────────────

// rdDiagnostic is the rdjson wire format for a single finding.
type rdDiagnostic struct {
	Message  string     `json:"message"`
	Location rdLocation `json:"location"`
	Severity string     `json:"severity"`
	Code     *rdCode    `json:"code,omitempty"`
	Source   *rdSource  `json:"source,omitempty"`
}

type rdLocation struct {
	Path  string   `json:"path"`
	Range rdRange  `json:"range"`
}

type rdRange struct {
	Start rdPosition `json:"start"`
	End   rdPosition `json:"end,omitempty"`
}

type rdPosition struct {
	Line   int `json:"line"`
	Column int `json:"column,omitempty"`
}

type rdCode struct {
	Value string `json:"value"`
	URL   string `json:"url,omitempty"`
}

type rdSource struct {
	Name string `json:"name"`
	URL  string `json:"url,omitempty"`
}

// WriteRDJSON writes the diagnostics from result to path in rdjson format
// (newline-delimited JSON, one Diagnostic object per line).
func WriteRDJSON(result *gateway.ReviewResult, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create rdjson file: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)

	src := &rdSource{Name: result.Source.Name, URL: result.Source.URL}
	if src.Name == "" {
		src.Name = "ai-review"
	}

	for _, d := range result.Diagnostics {
		rd := rdDiagnostic{
			Message: d.Message,
			Location: rdLocation{
				Path: d.Location.Path,
				Range: rdRange{
					Start: rdPosition{Line: d.Location.Range.Start.Line, Column: d.Location.Range.Start.Column},
					End:   rdPosition{Line: d.Location.Range.End.Line, Column: d.Location.Range.End.Column},
				},
			},
			Severity: d.Severity,
			Source:   src,
		}
		if d.Code.Value != "" {
			rd.Code = &rdCode{Value: d.Code.Value, URL: d.Code.URL}
		}
		if err := enc.Encode(rd); err != nil {
			return fmt.Errorf("encode diagnostic: %w", err)
		}
	}
	return nil
}

// WriteOverview writes the plain-text overview to path.
func WriteOverview(result *gateway.ReviewResult, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	return os.WriteFile(path, []byte(result.Overview), 0644)
}

// ─── reviewdog invocation ─────────────────────────────────────────────────────

// InvokeReviewdog runs the reviewdog binary with the given rdjson input file
// and reporter. Standard output and error are forwarded to the caller's streams.
func InvokeReviewdog(inputFile, reporter string) error {
	// Look for reviewdog in ~/bin first (common CI setup), then $PATH.
	rdBin := filepath.Join(os.Getenv("HOME"), "bin", "reviewdog")
	if _, err := os.Stat(rdBin); err != nil {
		rdBin = "reviewdog"
	}

	cmd := exec.Command(rdBin,
		"-f", "rdjson",
		"-reporter", reporter,
		"-fail-on-error",
	)
	cmd.Stdin, _ = os.Open(inputFile) //nolint:errcheck — Open failure yields nil reader which reviewdog handles
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("reviewdog: %w", err)
	}
	return nil
}

// ─── GitHub API helpers ───────────────────────────────────────────────────────

// PostOverviewComment posts the overview as a PR comment.
// Any existing overview comments (identified by overviewSentinel) are deleted first.
func PostOverviewComment(token, repo, prNumber, overview string) error {
	if err := DeleteExistingOverviewComments(token, repo, prNumber); err != nil {
		return err
	}

	body := fmt.Sprintf("%s\n\n**AI Review Overview**\n\n%s", overviewSentinel, overview)
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%s/comments", repo, prNumber)

	payload, _ := json.Marshal(map[string]string{"body": body})
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build comment request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("post comment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("post comment: HTTP %d: %s", resp.StatusCode, preview)
	}
	return nil
}

// DeleteExistingOverviewComments removes all PR comments that contain overviewSentinel.
func DeleteExistingOverviewComments(token, repo, prNumber string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%s/comments?per_page=100", repo, prNumber)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build list request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("list comments: %w", err)
	}
	defer resp.Body.Close()

	var comments []struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return fmt.Errorf("decode comments: %w", err)
	}

	for _, c := range comments {
		if !strings.Contains(c.Body, overviewSentinel) {
			continue
		}
		delURL := fmt.Sprintf("https://api.github.com/repos/%s/issues/comments/%d", repo, c.ID)
		delReq, err := http.NewRequest(http.MethodDelete, delURL, nil)
		if err != nil {
			return fmt.Errorf("build delete request: %w", err)
		}
		delReq.Header.Set("Authorization", "Bearer "+token)
		delReq.Header.Set("Accept", "application/vnd.github+json")

		delResp, err := http.DefaultClient.Do(delReq)
		if err != nil {
			return fmt.Errorf("delete comment %d: %w", c.ID, err)
		}
		delResp.Body.Close()
	}
	return nil
}
