// SPDX-License-Identifier: Apache-2.0
package adaptor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// AnthropicClient implements port.LLMClient using the Anthropic REST API.
type AnthropicClient struct {
	apiKey string
	model  string
	client *http.Client
}

// NewAnthropicClient creates a new Anthropic client.
func NewAnthropicClient(apiKey string, model string) *AnthropicClient {
	if model == "" {
		model = "claude-3-7-sonnet-20250219"
	}
	return &AnthropicClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

func (c *AnthropicClient) ConvertToSPDX(ctx context.Context, licenseText string) (string, error) {
	prompt := fmt.Sprintf(`Identify the SPDX license expression for the following license text.
Respond with ONLY the SPDX license expression (e.g., "BSD-3-Clause", "MIT OR Apache-2.0", "GPL-2.0-only WITH Classpath-exception-2.0").
Use operators like OR, AND, WITH for composite licenses.

License text:
%s

SPDX license expression:`, licenseText)

	reqBody := map[string]interface{}{
		"model":      c.model,
		"max_tokens": 50,
		"temperature": 0,
		"system":     "You are a license identification expert. Your task is to identify the SPDX license expression from license text. Respond ONLY with the SPDX license expression, nothing else. Use operators like OR, AND, WITH for composite licenses. If you cannot identify the license, respond with 'UNKNOWN'.",
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Anthropic API error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		slog.Error("Anthropic API error", "status", resp.StatusCode, "body", string(respBody))
		return "UNKNOWN", nil
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	for _, block := range result.Content {
		if block.Type == "text" && block.Text != "" {
			return block.Text, nil
		}
	}

	return "UNKNOWN", nil
}
