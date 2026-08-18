package ari

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/coder/websocket"
)

// Events connects to the ARI WebSocket event stream for the given Stasis
// application and returns a channel of decoded events plus an error channel
// that receives at most one terminal error before closing.
func (c *HTTPClient) Events(ctx context.Context, appName string) (<-chan Event, <-chan error, error) {
	wsURL := strings.Replace(c.baseURL, "http://", "ws://", 1)
	wsURL = strings.Replace(wsURL, "https://", "wss://", 1)

	q := url.Values{}
	q.Set("app", appName)
	q.Set("api_key", c.user+":"+c.password)
	q.Set("subscribeAll", "true")

	full := wsURL + "/ari/events?" + q.Encode()

	conn, _, err := websocket.Dial(ctx, full, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("ari: dial events websocket: %w", err)
	}

	events := make(chan Event)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)
		defer conn.Close(websocket.StatusNormalClosure, "")

		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				select {
				case errs <- err:
				default:
				}
				return
			}
			var ev Event
			if err := json.Unmarshal(data, &ev); err != nil {
				continue
			}
			select {
			case events <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()

	return events, errs, nil
}
