package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/store"
)

func localRequest(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.Host = "127.0.0.1:7878"
	return r
}

func TestIngestAndReadAPI(t *testing.T) {
	s, e := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	h := New(s, "secret", "test").Handler()
	b, _ := json.Marshal(model.Batch{HostID: "test", At: time.Now().UTC(), Results: []model.Result{{Name: "cpu", State: model.OK, Detail: "busy", Values: map[string]float64{"busy_percent": 20}}}})
	r := localRequest("POST", "/api/v1/ingest", bytes.NewReader(b))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 401 {
		t.Fatalf("missing token: %d", w.Code)
	}
	r = localRequest("POST", "/api/v1/ingest", bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer secret")
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 204 {
		t.Fatalf("ingest: %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/v1/hosts", "/api/v1/hosts/test/overview", "/api/v1/hosts/test/series?metric=busy_percent&range=24h"} {
		w = httptest.NewRecorder()
		h.ServeHTTP(w, localRequest("GET", path, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), "test") && !strings.Contains(path, "series") {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
}

func TestAPIHostAllowlist(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "host-guard.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "", "")
	for _, tc := range []struct {
		host string
		want int
	}{
		{"attacker.example:7878", http.StatusForbidden},
		{"127.0.0.1:7878", http.StatusOK},
		{"cockpit.example:8444", http.StatusForbidden},
	} {
		r := localRequest(http.MethodGet, "/api/v1/hosts", nil)
		r.Host = tc.host
		w := httptest.NewRecorder()
		a.Handler().ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("host %q: got %d, want %d", tc.host, w.Code, tc.want)
		}
	}
	a.AllowedHosts = []string{"cockpit.example"}
	r := localRequest(http.MethodGet, "/api/v1/hosts", nil)
	r.Host = "cockpit.example:8444"
	w := httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("configured host: %d", w.Code)
	}
	r = localRequest(http.MethodGet, "/api/v1/hosts", nil)
	r.Header.Set("Origin", "https://attacker.example")
	w = httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin request: %d", w.Code)
	}
	r.Header.Set("Origin", "https://127.0.0.1:7878")
	w = httptest.NewRecorder()
	a.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("same-origin request: %d", w.Code)
	}
}
func TestSSEDelivery(t *testing.T) {
	s, e := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	server := httptest.NewServer(New(s, "secret", "h").Handler())
	defer server.Close()
	resp, e := http.Get(server.URL + "/api/v1/live")
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 4096)
		all := ""
		for {
			n, e := resp.Body.Read(buf)
			all += string(buf[:n])
			if strings.Contains(all, "event: overview") || e != nil {
				done <- all
				return
			}
		}
	}()
	now := time.Now().UTC()
	b, _ := json.Marshal(model.Batch{HostID: "h", At: now, Results: []model.Result{{Name: "cpu", State: model.OK, Detail: "busy"}}, AI: &model.AI{Sessions: []model.AgentSession{{ID: strings.Repeat("a", 64), Kind: "claude", Project: "demo", At: now, Source: "otlp", State: "working", Reason: "recent activity"}}}})
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/ingest", bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer secret")
	r, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	select {
	case text := <-done:
		if !strings.Contains(text, "event: overview") || !strings.Contains(text, `"agents"`) {
			t.Fatal(text)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SSE timeout")
	}
}

func TestReadTokenProtectsAllReads(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "ingest-secret", "")
	a.ReadToken = fixture("read-secret")
	h := a.Handler()
	for _, path := range []string{"/api/v1/hosts", "/api/v1/hosts/h/overview", "/api/v1/hosts/h/series?metric=cpu", "/api/v1/hosts/h/agents", "/api/v1/hosts/h/ai/usage?range=24h&by=project", "/api/v1/hosts/h/ai/quotas", "/api/v1/hosts/h/ai/costs?range=24h", "/api/v1/hosts/h/changes?range=24h", "/api/v1/hosts/h/energy", "/api/v1/live"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, localRequest("GET", path, nil))
		if w.Code != 401 {
			t.Fatalf("%s unprotected: %d", path, w.Code)
		}
	}
	for _, path := range []string{"/api/v1/hosts", "/"} {
		r := localRequest("GET", path, nil)
		r.Header.Set("Authorization", "Bearer read-secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("%s with read token: %d", path, w.Code)
		}
	}
}

func TestSSESubscriberLimitAndKeepAlive(t *testing.T) {
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := New(s, "ingest-secret", "")
	a.MaxSubscribers = 1
	a.KeepAlive = 10 * time.Millisecond
	server := httptest.NewServer(a.Handler())
	defer server.Close()
	first, err := http.Get(server.URL + "/api/v1/live")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Body.Close()
	second, err := http.Get(server.URL + "/api/v1/live")
	if err != nil {
		t.Fatal(err)
	}
	second.Body.Close()
	if second.StatusCode != 503 {
		t.Fatalf("second subscriber: %d", second.StatusCode)
	}
	buf := make([]byte, 128)
	done := make(chan string, 1)
	go func() {
		all := ""
		for {
			n, err := first.Body.Read(buf)
			all += string(buf[:n])
			if strings.Contains(all, ": keep-alive") || err != nil {
				done <- all
				return
			}
		}
	}()
	select {
	case got := <-done:
		if !strings.Contains(got, ": keep-alive") {
			t.Fatal(got)
		}
	case <-time.After(time.Second):
		t.Fatal("no keep-alive")
	}
}
