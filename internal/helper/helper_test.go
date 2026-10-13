package helper

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestAllowlistAndSmartSummary(t *testing.T) {
	calls := 0
	h := Handler{Devices: []string{"/dev/nvme0"}, Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls++
		if name != "smartctl" || strings.Join(args, " ") != "-j -H -A /dev/nvme0" {
			t.Fatalf("unexpected command %s %v", name, args)
		}
		return []byte(`{"smart_status":{"passed":true},"nvme_smart_health_information_log":{"percentage_used":5,"media_errors":0}}`), nil
	}}
	for _, path := range []string{"/smart/1", "/smart/-1", "/smart/../../etc/passwd", "/anything"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	if calls != 0 {
		t.Fatal("ran command for forbidden path")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/smart/0", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"wear_percent":5`) {
		t.Fatalf("SMART: %d %s", w.Code, w.Body.String())
	}
	if calls != 1 {
		t.Fatal(calls)
	}
}
func TestValidateDevices(t *testing.T) {
	if e := (Handler{Devices: []string{"/dev/nvme0;bad"}}).Validate(); e == nil {
		t.Fatal("accepted unsafe device")
	}
}

func TestSMARTCommandOutputIsBounded(t *testing.T) {
	output, err := runBounded(context.Background(), "sh", "-c", "head -c 1048577 /dev/zero")
	if err == nil || len(output) > maxSMARTOutput {
		t.Fatalf("oversized command output: bytes=%d error=%v", len(output), err)
	}
}

func TestSMARTRejectsTruncatedOutput(t *testing.T) {
	h := Handler{Devices: []string{"/dev/nvme0"}, Run: func(context.Context, string, ...string) ([]byte, error) {
		return []byte(`{"smart_status":{"passed":true}}`), errOutputLimit
	}}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/smart/0", nil))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("truncated SMART output: %d", w.Code)
	}
}

func TestDockerRouteReturnsOnlySummary(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "docker.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.String() != "/containers/json?all=1" {
			t.Errorf("unexpected Docker request: %s %s", r.Method, r.URL)
			http.Error(w, "bad request", 400)
			return
		}
		w.Write([]byte(`[{"Names":["/web"],"State":"running","Status":"Up 3 hours","Health":{"Status":"healthy","FailingStreak":0},"RestartCount":2,"Env":["SECRET=value"],"Command":"private","Labels":{"secret":"value"}}]`))
	})}
	go server.Serve(l)
	defer server.Close()
	h := Handler{DockerSockets: []string{socket}}
	for _, path := range []string{"/docker/1", "/docker/-1", "/docker/0/extra", "/docker/0?all=0"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/docker/0", nil))
	if w.Code != 200 {
		t.Fatalf("Docker: %d %s", w.Code, w.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || len(rows[0]) != 4 || rows[0]["name"] != "web" || rows[0]["health"] != "healthy" || rows[0]["restart_count"] != float64(2) {
		t.Fatalf("summary: %+v", rows)
	}
	if strings.Contains(w.Body.String(), "SECRET") || strings.Contains(w.Body.String(), "private") {
		t.Fatal("sensitive Docker fields leaked")
	}
}

func TestDockerInvalidResponseIsBadGateway(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "docker.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"not":"a list"}`))
	})}
	go server.Serve(l)
	defer server.Close()
	w := httptest.NewRecorder()
	Handler{DockerSockets: []string{socket}}.ServeHTTP(w, httptest.NewRequest("GET", "/docker/0", nil))
	if w.Code != http.StatusBadGateway {
		t.Fatalf("invalid Docker JSON: %d", w.Code)
	}
}
