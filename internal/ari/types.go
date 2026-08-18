package ari

// Channel is the subset of the ARI channel object the backend uses.
type Channel struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	State        string            `json:"state"`
	Caller       CallerID          `json:"caller"`
	Dialplan     DialplanInfo      `json:"dialplan"`
	ChannelVars  map[string]string `json:"channelvars,omitempty"`
}

type CallerID struct {
	Name   string `json:"name"`
	Number string `json:"number"`
}

type DialplanInfo struct {
	Context  string `json:"context"`
	Exten    string `json:"exten"`
	Priority int    `json:"priority"`
}

type Bridge struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Channels []string `json:"channels"`
}

// Event is a raw ARI event as received over the WebSocket event stream.
type Event struct {
	Type        string   `json:"type"`
	Channel     Channel  `json:"channel"`
	Args        []string `json:"args,omitempty"`
	Timestamp   string   `json:"timestamp"`
	Application string   `json:"application"`
}

const (
	EventStasisStart        = "StasisStart"
	EventStasisEnd          = "StasisEnd"
	EventChannelStateChange = "ChannelStateChange"
	EventChannelDestroyed   = "ChannelDestroyed"
)
