package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/store"
)

func TestMCPProtocolReadOnlyAndToken(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "mcp.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	if err := s.Ingest(model.Batch{HostID: "host", At: now, Results: []model.Result{{Name: "cpu", State: model.OK, Values: map[string]float64{"busy_percent": 20, "core_count": 2}}, {Name: "host", State: model.OK, Values: map[string]float64{"uptime_seconds": 3600}, Fields: map[string]string{"kernel_release": "6.12-demo", "arch": "amd64"}}}}); err != nil {
		t.Fatal(err)
	}
	h := New(s, "ingest", "").MCPHandler(strings.Repeat("t", 40))
	request := func(body, token string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	init := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	if got := request(init, ""); got.Code != 401 {
		t.Fatalf("missing token: %d", got.Code)
	}
	if got := request(init, "wrong"); got.Code != 401 {
		t.Fatalf("wrong token: %d", got.Code)
	}
	got := request(init, strings.Repeat("t", 40))
	if got.Code != 200 || !strings.Contains(got.Body.String(), `"protocolVersion":"2025-06-18"`) {
		t.Fatalf("initialize: %d %s", got.Code, got.Body.String())
	}
	got = request(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, strings.Repeat("t", 40))
	var list struct {
		Result struct {
			Tools []struct {
				Name        string          `json:"name"`
				Annotations map[string]bool `json:"annotations"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(got.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Result.Tools) != 4 {
		t.Fatalf("tools: %s", got.Body.String())
	}
	for _, tool := range list.Result.Tools {
		if !tool.Annotations["readOnlyHint"] {
			t.Fatalf("not read only: %s", tool.Name)
		}
	}
	got = request(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"host_status","arguments":{"host":"host"}}}`, strings.Repeat("t", 40))
	if got.Code != 200 || !strings.Contains(got.Body.String(), `uptime_seconds`) || !strings.Contains(got.Body.String(), `6.12-demo`) {
		t.Fatalf("status: %d %s", got.Code, got.Body.String())
	}
	got = request(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"kill_process","arguments":{"host":"host"}}}`, strings.Repeat("t", 40))
	if !strings.Contains(got.Body.String(), `"isError":true`) {
		t.Fatalf("unexpected action: %s", got.Body.String())
	}
	r := httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewBufferString(init))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://other.example")
	r.Header.Set("Authorization", "Bearer "+strings.Repeat("t", 40))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("origin: %d", w.Code)
	}
}

func TestMCPRunningIncludesPossibleLoopFlag(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "mcp-loop.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	now := time.Now().UTC()
	id := strings.Repeat("a", 64)
	session := model.AgentSession{ID: id, Kind: "codex", At: now, StartedAt: now.Add(-time.Hour), State: "working", Source: "otlp", LastTool: "Bash", LastToolAt: now, LastFileEditAt: now.Add(-27 * time.Minute)}
	if err := s.Ingest(model.Batch{HostID: "host", At: now, AI: &model.AI{Sessions: []model.AgentSession{session}}}); err != nil {
		t.Fatal(err)
	}
	data, why := New(s, "", "").mcpCall("what_is_running", "host", "", "", 0)
	if why != "" || !strings.Contains(string(data), `"flag":true`) || !strings.Contains(string(data), "possibly looping") {
		t.Fatalf("%s %s", why, data)
	}
}
