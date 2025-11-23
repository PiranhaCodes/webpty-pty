package api

import "encoding/json"

// Request represents an incoming request over the UNIX socket.
type Request struct {
	Action string          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

// Response represents a response to a request.
type Response struct {
	Ok   bool        `json:"ok"`
	Err  string      `json:"err,omitempty"`
	Data interface{} `json:"data,omitempty"`
}

// SpawnRequest is the data for a spawn action.
type SpawnRequest struct{}

// SpawnResponse is the data returned from a spawn action.
type SpawnResponse struct {
	ID string `json:"id"`
}

// WriteRequest is the data for a write action.
type WriteRequest struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

// ResizeRequest is the data for a resize action.
type ResizeRequest struct {
	ID   string `json:"id"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

// KillRequest is the data for a kill action.
type KillRequest struct {
	ID string `json:"id"`
}

// ListResponse is the data returned from a list action.
type ListResponse struct {
	Sessions []SessionInfo `json:"sessions"`
	Count    int           `json:"count"`
}

// SessionInfo contains information about a session.
type SessionInfo struct {
	ID     string `json:"id"`
	Status string `json:"status"` // "active" or "exiting"
}
