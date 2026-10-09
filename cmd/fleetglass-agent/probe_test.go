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

	"github.com/thejoyfulist/fleetglass/internal/api"
	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/probe"
	"github.com/thejoyfulist/fleetglass/internal/store"
)

func TestProbeNeedsTwoConsecutiveFailures(t *testing.T) {
	a := &agent{probeFailures: map[string]int{}}
	for i, step := range []struct{ input, want model.State }{{model.Crit, model.Unknown}, {model.OK, model.OK}, {model.Crit, model.Unknown}, {model.Crit, model.Crit}, {model.OK, model.OK}} {
		if got := a.probeState("site", step.input); got != step.want {
			t.Fatalf("step %d: %s want %s", i, got, step.want)
		}
	}
}

func TestManagedCheckReadFailureNeedsTwoConsecutiveFailures(t *testing.T) {
	a := &agent{probeFailures: map[string]int{}}
	for i, step := range []struct{ input, want model.State }{{model.Unknown, model.Warn}, {model.Unknown, model.Crit}, {model.OK, model.OK}, {model.Crit, model.Warn}, {model.Crit, model.Crit}} {
		if got := a.checkState("backup:fixture", step.input); got != step.want {
			t.Fatalf("step %d: got %s want %s", i, got, step.want)
		}
	}
}

func TestConfiguredHTTPStatusAndBackupFailuresConfirmDown(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	target := listener.Addr().String()
	listener.Close()
	now := time.Now().UTC()
	root := t.TempDir()
	checks := []struct {
		name  string
		state model.State
		first model.State
	}{
		{"http", probe.HTTPCheck(context.Background(), config.HTTPProbe{Name: "http", DisplayName: "HTTP", URL: "http://" + target, Method: "GET", TimeoutMS: 100, IntervalSeconds: 30}, now).State, model.Unknown},
		{"status", probe.Status(config.StatusFile{Name: "missing", Path: filepath.Join(root, "status.json"), MissingState: model.Crit, FieldPath: "state", MaxAgeSeconds: 60}, now).State, model.Warn},
		{"backup", probe.Backup(config.BackupSet{Name: "missing", Pattern: filepath.Join(root, "backup"), MaxAgeSeconds: 60}, now).State, model.Warn},
	}
	a := &agent{probeFailures: map[string]int{}}
	for _, check := range checks {
		if check.state != model.Crit {
			t.Fatalf("%s failure not measured: %s", check.name, check.state)
		}
		confirm := a.checkState
		if check.name == "http" {
			confirm = a.probeState
		}
		if first, second := confirm(check.name, check.state), confirm(check.name, check.state); first != check.first || second != model.Crit {
			t.Fatalf("%s confirmation: %s then %s", check.name, first, second)
		}
	}
}

func TestClosedInternalTCPReachesOverviewAndAPI(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	target := listener.Addr().String()
	listener.Close()
	s, err := store.Open(filepath.Join(t.TempDir(), "fleetglass.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := &agent{probeFailures: map[string]int{}}
	p := config.InternalProbe{Name: "closed", DisplayName: "Closed target", Kind: "tcp", Target: target, TimeoutMS: 100}
	for i, want := range []model.State{model.Warn, model.Crit} {
		at := time.Now().UTC()
		r := a.collectInternal(p, at)
		if r.State != want || r.CadenceSeconds != 300 {
			t.Fatalf("check %d: %+v", i+1, r)
		}
		if err := s.Ingest(model.Batch{HostID: "fixture", At: at, Results: []model.Result{r}, Probes: a.pendingProbes}); err != nil {
			t.Fatal(err)
		}
		a.pendingProbes = nil
	}
	server := httptest.NewServer(api.New(s, "test-token", "fixture").Handler())
	defer server.Close()
	resp, err := http.Get(server.URL + "/api/v1/hosts/fixture/overview")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var overview model.Overview
	if err := json.NewDecoder(resp.Body).Decode(&overview); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || len(overview.Results) != 1 || overview.Results[0].State != model.Crit {
		t.Fatalf("overview response: status=%d results=%+v", resp.StatusCode, overview.Results)
	}
	resp2, err := http.Get(server.URL + "/api/v1/hosts/fixture/probes")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var probes []store.ProbeSummary
	if err := json.NewDecoder(resp2.Body).Decode(&probes); err != nil {
		t.Fatal(err)
	}
	if len(probes) != 1 || probes[0].State != model.Crit {
		t.Fatalf("probe response: %+v", probes)
	}
	if urlFile := os.Getenv("FLEETGLASS_E2E_URL_FILE"); urlFile != "" {
		doneFile := os.Getenv("FLEETGLASS_E2E_DONE_FILE")
		if doneFile == "" {
			t.Fatal("missing E2E completion path")
		}
		if err := os.WriteFile(urlFile, []byte(server.URL), 0600); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(doneFile); err == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("cockpit verification did not complete")
	}
}
