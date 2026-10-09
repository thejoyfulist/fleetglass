package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/probe"
)

func source(name string, ok bool, detail string) model.Result {
	state := model.OK
	if !ok {
		state = model.Unknown
	}
	return model.Result{Name: "changes:" + name, State: state, Detail: detail, CadenceSeconds: 300}
}

func (a *agent) changes(ctx context.Context) ([]model.Change, []model.Result) {
	now := time.Now()
	out := []model.Change{}
	states := []model.Result{}
	if !readableAPTHistory(a.c.APTHistoryPath) {
		states = append(states, source("apt", false, "APT history unreadable"))
	} else {
		out = append(out, probe.APTChanges(a.c.APTHistoryPath, now)...)
		states = append(states, source("apt", true, "APT history scanned"))
	}
	if b, e := os.ReadFile(filepath.Join(a.c.ProcRoot, "stat")); e != nil || !strings.Contains(string(b), "btime ") {
		states = append(states, source("boot", false, "boot time unreadable"))
	} else {
		out = append(out, probe.BootChange(a.c.ProcRoot, now)...)
		states = append(states, source("boot", true, "boot time observed"))
	}
	if len(a.c.RepositoryRoots) > 0 {
		ok := true
		for _, r := range a.c.RepositoryRoots {
			if _, e := os.ReadDir(r); e != nil {
				ok = false
			}
		}
		out = append(out, probe.MergeChanges(ctx, a.c.RepositoryRoots, a.c.ExcludedRepositories, now, nil)...)
		states = append(states, source("git", ok, "local merge history scanned"))
		if !ok {
			states[len(states)-1].Detail = "repository root unreadable"
		}
	}
	if len(a.c.SystemdUnits) > 0 {
		args := []string{"-o", "json", "--no-pager", "--since=-7d", "-n", "1000"}
		for _, u := range a.c.SystemdUnits {
			args = append(args, "-u", u)
		}
		if raw, e := probe.Exec(ctx, "journalctl", args...); e == nil {
			if len(raw) > 1<<20 {
				raw = raw[len(raw)-(1<<20):]
			}
			out = append(out, probe.JournalChanges(raw, a.c.SystemdUnits, now)...)
			states = append(states, source("journal", true, "watched unit history scanned"))
		} else {
			states = append(states, source("journal", false, "journal unreadable"))
		}
	}
	for i, m := range a.c.LogMarkers {
		if !m.ChangeEvent {
			continue
		}
		if r := probe.Marker(m, now); r.State == model.Unknown && r.Detail == "log unreadable" {
			out = append(out, a.helperMarkerChanges(ctx, i)...)
		} else {
			out = append(out, probe.MarkerChanges(m, now)...)
		}
	}
	if len(out) > 400 {
		out = out[len(out)-400:]
	}
	return out, states
}

func readableAPTHistory(root string) bool {
	paths := []string{filepath.Join(root, "history.log"), filepath.Join(root, "history.log.1")}
	gz, _ := filepath.Glob(filepath.Join(root, "history.log.*.gz"))
	paths = append(paths, gz...)
	for _, p := range paths {
		f, e := os.Open(p)
		if e == nil {
			f.Close()
			return true
		}
	}
	return false
}

func (a *agent) helperMarkerChanges(ctx context.Context, i int) []model.Change {
	if a.c.HelperSocket == "" {
		return nil
	}
	transport := &http.Transport{DialContext: func(c context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(c, "unix", a.c.HelperSocket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://helper/log-marker-events/%d", i), nil)
	resp, e := client.Do(req)
	if e != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	var out []model.Change
	if json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<16)).Decode(&out) != nil {
		return nil
	}
	return out
}
