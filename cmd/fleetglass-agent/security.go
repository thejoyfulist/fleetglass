package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/helper"
	"github.com/thejoyfulist/fleetglass/internal/model"
)

func (a *agent) sensitive(ctx context.Context) []model.Result {
	unknown := []model.Result{{Name: "security:sensitive_files", State: model.Unknown, Detail: "sensitive file helper unavailable"}}
	if a.c.HelperSocket == "" {
		return unknown
	}
	transport := &http.Transport{DialContext: func(c context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(c, "unix", a.c.HelperSocket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://helper/sensitive-files", nil)
	resp, err := client.Do(req)
	if err != nil {
		return unknown
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return unknown
	}
	var files []helper.SensitiveFile
	if json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(&files) != nil {
		return unknown
	}
	if a.fileHashes == nil {
		a.fileHashes = map[string]string{}
	}
	next := map[string]string{}
	out := []model.Result{}
	changes := 0
	for i, f := range files {
		next[f.Path] = f.SHA256
		if prior, ok := a.fileHashes[f.Path]; ok && prior != f.SHA256 {
			changes++
			out = append(out, model.Result{Name: "security:file_change:" + strconv.Itoa(i), State: model.Warn, Detail: "sensitive file changed since previous sample", Values: map[string]float64{"size": float64(f.Size)}, Fields: map[string]string{"path": f.Path, "mtime_ns": strconv.FormatInt(f.MtimeNS, 10), "sha256": f.SHA256}})
		}
	}
	for path := range a.fileHashes {
		if _, ok := next[path]; !ok {
			changes++
			out = append(out, model.Result{Name: "security:file_change:removed:" + strconv.Itoa(len(out)), State: model.Warn, Detail: "sensitive file removed since previous sample", Fields: map[string]string{"path": path}})
		}
	}
	a.fileHashes = next
	state := model.OK
	if changes > 0 {
		state = model.Warn
	}
	out = append(out, model.Result{Name: "security:sensitive_files", State: state, Detail: "fixed sensitive file list", Values: map[string]float64{"monitored": float64(len(files)), "changes": float64(changes)}})
	return out
}

type restartEvent struct {
	at    time.Time
	count float64
}

func (a *agent) withDockerRestarts(results []model.Result) []model.Result {
	now := time.Now().UTC()
	cutoff := now.Add(-24 * time.Hour)
	recent := a.dockerEvents[:0]
	for _, at := range a.dockerEvents {
		if at.at.After(cutoff) {
			recent = append(recent, at)
		}
	}
	a.dockerEvents = recent
	first := a.dockerRestarts == nil
	if first {
		a.dockerRestarts = map[string]float64{}
	}
	known := false
	for _, r := range results {
		if !strings.HasPrefix(r.Name, "container:") || r.State == model.Unknown {
			continue
		}
		count, ok := r.Values["restart_count"]
		if !ok {
			continue
		}
		known = true
		if old, seen := a.dockerRestarts[r.Name]; seen && count > old {
			a.dockerEvents = append(a.dockerEvents, restartEvent{at: now, count: count - old})
		}
		a.dockerRestarts[r.Name] = count
	}
	if known && !first {
		state := model.OK
		total := 0.0
		for _, event := range a.dockerEvents {
			total += event.count
		}
		if total > 0 {
			state = model.Warn
		}
		return append(results, model.Result{Name: "recent_errors:container_restarts", State: state, Detail: "Docker restarts observed in the last 24 h", Values: map[string]float64{"count": total}})
	}
	return append(results, model.Result{Name: "recent_errors:container_restarts", State: model.Unknown, Detail: "Docker restart baseline unavailable"})
}
