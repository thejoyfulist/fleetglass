package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/projects"
)

func (a *agent) scanStorage() []model.StorageEntry {
	out := []model.StorageEntry{}
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", a.c.HelperSocket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 22 * time.Second}
	for i, root := range a.c.StorageRoots {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		entries := projects.ScanIONice(ctx, root)
		cancel()
		complete := len(entries) > 0
		for _, entry := range entries {
			if !entry.Complete {
				complete = false
				break
			}
		}
		if !complete && a.c.HelperSocket != "" {
			req, _ := http.NewRequest("GET", fmt.Sprintf("http://helper/storage/%d", i), nil)
			resp, err := client.Do(req)
			if err == nil {
				if resp.StatusCode == 200 {
					var privileged []model.StorageEntry
					if json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&privileged) == nil && len(privileged) > 0 {
						entries = privileged
					}
				}
				resp.Body.Close()
			}
		}
		out = append(out, entries...)
	}
	return out
}

func (a *agent) collectProjects(sessions []model.AgentSession, now time.Time) []model.ProjectProcess {
	if len(a.c.ProjectRoots)+len(a.c.WorktreeRoots) == 0 {
		return nil
	}
	if a.c.HelperSocket != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", a.c.HelperSocket)
		}}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://helper/projects", nil)
		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				var samples []model.ProjectProcess
				if json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&samples) == nil {
					active := map[int]bool{}
					for _, s := range sessions {
						if s.PID > 0 && s.State != "ended" {
							active[s.PID] = true
						}
					}
					for i := range samples {
						if active[samples[i].PID] {
							samples[i].Kind = "agent"
						}
					}
					return samples
				}
			}
		}
	}
	return a.projectCollector.Measure(a.c.ProcRoot, a.c.ProjectRoots, a.c.WorktreeRoots, sessions, now)
}
