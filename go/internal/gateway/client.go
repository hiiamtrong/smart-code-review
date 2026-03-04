package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/hiiamtrong/smart-code-review/internal/config"
)

// StreamingReview posts the diff to the AI Gateway and processes the SSE response.
// onDiagnostic is called for each "diagnostic" event as it streams in (may be nil).
// Automatically falls back to SyncReview if the SSE stream cannot be parsed.
func StreamingReview(
	ctx context.Context,
	cfg *config.Config,
	payload ReviewPayload,
	onDiagnostic func(Diagnostic),
) (*ReviewResult, error) {
	timeout := time.Duration(cfg.GatewayTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, contentType, err := buildMultipartBody(payload, true)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.AIGatewayURL+"/review", body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", cfg.AIGatewayAPIKey)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gateway request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("gateway returned HTTP %d: %s", resp.StatusCode, preview)
	}

	result, err := parseSSEStream(resp.Body, onDiagnostic)
	if err != nil {
		// SSE parse failed — fall back to sync
		return SyncReview(ctx, cfg, payload)
	}
	return result, nil
}

// SyncReview is the non-streaming fallback. It posts the diff and expects a
// single JSON response body in ReviewResult format.
func SyncReview(
	ctx context.Context,
	cfg *config.Config,
	payload ReviewPayload,
) (*ReviewResult, error) {
	timeout := time.Duration(cfg.GatewayTimeoutSec) * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, contentType, err := buildMultipartBody(payload, false)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", cfg.AIGatewayURL+"/review", body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("X-API-Key", cfg.AIGatewayAPIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gateway request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		preview, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("gateway returned HTTP %d: %s", resp.StatusCode, preview)
	}

	var result ReviewResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// ─── internal helpers ─────────────────────────────────────────────────────────

// buildMultipartBody constructs the multipart/form-data body with "metadata"
// and "git_diff" fields, matching the bash curl -F invocation exactly.
func buildMultipartBody(payload ReviewPayload, stream bool) (io.Reader, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	// "metadata" field — JSON
	meta := metadataPayload{
		AIModel:    payload.AIModel,
		AIProvider: payload.AIProvider,
		Language:   payload.Language,
		ReviewMode: "file",
		Stream:     stream,
		GitInfo: gitInfoJSON{
			CommitHash: payload.GitInfo.CommitHash,
			BranchName: payload.GitInfo.BranchName,
			PRNumber:   payload.GitInfo.PRNumber,
			RepoURL:    payload.GitInfo.RepoURL,
			Author:     personJSON{Name: payload.GitInfo.Author.Name, Email: payload.GitInfo.Author.Email},
			Committer:  personJSON{Name: payload.GitInfo.Committer.Name, Email: payload.GitInfo.Committer.Email},
		},
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return nil, "", err
	}
	if err := w.WriteField("metadata", string(metaJSON)); err != nil {
		return nil, "", err
	}

	// "git_diff" field — file upload
	fw, err := w.CreateFormFile("git_diff", "diff.txt")
	if err != nil {
		return nil, "", err
	}
	if _, err := fw.Write([]byte(payload.Diff)); err != nil {
		return nil, "", err
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return &buf, w.FormDataContentType(), nil
}
