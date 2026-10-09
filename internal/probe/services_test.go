package probe

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestSystemdRecordedOutput(t *testing.T) {
	s := Services{Units: []string{"ssh.service"}, Run: func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[0] == "list-units" {
			return []byte(`[{"unit":"broken.service","active":"failed","sub":"failed"}]`), nil
		}
		return []byte("ActiveState=active\nSubState=running\n"), nil
	}}
	r := s.Check(context.Background())
	if len(r) != 3 || r[1].Name != "systemd:broken.service" || r[1].State != "crit" || r[2].State != "ok" {
		t.Fatalf("systemd: %+v", r)
	}
	if r[2].Detail != "active (running)" {
		t.Fatalf("systemd detail: %q", r[2].Detail)
	}
}
func TestDockerUnixSocket(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "docker.sock")
	l, e := net.Listen("unix", socket)
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/containers/json" || r.URL.Query().Get("all") != "1" {
			t.Errorf("request %s", r.URL)
			http.Error(w, "wrong path", 400)
			return
		}
		json.NewEncoder(w).Encode([]map[string]any{{"Id": "abc", "Names": []string{"/web"}, "State": "running", "Status": "Up 3 hours", "RestartCount": 2, "Health": map[string]any{"Status": "unhealthy", "FailingStreak": 4}}})
	})}
	go server.Serve(l)
	defer server.Close()
	r := (Docker{Sockets: []string{socket}}).Check(context.Background())
	if len(r) != 2 || r[1].Name != "container:0:web" || r[1].State != "crit" || r[1].Values["restart_count"] != 2 {
		t.Fatalf("Docker: %+v", r)
	}
	os.Remove(socket)
}

func TestDockerPermissionFallback(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "helper.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/docker/0" {
			t.Errorf("helper path: %s", r.URL.Path)
		}
		w.Write([]byte(`[{"name":"web","state":"running","health":"unhealthy","restart_count":3}]`))
	})}
	go server.Serve(l)
	defer server.Close()
	d := Docker{Sockets: []string{"/run/rootless/docker.sock"}, HelperSocket: socket, Dial: func(context.Context, string) (net.Conn, error) { return nil, os.ErrPermission }}
	r := d.Check(context.Background())
	if len(r) != 2 || r[0].State != "ok" || r[1].Name != "container:0:web" || r[1].State != "crit" || r[1].Values["restart_count"] != 3 {
		t.Fatalf("fallback: %+v", r)
	}
}

func TestDockerHelperFailureDetails(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"daemon stopped", 503, "Docker unavailable", "Docker daemon not reachable"},
		{"bad status", 502, "bad gateway", "invalid Docker helper response"},
		{"bad JSON", 200, "{", "invalid Docker helper response"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			socket := filepath.Join(t.TempDir(), "helper.sock")
			listener, err := net.Listen("unix", socket)
			if err != nil {
				t.Fatal(err)
			}
			server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) })}
			go server.Serve(listener)
			defer server.Close()
			d := Docker{Sockets: []string{"/run/docker.sock"}, HelperSocket: socket, Dial: func(context.Context, string) (net.Conn, error) { return nil, os.ErrPermission }}
			got := d.Check(context.Background())
			if len(got) != 1 || got[0].Detail != tc.want {
				t.Fatalf("result: %+v", got)
			}
		})
	}
	d := Docker{Sockets: []string{"/run/docker.sock"}, HelperSocket: filepath.Join(t.TempDir(), "absent.sock"), Dial: func(context.Context, string) (net.Conn, error) { return nil, os.ErrPermission }}
	got := d.Check(context.Background())
	if len(got) != 1 || got[0].Detail != "Docker helper unavailable" {
		t.Fatalf("unreachable helper: %+v", got)
	}
}
