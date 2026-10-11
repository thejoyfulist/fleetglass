package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type Process struct {
	PID       int    `json:"pid"`
	PPID      int    `json:"ppid"`
	UID       uint32 `json:"uid"`
	StartTime uint64 `json:"start_time"`
	CWD       string `json:"cwd"`
	Comm      string `json:"-"`
}

func HydrateComms(root string, processes []Process) {
	for i := range processes {
		b, e := os.ReadFile(filepath.Join(root, strconv.Itoa(processes[i].PID), "comm"))
		if e != nil {
			continue
		}
		v := strings.TrimSpace(string(b))
		if len(v) <= 15 && (strings.HasPrefix(v, "claude") || strings.HasPrefix(v, "codex")) {
			processes[i].Comm = v
		}
	}
}

func HelperProcesses(ctx context.Context, socket string) []Process {
	if socket == "" {
		return nil
	}
	tr := &http.Transport{DialContext: func(c context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(c, "unix", socket)
	}}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 3 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://helper/agents", nil)
	resp, e := client.Do(req)
	if e != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	var out []Process
	if json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(&out) != nil {
		return nil
	}
	return out
}
func Project(cwd string, orcaRoot string) (string, string) {
	cwd = filepath.Clean(cwd)
	if orcaRoot != "" {
		rel, e := filepath.Rel(orcaRoot, cwd)
		if e == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) >= 2 {
				return parts[0], parts[1]
			}
		}
	}
	p := cwd
	for {
		if info, e := os.Stat(filepath.Join(p, ".git")); e == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return filepath.Base(p), ""
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	return filepath.Base(cwd), ""
}
func projectForRoots(cwd string, roots []string) (string, string) {
	for _, root := range roots {
		rel, e := filepath.Rel(root, cwd)
		if e == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return Project(cwd, root)
		}
	}
	if len(roots) > 0 && filepath.IsAbs(cwd) {
		return "unmatched", filepath.Base(filepath.Clean(cwd))
	}
	return Project(cwd, "")
}

func outsideProject(cwd string) string {
	project, _ := Project(cwd, "")
	return project
}

// NormalizeProvenance records the feed that supplied each session field.
func NormalizeProvenance(s *model.AgentSession) {
	if len(s.Sources) == 0 && s.Source != "" {
		s.Sources = []string{s.Source}
	}
	if s.Provenance.State == "" {
		s.Provenance.State = s.Source
	}
	if s.Provenance.Model == "" && s.Model != "" {
		s.Provenance.Model = s.Source
	}
	if s.Provenance.Project == "" && (s.Project != "" || s.Worktree != "") {
		s.Provenance.Project = s.Source
	}
	if s.PID != 0 {
		s.Provenance.Resources = "process"
		if !containsSource(s.Sources, "process") {
			s.Sources = append(s.Sources, "process")
		}
	}
	if s.Provenance.Tokens != "" {
		s.Provenance.Cost = "estimated"
	}
}

func containsSource(sources []string, source string) bool {
	for _, value := range sources {
		if value == source {
			return true
		}
	}
	return false
}

// MergeSessions keeps all observed feeds and the origin of the chosen fields.
func MergeSessions(sessions []model.AgentSession) []model.AgentSession {
	out := make([]model.AgentSession, 0, len(sessions))
	byID := map[string]int{}
	for _, s := range sessions {
		NormalizeProvenance(&s)
		idx, ok := byID[s.ID]
		if !ok {
			byID[s.ID] = len(out)
			out = append(out, s)
			continue
		}
		old := out[idx]
		preferred, other := old, s
		if s.Source == "otlp" && old.Source != "otlp" || (s.Source == old.Source && s.At.After(old.At)) {
			preferred, other = s, old
		}
		for _, source := range other.Sources {
			if !containsSource(preferred.Sources, source) {
				preferred.Sources = append(preferred.Sources, source)
			}
		}
		if preferred.Project == "" || preferred.Project == "unknown" {
			preferred.Project = other.Project
			preferred.Provenance.Project = other.Provenance.Project
		}
		if preferred.Worktree == "" {
			preferred.Worktree = other.Worktree
			preferred.Provenance.Project = other.Provenance.Project
		}
		if preferred.Model == "" {
			preferred.Model = other.Model
			preferred.Provenance.Model = other.Provenance.Model
		}
		if other.LastToolAt.After(preferred.LastToolAt) {
			preferred.LastToolAt = other.LastToolAt
			preferred.LastTool = other.LastTool
			preferred.Provenance.Tool = other.Provenance.Tool
		}
		if other.LastFileEditAt.After(preferred.LastFileEditAt) {
			preferred.LastFileEditAt = other.LastFileEditAt
			preferred.Provenance.FileEdit = other.Provenance.FileEdit
		}
		if other.At.After(preferred.At) {
			preferred.At = other.At
			preferred.State = other.State
			preferred.Reason = other.Reason
			preferred.ReasonCode = other.ReasonCode
			preferred.LastTurnEnded = other.LastTurnEnded
			preferred.Provenance.State = other.Provenance.State
		}
		if preferred.StartedAt.IsZero() || !other.StartedAt.IsZero() && other.StartedAt.Before(preferred.StartedAt) {
			preferred.StartedAt = other.StartedAt
		}
		if preferred.Provenance.Resources == "" {
			preferred.Provenance.Resources = other.Provenance.Resources
		}
		out[idx] = preferred
	}
	return out
}
func Enrich(sessions []model.AgentSession, processes []Process, roots []string, procRoot string, now time.Time) {
	boot := bootTime(procRoot, now)
	// A process can belong to only one session. Prefer the most recent eligible
	// session when several logs came from the same working directory.
	claimed := map[int]bool{}
	order := make([]int, len(sessions))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool { return sessions[order[i]].At.After(sessions[order[j]].At) })
	for _, i := range order {
		match := -1
		for j, p := range processes {
			if claimed[p.PID] {
				continue
			}
			project, worktree := projectForRoots(p.CWD, roots)
			if sessions[i].Project != "" && sessions[i].Project != "unknown" && sessions[i].Project != "unmatched" && sessions[i].Project != project && (project != "unmatched" || sessions[i].Project != outsideProject(p.CWD)) {
				continue
			}
			if sessions[i].Kind == "claude" && !strings.HasPrefix(p.Comm, "claude") || sessions[i].Kind == "codex" && !strings.HasPrefix(p.Comm, "codex") {
				continue
			}
			// Linux exposes process start time in USER_HZ ticks (100 per second).
			if !boot.IsZero() && p.StartTime > 0 && !sessions[i].At.IsZero() && boot.Add(time.Duration(p.StartTime)*time.Second/100).After(sessions[i].At.Add(time.Minute)) {
				continue
			}
			if match != -1 {
				match = -2
				break
			}
			match = j
			_ = worktree
		}
		if match < 0 {
			if sessions[i].Project == "" {
				sessions[i].Project = "unmatched"
			}
			continue
		}
		p := processes[match]
		claimed[p.PID] = true
		sessions[i].PID = p.PID
		u, e := user.LookupId(strconv.Itoa(int(p.UID)))
		if e == nil {
			sessions[i].User = u.Username
		}
		sessions[i].Project, sessions[i].Worktree = projectForRoots(p.CWD, roots)
		sessions[i].Provenance.Project = "process"
		sessions[i].Provenance.Resources = "process"
		if !containsSource(sessions[i].Sources, "process") {
			sessions[i].Sources = append(sessions[i].Sources, "process")
		}
	}
}

func bootTime(procRoot string, now time.Time) time.Time {
	if b, e := os.ReadFile(filepath.Join(procRoot, "uptime")); e == nil {
		var seconds float64
		if _, e := fmt.Sscan(string(b), &seconds); e == nil && seconds > 0 {
			return now.Add(-time.Duration(seconds * float64(time.Second)))
		}
	}
	return time.Time{}
}

// ProcessSessions exposes agent processes for which no log or OTLP session matched.
func ProcessSessions(sessions []model.AgentSession, processes []Process, roots []string, procRoot string, now time.Time) []model.AgentSession {
	claimed := map[int]bool{}
	for _, s := range sessions {
		if s.PID != 0 {
			claimed[s.PID] = true
		}
	}
	boot := bootTime(procRoot, now)
	for _, p := range processes {
		if claimed[p.PID] {
			continue
		}
		kind := ""
		if strings.HasPrefix(p.Comm, "claude") {
			kind = "claude"
		} else if strings.HasPrefix(p.Comm, "codex") {
			kind = "codex"
		}
		if kind == "" {
			continue
		}
		project, worktree := projectForRoots(p.CWD, roots)
		at := time.Time{}
		if !boot.IsZero() && p.StartTime > 0 {
			at = boot.Add(time.Duration(p.StartTime) * time.Second / 100).UTC()
		}
		u, e := user.LookupId(strconv.Itoa(int(p.UID)))
		username := ""
		if e == nil {
			username = u.Username
		}
		sessions = append(sessions, model.AgentSession{ID: Hash(fmt.Sprintf("process:%d:%d:%d", p.PID, p.UID, p.StartTime)), Kind: kind, Project: project, Worktree: worktree, User: username, PID: p.PID, At: at, StartedAt: at, State: "running", Reason: "process alive; session activity unavailable", Source: "process"})
		NormalizeProvenance(&sessions[len(sessions)-1])
		claimed[p.PID] = true
	}
	return sessions
}

type procStat struct {
	ppid  int
	ticks uint64
	rss   int64
}
type Tracker struct {
	previous    map[int]uint64
	accumulated map[int]float64
	peak        map[int]int64
	at          time.Time
}

func (t *Tracker) Measure(root string, roots []Process, now time.Time) map[int]struct {
	CPU          float64
	RSS          int64
	CPUSeconds   float64
	PeakRSSBytes int64
} {
	entries, e := os.ReadDir(root)
	if e != nil {
		return nil
	}
	stats := map[int]procStat{}
	for _, entry := range entries {
		pid, e := strconv.Atoi(entry.Name())
		if e != nil || pid <= 0 {
			continue
		}
		b, e := os.ReadFile(filepath.Join(root, entry.Name(), "stat"))
		if e != nil || len(b) > 4096 {
			continue
		}
		end := strings.LastIndexByte(string(b), ')')
		if end < 0 {
			continue
		}
		f := strings.Fields(string(b[end+1:]))
		if len(f) < 22 {
			continue
		}
		ppid, e := strconv.Atoi(f[1])
		if e != nil {
			continue
		}
		ut, _ := strconv.ParseUint(f[11], 10, 64)
		st, _ := strconv.ParseUint(f[12], 10, 64)
		rssPages, _ := strconv.ParseInt(f[21], 10, 64)
		stats[pid] = procStat{ppid: ppid, ticks: ut + st, rss: rssPages * int64(os.Getpagesize())}
	}
	results := map[int]struct {
		CPU          float64
		RSS          int64
		CPUSeconds   float64
		PeakRSSBytes int64
	}{}
	if t.accumulated == nil {
		t.accumulated = map[int]float64{}
		t.peak = map[int]int64{}
	}
	current := map[int]uint64{}
	elapsed := now.Sub(t.at).Seconds()
	for _, rootProcess := range roots {
		var ticks uint64
		var rss int64
		for pid, stat := range stats {
			parent := pid
			for depth := 0; depth < 256; depth++ {
				if parent == rootProcess.PID {
					ticks += stat.ticks
					rss += stat.rss
					break
				}
				v, ok := stats[parent]
				if !ok || v.ppid == parent {
					break
				}
				parent = v.ppid
			}
		}
		current[rootProcess.PID] = ticks
		cpu := 0.0
		if old, ok := t.previous[rootProcess.PID]; ok && ticks >= old && elapsed > 0 {
			cpu = float64(ticks-old) / 100 / elapsed * 100
			t.accumulated[rootProcess.PID] += float64(ticks-old) / 100
		}
		if rss > t.peak[rootProcess.PID] {
			t.peak[rootProcess.PID] = rss
		}
		results[rootProcess.PID] = struct {
			CPU          float64
			RSS          int64
			CPUSeconds   float64
			PeakRSSBytes int64
		}{cpu, rss, t.accumulated[rootProcess.PID], t.peak[rootProcess.PID]}
	}
	t.previous = current
	for pid := range t.accumulated {
		if _, ok := current[pid]; !ok {
			delete(t.accumulated, pid)
			delete(t.peak, pid)
		}
	}
	t.at = now
	return results
}
