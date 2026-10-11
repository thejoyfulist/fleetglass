package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/ai"
	"github.com/thejoyfulist/fleetglass/internal/api"
	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/probe"
	"github.com/thejoyfulist/fleetglass/internal/rapl"
	"github.com/thejoyfulist/fleetglass/internal/store"
)

func TestInactiveLogSessionReachesServerAsEnded(t *testing.T) {
	root := t.TempDir()
	logRoot := filepath.Join(root, "logs")
	if err := os.Mkdir(logRoot, 0700); err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC().Add(-31 * time.Minute)
	line := `{"timestamp":"` + at.Format(time.RFC3339Nano) + `","sessionId":"fixture-session","type":"assistant","message":{"id":"fixture-message"}}` + "\n"
	if err := os.WriteFile(filepath.Join(logRoot, "session.jsonl"), []byte(line), 0600); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(filepath.Join(root, "fleetglass.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	server := httptest.NewServer(api.New(s, "test-token", "fixture-host").Handler())
	defer server.Close()
	a := &agent{c: config.Config{HostID: "fixture-host", ServerURL: server.URL, Token: fixture("test-token"), ProcRoot: filepath.Join(root, "proc"), HelperSocket: filepath.Join(root, "missing.sock")}, client: server.Client(), receiver: ai.NewReceiver(), logs: ai.NewLogReader([]ai.Root{{Kind: "claude", Path: logRoot}})}
	a.send(nil)
	resp, err := server.Client().Get(server.URL + "/api/v1/hosts/fixture-host/agents")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var sessions []model.AgentSession
	if err := json.NewDecoder(resp.Body).Decode(&sessions); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || len(sessions) != 1 || sessions[0].PID != 0 || sessions[0].State != "ended" || sessions[0].Reason != "no process matched; no activity for over 30 minutes" {
		t.Fatalf("inactive session after agent and server: status=%d sessions=%+v", resp.StatusCode, sessions)
	}
}

func TestPowerUsesHelperWhenDirectReadFails(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "helper.sock")
	l, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	energy := uint64(1000000)
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rapl" {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(rapl.Sample{Domains: []rapl.Domain{{ID: "intel-rapl:0", Energy: energy, Max: 10000000}}})
		energy += 1000000
	})}
	go server.Serve(l)
	defer server.Close()
	a := &agent{c: config.Config{SysRoot: t.TempDir(), HelperSocket: socket}, machine: probe.NewMachine(t.TempDir(), t.TempDir())}
	if r := a.power(context.Background()); r.State != "unknown" {
		t.Fatalf("first sample: %+v", r)
	}
	time.Sleep(time.Millisecond)
	if r := a.power(context.Background()); r.State != "ok" || r.Values["watts"] <= 0 {
		t.Fatalf("helper power: %+v", r)
	}
}

func TestPowerPrefersDirectRead(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "class/powercap/intel-rapl:0")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"name": "package-0", "energy_uj": "100", "max_energy_range_uj": "1000"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0600); err != nil {
			t.Fatal(err)
		}
	}
	a := &agent{c: config.Config{SysRoot: root, HelperSocket: filepath.Join(t.TempDir(), "absent.sock")}, machine: probe.NewMachine(t.TempDir(), root)}
	if r := a.power(context.Background()); r.Detail != "waiting for RAPL delta" {
		t.Fatalf("direct sample: %+v", r)
	}
	time.Sleep(time.Millisecond)
	if err := os.WriteFile(filepath.Join(dir, "energy_uj"), []byte("200"), 0600); err != nil {
		t.Fatal(err)
	}
	if r := a.power(context.Background()); r.State != "ok" {
		t.Fatalf("direct power: %+v", r)
	}
}
