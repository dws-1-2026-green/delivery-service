package client

import (
	"bytes"
	"context"
	"net/http"
)

type HTTPClient struct {
	Client *http.Client
}

func NewHTTPClient() *HTTPClient {
	return &HTTPClient{
		Client: &http.Client{},
	}
}

func (c *HTTPClient) Send(ctx context.Context, method, url string, headers map[string]string, payload []byte) error {
	var req *http.Request
	var err error

	if method == http.MethodGet {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, bytes.NewReader(payload))
	}
	if err != nil {
		return err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	if method != http.MethodGet && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
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

type HTTPError struct {
	StatusCode int
}

func (e *HTTPError) Error() string {
	return http.StatusText(e.StatusCode)
}
