package plex

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Client communicates with the Plex Media Server API.
type Client struct {
	baseURL   string
	token     string
	sectionID string
	http      *http.Client
}

// NewClient creates a new Plex API client.
func NewClient(baseURL, token, sectionID string) *Client {
	return &Client{
		baseURL:   baseURL,
		token:     token,
		sectionID: sectionID,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RefreshLibrary triggers a scan of the configured library section.
func (c *Client) RefreshLibrary(ctx context.Context) error {
	url := fmt.Sprintf("%s/library/sections/%s/refresh", c.baseURL, c.sectionID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create refresh request: %w", err)
	}

	req.Header.Set("X-Plex-Token", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("plex refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("plex refresh returned status %d", resp.StatusCode)
	}

	return nil
}
