// Package dockerapi holds Docker Engine API types shared by the agent and the helper.
package dockerapi

import (
	"bytes"
	"encoding/json"
)

// Health decodes a container's health from GET /containers/json. Recent Engine APIs (seen on 1.56)
// returns an object ({"Status":"healthy","FailingStreak":0}); older versions may
// return a plain string or omit it.
type Health string

func (h *Health) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if bytes.Equal(b, []byte("null")) {
		*h = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*h = normalized(s)
		return nil
	}
	var v struct {
		Status string `json:"Status"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*h = normalized(v.Status)
	return nil
}

func normalized(status string) Health {
	switch status {
	case "healthy", "unhealthy", "starting":
		return Health(status)
	default:
		return ""
	}
}
