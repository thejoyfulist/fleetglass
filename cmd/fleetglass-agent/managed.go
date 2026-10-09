package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/probe"
)

func (a *agent) managed(ctx context.Context) []model.Result {
	now := time.Now()
	out := []model.Result{}
	for i, set := range a.c.BackupSets {
		r := probe.Backup(set, now)
		if r.State == model.Unknown {
			r = a.managedHelper(ctx, fmt.Sprintf("backup/%d", i), r)
		}
		out = append(out, r)
	}
	for i, file := range a.c.StatusFiles {
		r := probe.Status(file, now)
		if r.State == model.Unknown && r.Detail == "status file unreadable" {
			r = a.managedHelper(ctx, fmt.Sprintf("status/%d", i), r)
		}
		out = append(out, r)
	}
	for i, marker := range a.c.LogMarkers {
		r := probe.Marker(marker, now)
		if r.State == model.Unknown && r.Detail == "log unreadable" {
			r = a.managedHelper(ctx, fmt.Sprintf("log-marker/%d", i), r)
		}
		out = append(out, r)
	}
	out = append(out, probe.Repositories(ctx, a.c.RepositoryRoots, a.c.LocalOnlyRepositories, a.c.ExcludedRepositories, now, nil)...)
	return out
}

func (a *agent) managedHelper(ctx context.Context, endpoint string, fallback model.Result) model.Result {
	if a.c.HelperSocket == "" {
		return fallback
	}
	transport := &http.Transport{DialContext: func(c context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(c, "unix", a.c.HelperSocket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://helper/"+endpoint, nil)
	resp, e := client.Do(req)
	if e != nil {
		return fallback
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fallback
	}
	var r model.Result
	if json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<16)).Decode(&r) != nil || r.Name != fallback.Name {
		return fallback
	}
	return r
}
