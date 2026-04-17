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

func (c *HTTPClient) Send(ctx context.Context, url string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
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
