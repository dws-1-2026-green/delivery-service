package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type HTTPClient struct {
	Client *http.Client
}

func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		Client: &http.Client{},
	}
}

// Send sends an HTTP request with the specified method, headers, and payload
// For GET requests, payload is sent as query parameters
// For POST/PUT/PATCH requests, payload is sent as JSON in the request body
func (c *HTTPClient) Send(ctx context.Context, method string, urlStr string, headers map[string]string, payload []byte) error {
	if method == "" {
		method = http.MethodPost
	}

	var req *http.Request
	var err error

	if method == http.MethodGet {
		// For GET requests, append payload as query parameters
		req, err = c.buildGETRequest(ctx, urlStr, headers, payload)
	} else {
		// For POST, PUT, PATCH requests, send payload in body
		req, err = http.NewRequestWithContext(ctx, method, urlStr, bytes.NewReader(payload))
		if err != nil {
			return err
		}

		// Set Content-Type header for non-GET requests
		req.Header.Set("Content-Type", "application/json")
	}

	if err != nil {
		return err
	}

	// Apply custom headers from subscription
	for key, value := range headers {
		// Don't override Content-Type for GET requests, it's already set
		if key == "Content-Type" && method == http.MethodGet {
			continue
		}
		req.Header.Set(key, value)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return &HTTPError{StatusCode: resp.StatusCode}
	}

	return nil
}

// buildGETRequest constructs a GET request with payload as query parameters
func (c *HTTPClient) buildGETRequest(ctx context.Context, urlStr string, headers map[string]string, payload []byte) (*http.Request, error) {
	baseURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, err
	}

	// Parse payload as JSON and add to query parameters
	var data map[string]interface{}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &data); err != nil {
			return nil, fmt.Errorf("failed to parse payload as JSON: %w", err)
		}

		q := baseURL.Query()
		for key, value := range data {
			// Convert value to string
			q.Set(key, fmt.Sprintf("%v", value))
		}
		baseURL.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL.String(), nil)
	if err != nil {
		return nil, err
	}

	return req, nil
}

type HTTPError struct {
	StatusCode int
}

func (e *HTTPError) Error() string {
	return http.StatusText(e.StatusCode)
}