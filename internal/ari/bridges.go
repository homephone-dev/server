package ari

import (
	"context"
	"net/http"
	"net/url"
)

func (c *HTTPClient) CreateBridge(ctx context.Context, bridgeType string) (Bridge, error) {
	q := url.Values{}
	if bridgeType != "" {
		q.Set("type", bridgeType)
	}
	var b Bridge
	err := c.do(ctx, http.MethodPost, "/ari/bridges", q, nil, &b)
	return b, err
}

func (c *HTTPClient) AddChannelToBridge(ctx context.Context, bridgeID, channelID string) error {
	q := url.Values{}
	q.Set("channel", channelID)
	return c.do(ctx, http.MethodPost, "/ari/bridges/"+bridgeID+"/addChannel", q, nil, nil)
}

func (c *HTTPClient) DeleteBridge(ctx context.Context, bridgeID string) error {
	return c.do(ctx, http.MethodDelete, "/ari/bridges/"+bridgeID, nil, nil, nil)
}
