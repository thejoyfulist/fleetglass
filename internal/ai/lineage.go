package ai

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type lineageEntry struct {
	start                            uint64
	session, kind, project, worktree string
}
type lineageProcess struct {
	parent       int
	start, ticks uint64
	rss          int64
	name         string
}
type LineageTracker struct {
	known    map[int]lineageEntry
	previous map[int]uint64
	at       time.Time
}

func lineageListening(procRoot string) (map[string]int, bool) {
	out := map[string]int{}
	measured := false
	for _, table := range []string{"tcp", "tcp6"} {
		data, err := os.ReadFile(filepath.Join(procRoot, "net", table))
		if err != nil {
			continue
		}
		measured = true
		for _, line := range strings.Split(string(data), "\n") {
			fields := strings.Fields(line)
			if len(fields) < 10 || fields[3] != "0A" {
				continue
			}
			address := strings.Split(fields[1], ":")
			if len(address) != 2 {
				continue
			}
			port, err := strconv.ParseInt(address[1], 16, 32)
			if err == nil && port > 0 {
				out[fields[9]] = int(port)
			}
		}
	}
	return out, measured
}

func lineagePorts(procRoot string, pid int, listening map[string]int) ([]int, bool) {
	root := filepath.Join(procRoot, strconv.Itoa(pid), "fd")
	files, err := os.ReadDir(root)
	if err != nil {
		return nil, false
	}
	seen := map[int]bool{}
	out := []int{}
	for _, file := range files {
		target, err := os.Readlink(filepath.Join(root, file.Name()))
		if err != nil || !strings.HasPrefix(target, "socket:[") {
			continue
		}
		inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
		if port := listening[inode]; port > 0 && !seen[port] {
			seen[port] = true
			out = append(out, port)
		}
	}
	sort.Ints(out)
	return out, true
}

func readLineageProcess(procRoot string, pid int) (lineageProcess, bool) {
	var out lineageProcess
	b, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "stat"))
	if err != nil || len(b) > 4096 {
		return out, false
	}
	end := strings.LastIndexByte(string(b), ')')
	startName := strings.IndexByte(string(b), '(')
	if end < 0 || startName < 0 || end <= startName {
		return out, false
	}
	f := strings.Fields(string(b[end+1:]))
	if len(f) < 22 {
		return out, false
	}
	out.parent, err = strconv.Atoi(f[1])
	if err != nil {
		return out, false
	}
	out.start, err = strconv.ParseUint(f[19], 10, 64)
	if err != nil {
		return out, false
	}
	user, _ := strconv.ParseUint(f[11], 10, 64)
	system, _ := strconv.ParseUint(f[12], 10, 64)
	out.ticks = user + system
	pages, _ := strconv.ParseInt(f[21], 10, 64)
	if pages > 0 {
		out.rss = pages * int64(os.Getpagesize())
	}
	out.name = string(b[startName+1 : end])
	if out.name == "" {
		out.name = "unknown"
	}
	if len(out.name) > 256 {
		out.name = out.name[:256]
	}
	return out, true
}

// Annotate retains only witnessed ancestry. A descendant reparented after its
// agent exits keeps its session identity until that PID disappears or is reused.
func (t *LineageTracker) Annotate(procRoot string, sessions []model.AgentSession, processes []model.ProjectProcess, now time.Time) []model.ProjectProcess {
	if t.known == nil {
		t.known = map[int]lineageEntry{}
	}
	all := map[int]lineageProcess{}
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return processes
	}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		if process, ok := readLineageProcess(procRoot, pid); ok {
			all[pid] = process
		}
	}
	current := map[int]lineageEntry{}
	active := map[string]bool{}
	rootPIDs := map[int]bool{}
	for _, session := range sessions {
		if session.PID <= 0 || session.State == "ended" {
			continue
		}
		if process, ok := all[session.PID]; ok {
			current[session.PID] = lineageEntry{start: process.start, session: session.ID, kind: session.Kind, project: session.Project, worktree: session.Worktree}
			active[session.ID] = true
			rootPIDs[session.PID] = true
		}
	}
	var resolve func(int, int) (lineageEntry, bool)
	resolve = func(pid, depth int) (lineageEntry, bool) {
		process, ok := all[pid]
		if !ok || depth > 128 {
			return lineageEntry{}, false
		}
		if value, ok := current[pid]; ok && value.start == process.start {
			return value, true
		}
		if value, ok := t.known[pid]; ok && value.start == process.start {
			current[pid] = value
			return value, true
		}
		if process.parent <= 1 || process.parent == pid {
			return lineageEntry{}, false
		}
		parent, ok := resolve(process.parent, depth+1)
		if !ok {
			return lineageEntry{}, false
		}
		value := parent
		value.start = process.start
		current[pid] = value
		return value, true
	}
	boot := bootTime(procRoot, now)
	seen := map[int]bool{}
	for i := range processes {
		process := &processes[i]
		seen[process.PID] = true
		process.AgentSessionID = ""
		process.AgentKind = ""
		process.LeftBehind = false
		process.StartedAt = time.Time{}
		lineage, ok := resolve(process.PID, 0)
		if !ok {
			continue
		}
		process.AgentSessionID = lineage.session
		process.AgentKind = lineage.kind
		process.LeftBehind = !active[lineage.session]
		if !boot.IsZero() {
			process.StartedAt = boot.Add(time.Duration(lineage.start) * time.Second / 100).UTC()
		}
	}
	for pid, process := range all {
		if seen[pid] || rootPIDs[pid] {
			continue
		}
		lineage, ok := resolve(pid, 0)
		if !ok {
			continue
		}
		cpu := float64(0)
		if prior, ok := t.previous[pid]; ok && t.known[pid].start == process.start && process.ticks >= prior && now.After(t.at) {
			cpu = float64(process.ticks-prior) / 100 / now.Sub(t.at).Seconds() * 100
		}
		listening, networkMeasured := lineageListening(filepath.Join(procRoot, strconv.Itoa(pid)))
		ports, fdMeasured := lineagePorts(procRoot, pid, listening)
		measured := fdMeasured && networkMeasured
		kind := "process"
		if len(ports) > 0 {
			kind = "dev_server"
		}
		entry := model.ProjectProcess{PID: pid, Name: process.name, Project: lineage.project, Worktree: lineage.worktree, Kind: kind, CPUPercent: cpu, MemoryBytes: float64(process.rss), Ports: ports, PortsMeasured: measured, ConnectionsMeasured: false, StopCommand: fmt.Sprintf("kill -TERM -- %d", pid), AgentSessionID: lineage.session, AgentKind: lineage.kind, LeftBehind: !active[lineage.session]}
		if !boot.IsZero() {
			entry.StartedAt = boot.Add(time.Duration(lineage.start) * time.Second / 100).UTC()
		}
		processes = append(processes, entry)
	}
	t.known = current
	t.previous = map[int]uint64{}
	for pid, process := range all {
		if _, ok := current[pid]; ok {
			t.previous[pid] = process.ticks
		}
	}
	t.at = now
	return processes
}
