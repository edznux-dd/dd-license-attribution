// SPDX-License-Identifier: Apache-2.0
package adaptor

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/DataDog/dd-license-attribution/internal/port"
)

// RealHTTPFetcher implements port.HTTPFetcher using net/http.
type RealHTTPFetcher struct {
	client *http.Client
}

// NewRealHTTPFetcher creates a new HTTP fetcher with a default timeout.
func NewRealHTTPFetcher() *RealHTTPFetcher {
	return &RealHTTPFetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (f *RealHTTPFetcher) Get(ctx context.Context, url string) (*port.HTTPResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &port.HTTPResponse{
		StatusCode: resp.StatusCode,
		Body:       body,
	}, nil
}
