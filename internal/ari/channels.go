package ari

import (
	"context"
	"net/http"
	"net/url"
)

// Originate creates a new outbound channel to endpoint, placing it directly
// into the given Stasis application with appArgs passed to the StasisStart
// event. Used both for ringing the home device and for dialing the mock
// outside trunk.
func (c *HTTPClient) Originate(ctx context.Context, endpoint, appName, appArgs, callerID string) (Channel, error) {
	q := url.Values{}
	q.Set("endpoint", endpoint)
	q.Set("app", appName)
	if appArgs != "" {
		q.Set("appArgs", appArgs)
	}
	if callerID != "" {
		q.Set("callerId", callerID)
	}
	var ch Channel
	err := c.do(ctx, http.MethodPost, "/ari/channels", q, nil, &ch)
	return ch, err
}
