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

// OpenAIClient implements port.LLMClient using the OpenAI REST API.
type OpenAIClient struct {
	apiKey string
	model  string
	client *http.Client
}

// NewOpenAIClient creates a new OpenAI client.
func NewOpenAIClient(apiKey string, model string) *OpenAIClient {
	if model == "" {
		model = "gpt-4"
	}
	return &OpenAIClient{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{},
	}
}

func (c *OpenAIClient) ConvertToSPDX(ctx context.Context, licenseText string) (string, error) {
	prompt := fmt.Sprintf(`Identify the SPDX license expression for the following license text.
Respond with ONLY the SPDX license expression (e.g., "BSD-3-Clause", "MIT OR Apache-2.0", "GPL-2.0-only WITH Classpath-exception-2.0").
Use operators like OR, AND, WITH for composite licenses.

License text:
%s

SPDX license expression:`, licenseText)

	reqBody := map[string]interface{}{
		"model": c.model,
		"messages": []map[string]string{
			{
				"role":    "system",
				"content": "You are a license identification expert. Your task is to identify the SPDX license expression from license text. Respond ONLY with the SPDX license expression, nothing else. Use operators like OR, AND, WITH for composite licenses. If you cannot identify the license, respond with 'UNKNOWN'.",
			},
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"temperature": 0,
		"max_tokens":  50,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("OpenAI API error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		slog.Error("OpenAI API error", "status", resp.StatusCode, "body", string(respBody))
		return "UNKNOWN", nil
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	if len(result.Choices) == 0 || result.Choices[0].Message.Content == "" {
		return "UNKNOWN", nil
	}

	return result.Choices[0].Message.Content, nil
}
