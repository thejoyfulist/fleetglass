package helper

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/dockerapi"
	"github.com/thejoyfulist/fleetglass/internal/probe"
	"github.com/thejoyfulist/fleetglass/internal/projects"
	"github.com/thejoyfulist/fleetglass/internal/rapl"
)

func imageFingerprint(id string) string {
	if id == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:8])
}

type Run func(context.Context, string, ...string) ([]byte, error)

const maxSMARTOutput = 1 << 20

var errOutputLimit = errors.New("command output limit exceeded")

type boundedOutput struct {
	buf   bytes.Buffer
	limit int
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.buf.Len() {
		return 0, errOutputLimit
	}
	return b.buf.Write(p)
}

func runBounded(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out := &boundedOutput{limit: maxSMARTOutput}
	cmd.Stdout = out
	err := cmd.Run()
	return out.buf.Bytes(), err
}

type Handler struct {
	Devices          []string
	DockerSockets    []string
	SysRoot          string
	Run              Run
	ProcRoot         string
	AgentCommands    []string
	BackupSets       []config.BackupSet
	StatusFiles      []config.StatusFile
	LogMarkers       []config.LogMarker
	StorageRoots     []string
	ProjectRoots     []string
	WorktreeRoots    []string
	ProjectCollector *projects.Collector
	ProjectMu        *sync.Mutex
}
type Smart struct {
	Device          string   `json:"device"`
	Healthy         *bool    `json:"healthy,omitempty"`
	WearPercent     *float64 `json:"wear_percent,omitempty"`
	Temperature     *float64 `json:"temperature_celsius,omitempty"`
	PowerOnHours    *float64 `json:"power_on_hours,omitempty"`
	MediaErrors     *float64 `json:"media_errors,omitempty"`
	UnsafeShutdowns *float64 `json:"unsafe_shutdowns,omitempty"`
}

func number(m map[string]any, key string) *float64 {
	v, ok := m[key].(float64)
	if !ok {
		return nil
	}
	return &v
}
func smartSummary(b []byte, device string) (Smart, error) {
	s := Smart{Device: device}
	var m map[string]any
	if e := json.Unmarshal(b, &m); e != nil {
		return s, e
	}
	if x, ok := m["smart_status"].(map[string]any); ok {
		if v, ok := x["passed"].(bool); ok {
			s.Healthy = &v
		}
	}
	if x, ok := m["nvme_smart_health_information_log"].(map[string]any); ok {
		s.WearPercent = number(x, "percentage_used")
		s.PowerOnHours = number(x, "power_on_hours")
		s.MediaErrors = number(x, "media_errors")
		s.UnsafeShutdowns = number(x, "unsafe_shutdowns")
		s.Temperature = number(x, "temperature")
	}
	if x, ok := m["temperature"].(map[string]any); ok {
		s.Temperature = number(x, "current")
	}
	if x, ok := m["power_on_time"].(map[string]any); ok {
		s.PowerOnHours = number(x, "hours")
	}
	return s, nil
}
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.URL.Path == "/projects" && r.URL.RawQuery == "":
		if h.ProjectCollector == nil || h.ProjectMu == nil {
			http.Error(w, "project collector unavailable", 503)
			return
		}
		h.ProjectMu.Lock()
		v := h.ProjectCollector.Measure(h.ProcRoot, h.ProjectRoots, h.WorktreeRoots, nil, time.Now().UTC())
		h.ProjectMu.Unlock()
		json.NewEncoder(w).Encode(v)
	case strings.HasPrefix(r.URL.Path, "/storage/") && r.URL.RawQuery == "":
		idx, e := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/storage/"))
		if e != nil || idx < 0 || idx >= len(h.StorageRoots) {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		json.NewEncoder(w).Encode(projects.ScanIONice(ctx, h.StorageRoots[idx]))
	case strings.HasPrefix(r.URL.Path, "/log-marker-events/") && r.URL.RawQuery == "":
		idx, e := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/log-marker-events/"))
		if e != nil || idx < 0 || idx >= len(h.LogMarkers) || !h.LogMarkers[idx].ChangeEvent {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(probe.MarkerChanges(h.LogMarkers[idx], time.Now()))
	case strings.HasPrefix(r.URL.Path, "/backup/") && r.URL.RawQuery == "":
		idx, e := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/backup/"))
		if e != nil || idx < 0 || idx >= len(h.BackupSets) {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(probe.Backup(h.BackupSets[idx], time.Now()))
	case strings.HasPrefix(r.URL.Path, "/status/") && r.URL.RawQuery == "":
		idx, e := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/status/"))
		if e != nil || idx < 0 || idx >= len(h.StatusFiles) {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(probe.Status(h.StatusFiles[idx], time.Now()))
	case strings.HasPrefix(r.URL.Path, "/log-marker/") && r.URL.RawQuery == "":
		idx, e := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/log-marker/"))
		if e != nil || idx < 0 || idx >= len(h.LogMarkers) {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(probe.Marker(h.LogMarkers[idx], time.Now()))
	case r.URL.Path == "/agents" && r.URL.RawQuery == "":
		json.NewEncoder(w).Encode(Processes(h.ProcRoot, h.AgentCommands))
	case r.URL.Path == "/sensitive-files" && r.URL.RawQuery == "":
		files, err := SensitiveFiles("")
		if err != nil {
			http.Error(w, "sensitive files unavailable", 503)
			return
		}
		json.NewEncoder(w).Encode(files)
	case r.URL.Path == "/rapl":
		sample, err := rapl.Read(h.SysRoot)
		if err != nil {
			http.Error(w, "RAPL unavailable", 503)
			return
		}
		json.NewEncoder(w).Encode(sample)
	case strings.HasPrefix(r.URL.Path, "/smart/"):
		idx, e := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/smart/"))
		if e != nil || idx < 0 || idx >= len(h.Devices) {
			http.NotFound(w, r)
			return
		}
		run := h.Run
		if run == nil {
			run = runBounded
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		b, e := run(ctx, "smartctl", "-j", "-H", "-A", h.Devices[idx])
		if errors.Is(e, errOutputLimit) {
			http.Error(w, "SMART output too large", http.StatusBadGateway)
			return
		}
		if e != nil && len(b) == 0 {
			http.Error(w, "SMART unavailable", 503)
			return
		}
		s, e := smartSummary(b, h.Devices[idx])
		if e != nil {
			http.Error(w, "SMART invalid", 503)
			return
		}
		json.NewEncoder(w).Encode(s)
	case strings.HasPrefix(r.URL.Path, "/docker/"):
		idx, e := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/docker/"))
		if e != nil || idx < 0 || idx >= len(h.DockerSockets) || r.URL.RawQuery != "" {
			http.NotFound(w, r)
			return
		}
		transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", h.DockerSockets[idx])
		}}
		defer transport.CloseIdleConnections()
		client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		req, _ := http.NewRequestWithContext(ctx, "GET", "http://docker/containers/json?all=1", nil)
		resp, e := client.Do(req)
		if e != nil {
			http.Error(w, "Docker unavailable", 503)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			http.Error(w, "Docker unavailable", 503)
			return
		}
		var raw []struct {
			Names        []string         `json:"Names"`
			State        string           `json:"State"`
			Status       string           `json:"Status"`
			Health       dockerapi.Health `json:"Health"`
			RestartCount int              `json:"RestartCount"`
			ImageID      string           `json:"ImageID"`
		}
		if e := json.NewDecoder(http.MaxBytesReader(w, resp.Body, 4<<20)).Decode(&raw); e != nil {
			http.Error(w, "Docker response invalid", http.StatusBadGateway)
			return
		}
		summaries := make([]struct {
			Name         string `json:"name"`
			State        string `json:"state"`
			Health       string `json:"health"`
			RestartCount int    `json:"restart_count"`
			ImageHash    string `json:"image_hash,omitempty"`
		}, 0, len(raw))
		for _, c := range raw {
			name := "unnamed"
			if len(c.Names) > 0 {
				name = strings.TrimPrefix(c.Names[0], "/")
			}
			health := string(c.Health)
			if health == "" {
				if strings.Contains(c.Status, "unhealthy") {
					health = "unhealthy"
				} else if strings.Contains(c.Status, "healthy") {
					health = "healthy"
				}
			}
			summaries = append(summaries, struct {
				Name         string `json:"name"`
				State        string `json:"state"`
				Health       string `json:"health"`
				RestartCount int    `json:"restart_count"`
				ImageHash    string `json:"image_hash,omitempty"`
			}{name, c.State, health, c.RestartCount, imageFingerprint(c.ImageID)})
		}
		json.NewEncoder(w).Encode(summaries)
	default:
		http.NotFound(w, r)
	}
}

var devicePattern = regexp.MustCompile(`^/dev/(nvme[0-9]+(n[0-9]+)?|sd[a-z]+)$`)

func (h Handler) Validate() error {
	if !filepath.IsAbs(h.ProcRoot) || filepath.Clean(h.ProcRoot) != h.ProcRoot {
		return fmt.Errorf("invalid proc root")
	}
	for _, comm := range h.AgentCommands {
		if !commPattern.MatchString(comm) {
			return fmt.Errorf("invalid agent command")
		}
	}
	for _, d := range h.Devices {
		if !devicePattern.MatchString(d) {
			return fmt.Errorf("invalid SMART device")
		}
	}
	for _, socket := range h.DockerSockets {
		if !filepath.IsAbs(socket) || filepath.Clean(socket) != socket {
			return fmt.Errorf("invalid Docker socket path")
		}
	}
	for _, root := range h.StorageRoots {
		if !filepath.IsAbs(root) || filepath.Clean(root) != root {
			return fmt.Errorf("invalid storage root")
		}
	}
	for _, roots := range [][]string{h.ProjectRoots, h.WorktreeRoots} {
		for _, root := range roots {
			if !filepath.IsAbs(root) || filepath.Clean(root) != root {
				return fmt.Errorf("invalid project root")
			}
		}
	}
	return nil
}
