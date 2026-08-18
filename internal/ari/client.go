// Package ari is a hand-rolled client for Asterisk's REST Interface (ARI).
// There is no official or lightweight Go ARI SDK, so REST calls go over
// net/http and the event stream uses github.com/coder/websocket directly.
package ari

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client is the interface consumed by internal/callctl, so tests can supply
// a fake implementation without a real Asterisk instance.
type Client interface {
	Answer(ctx context.Context, channelID string) error
	Hangup(ctx context.Context, channelID string, reason string) error
	Originate(ctx context.Context, endpoint, appName, appArgs, callerID string) (Channel, error)
	CreateBridge(ctx context.Context, bridgeType string) (Bridge, error)
	AddChannelToBridge(ctx context.Context, bridgeID, channelID string) error
	DeleteBridge(ctx context.Context, bridgeID string) error
	Events(ctx context.Context, appName string) (<-chan Event, <-chan error, error)
}

type HTTPClient struct {
	baseURL  string
	user     string
	password string
	http     *http.Client
}

func New(baseURL, user, password string) *HTTPClient {
	return &HTTPClient{
		baseURL:  baseURL,
		user:     user,
		password: password,
		http:     &http.Client{},
	}
}

func (c *HTTPClient) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	u := c.baseURL + path
	if query != nil {
		u += "?" + query.Encode()
	}

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.user, c.password)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ari: %s %s: status %d: %s", method, path, resp.StatusCode, string(respBody))
	}
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("ari: decode response: %w", err)
		}
	}
	return nil
}

func (c *HTTPClient) Answer(ctx context.Context, channelID string) error {
	return c.do(ctx, http.MethodPost, "/ari/channels/"+channelID+"/answer", nil, nil, nil)
}

func (c *HTTPClient) Hangup(ctx context.Context, channelID string, reason string) error {
	q := url.Values{}
	if reason != "" {
		q.Set("reason", reason)
	}
	return c.do(ctx, http.MethodDelete, "/ari/channels/"+channelID, q, nil, nil)
}
