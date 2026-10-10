package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

// MCPHandler is a separate, token-only, stateless Streamable HTTP endpoint.
// It deliberately exposes no mutations, command lines, secrets, or raw paths.
func (a *API) MCPHandler(token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mcp" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Origin") != "" {
			http.Error(w, "origin not allowed", http.StatusForbidden)
			return
		}
		authorization := r.Header.Get("Authorization")
		got := strings.TrimPrefix(authorization, "Bearer ")
		if token == "" || !strings.HasPrefix(authorization, "Bearer ") || !readTokenMatches(got, token) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			http.Error(w, "JSON required", http.StatusUnsupportedMediaType)
			return
		}
		var request struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&request); err != nil || request.JSONRPC != "2.0" {
			http.Error(w, "invalid JSON-RPC request", http.StatusBadRequest)
			return
		}
		if len(request.ID) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		response := map[string]any{"jsonrpc": "2.0", "id": request.ID}
		switch request.Method {
		case "initialize":
			response["result"] = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]string{"name": "fleetglass", "version": "0.9"}}
		case "ping":
			response["result"] = map[string]any{}
		case "tools/list":
			response["result"] = map[string]any{"tools": mcpTools}
		case "tools/call":
			var params struct {
				Name      string `json:"name"`
				Arguments struct {
					Host   string `json:"host"`
					Figure string `json:"figure"`
					At     string `json:"at"`
					Port   int    `json:"port"`
				} `json:"arguments"`
			}
			if json.Unmarshal(request.Params, &params) != nil {
				response["error"] = map[string]any{"code": -32602, "message": "invalid arguments"}
				break
			}
			v, err := a.mcpCall(params.Name, params.Arguments.Host, params.Arguments.Figure, params.Arguments.At, params.Arguments.Port)
			if err != "" {
				response["result"] = map[string]any{"content": []map[string]string{{"type": "text", "text": err}}, "isError": true}
			} else {
				response["result"] = map[string]any{"content": []map[string]string{{"type": "text", "text": string(v)}}}
			}
		default:
			response["error"] = map[string]any{"code": -32601, "message": "method not found"}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(response)
	})
}

var mcpTools = []map[string]any{
	{"name": "what_is_running", "description": "List current apps and AI agent sessions on one host.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"host": map[string]string{"type": "string"}}, "required": []string{"host"}}, "annotations": map[string]bool{"readOnlyHint": true}},
	{"name": "figure_attribution", "description": "Find apps behind CPU, memory, disk, or network use now or at a historical time (RFC3339).", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"host": map[string]string{"type": "string"}, "figure": map[string]any{"type": "string", "enum": []string{"cpu", "memory", "disk", "network"}}, "at": map[string]string{"type": "string"}}, "required": []string{"host", "figure"}}, "annotations": map[string]bool{"readOnlyHint": true}},
	{"name": "what_is_on_port", "description": "Find observed project processes listening on a TCP port.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"host": map[string]string{"type": "string"}, "port": map[string]string{"type": "integer"}}, "required": []string{"host", "port"}}, "annotations": map[string]bool{"readOnlyHint": true}},
	{"name": "host_status", "description": "Summarize the current health and measurement time of a host.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"host": map[string]string{"type": "string"}}, "required": []string{"host"}}, "annotations": map[string]bool{"readOnlyHint": true}},
}

func (a *API) mcpCall(name, host, figure, at string, port int) ([]byte, string) {
	if name != "what_is_running" && name != "figure_attribution" && name != "what_is_on_port" && name != "host_status" {
		return nil, "unknown tool"
	}
	if host == "" || len(host) > 128 || strings.ContainsAny(host, "/\\") {
		return nil, "invalid host"
	}
	overview, err := a.Store.Overview(host)
	if err != nil {
		return nil, "host not found"
	}
	var answer any
	switch name {
	case "what_is_running":
		apps, e := a.Store.Apps(host, "1h", time.Now().UTC())
		if e != nil {
			return nil, "data unavailable"
		}
		answer = map[string]any{"host": host, "at": overview.At, "apps": apps.Apps, "agents": overview.Agents}
	case "figure_attribution":
		if figure != "cpu" && figure != "memory" && figure != "disk" && figure != "network" {
			return nil, "invalid figure"
		}
		when := time.Now().UTC()
		historical := at != ""
		if historical {
			when, err = time.Parse(time.RFC3339, at)
			if err != nil || when.After(time.Now().Add(time.Minute)) || when.Before(time.Now().Add(-30*24*time.Hour)) {
				return nil, "invalid or unavailable time"
			}
		}
		apps, e := a.Store.Apps(host, "30d", time.Now().UTC())
		if e != nil {
			return nil, "data unavailable"
		}
		type row struct {
			Name  string  `json:"name"`
			Kind  string  `json:"kind"`
			Value float64 `json:"value"`
		}
		rows := []row{}
		sampleAt := overview.At
		add := func(app model.AppSample) {
			var n float64
			switch figure {
			case "cpu":
				n = app.CPUPercent
			case "memory":
				n = app.MemoryBytes
			case "disk":
				n = app.DiskReadRate + app.DiskWriteRate
			case "network":
				if app.NetworkRXRate != nil {
					n += *app.NetworkRXRate
				}
				if app.NetworkTXRate != nil {
					n += *app.NetworkTXRate
				}
			}
			if n > 0 {
				rows = append(rows, row{app.Name, app.Kind, n})
			}
		}
		if historical {
			closest := time.Duration(1<<63 - 1)
			for _, p := range apps.History {
				d := p.At.Sub(when)
				if d < 0 {
					d = -d
				}
				if d < closest {
					closest = d
					sampleAt = p.At
				}
			}
			if closest > time.Hour {
				return nil, "no app sample near requested time"
			}
			for _, p := range apps.History {
				if p.At.Equal(sampleAt) {
					add(p.AppSample)
				}
			}
		} else {
			for _, app := range apps.Apps {
				add(app)
			}
		}
		answer = map[string]any{"host": host, "figure": figure, "unit": map[string]string{"cpu": "percent of one core", "memory": "bytes", "disk": "bytes per second", "network": "bytes per second"}[figure], "at": sampleAt, "apps": rows}
	case "what_is_on_port":
		if port < 1 || port > 65535 {
			return nil, "invalid port"
		}
		observations, e := a.Store.Observations(host, time.Now().UTC())
		if e != nil {
			return nil, "data unavailable"
		}
		rows := []map[string]any{}
		listeners := []map[string]any{}
		for _, v := range overview.Results {
			if strings.HasPrefix(v.Name, "port:") && v.Values["port"] == float64(port) && v.State != model.Stale {
				listeners = append(listeners, map[string]any{"name": v.Name, "state": v.State, "at": v.At})
			}
		}
		for _, p := range observations.Projects {
			for _, n := range p.Ports {
				if n == port {
					rows = append(rows, map[string]any{"name": p.Name, "pid": p.PID, "project": p.Project, "worktree": p.Worktree, "kind": p.Kind})
				}
			}
		}
		answer = map[string]any{"host": host, "port": port, "listeners": listeners, "processes": rows, "note": "Process attribution covers configured project roots only; an empty result is not proof that the port is free."}
	case "host_status":
		counts := map[string]int{}
		var uptime any
		var kernel, arch string
		for _, v := range overview.Results {
			counts[string(v.State)]++
			if v.Name == "host" {
				if seconds, ok := v.Values["uptime_seconds"]; ok {
					uptime = seconds
				}
				kernel, arch = v.Fields["kernel_release"], v.Fields["arch"]
			}
		}
		answer = map[string]any{"host": host, "at": overview.At, "liveness": overview.Liveness, "checks": counts, "agents": len(overview.Agents), "uptime_seconds": uptime, "kernel_release": kernel, "arch": arch}
	}
	b, e := json.Marshal(answer)
	if e != nil {
		return nil, "data unavailable"
	}
	return b, ""
}
