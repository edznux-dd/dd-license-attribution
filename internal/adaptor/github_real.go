// SPDX-License-Identifier: Apache-2.0
package adaptor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/DataDog/dd-license-attribution/internal/port"
)

// RealGitHubClient implements port.GitHubClient using the GitHub REST API.
type RealGitHubClient struct {
	token      string
	httpClient *http.Client
}

// NewRealGitHubClient creates a new GitHub client with optional auth token.
func NewRealGitHubClient(token string) *RealGitHubClient {
	return &RealGitHubClient{
		token:      token,
		httpClient: &http.Client{},
	}
}

func (c *RealGitHubClient) doRequest(ctx context.Context, url string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, body, nil
}

func (c *RealGitHubClient) GetSBOM(ctx context.Context, owner, repo string) (int, *port.GitHubSBOM, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/dependency-graph/sbom", owner, repo)
	status, body, err := c.doRequest(ctx, url)
	if err != nil {
		return 0, nil, err
	}
	if status != 200 {
		return status, nil, nil
	}

	var wrapper struct {
		SBOM *port.GitHubSBOM `json:"sbom"`
	}
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return status, nil, fmt.Errorf("failed to parse SBOM: %w", err)
	}
	return status, wrapper.SBOM, nil
}

func (c *RealGitHubClient) GetRepository(ctx context.Context, owner, repo string) (int, *port.GitHubRepository, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	status, body, err := c.doRequest(ctx, url)
	if err != nil {
		return 0, nil, err
	}
	if status != 200 {
		// Handle redirect (301)
		if status == 301 {
			var redirectInfo struct {
				URL string `json:"url"`
			}
			if json.Unmarshal(body, &redirectInfo) == nil && redirectInfo.URL != "" {
				// Follow the redirect
				status2, body2, err2 := c.doRequest(ctx, redirectInfo.URL)
				if err2 != nil {
					return 0, nil, err2
				}
				if status2 == 200 {
					var repository port.GitHubRepository
					if err := json.Unmarshal(body2, &repository); err != nil {
						return status2, nil, err
					}
					return status2, &repository, nil
				}
				return status2, nil, nil
			}
		}
		return status, nil, nil
	}

	var repository port.GitHubRepository
	if err := json.Unmarshal(body, &repository); err != nil {
		return status, nil, err
	}
	return status, &repository, nil
}
