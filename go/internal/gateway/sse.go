package gateway

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// parseSSEStream reads a text/event-stream body and assembles a ReviewResult.
// onDiagnostic is called for each "diagnostic" event as it arrives (may be nil).
// Returns an error if the stream contains an "error" event or cannot be read.
func parseSSEStream(body io.Reader, onDiagnostic func(Diagnostic)) (*ReviewResult, error) {
	const maxTokenSize = 1 << 20 // 1 MB — handles large diagnostic payloads

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, maxTokenSize), maxTokenSize)

	var (
		result       ReviewResult
		currentEvent string
		currentData  strings.Builder
	)

	flush := func() error {
		if currentEvent == "" {
			return nil
		}
		data := strings.TrimSpace(currentData.String())
		currentData.Reset()
		event := currentEvent
		currentEvent = ""

		switch event {
		case "progress":
			// Informational only — no action needed.

		case "text":
			// Incremental text chunks accumulate into the overview.
			var payload struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err == nil {
				result.Overview += payload.Content
			}

		case "diagnostic":
			var d Diagnostic
			if err := json.Unmarshal([]byte(data), &d); err != nil {
				return fmt.Errorf("parse diagnostic event: %w", err)
			}
			if onDiagnostic != nil {
				onDiagnostic(d)
			}
			result.Diagnostics = append(result.Diagnostics, d)

		case "complete":
			var payload struct {
				Overview    string `json:"overview"`
				MaxSeverity string `json:"max_severity"`
				Source      Source `json:"source"`
			}
			if err := json.Unmarshal([]byte(data), &payload); err == nil {
				if payload.Overview != "" {
					result.Overview = payload.Overview
				}
				result.MaxSeverity = payload.MaxSeverity
				if payload.Source.Name != "" {
					result.Source = payload.Source
				}
			}

		case "error":
			var payload struct {
				Message string `json:"message"`
			}
			msg := data
			if err := json.Unmarshal([]byte(data), &payload); err == nil && payload.Message != "" {
				msg = payload.Message
			}
			return fmt.Errorf("gateway error: %s", msg)
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			currentEvent = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			if currentData.Len() > 0 {
				currentData.WriteByte('\n')
			}
			currentData.WriteString(strings.TrimSpace(line[len("data:"):]))
		case line == "":
			if err := flush(); err != nil {
				return nil, err
			}
		}
		// id: and retry: lines are intentionally ignored.
	}

	// Flush any trailing event that has no closing blank line.
	if err := flush(); err != nil {
		return nil, err
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read SSE stream: %w", err)
	}

	return &result, nil
}
