package projects

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

const idleAfter = 15 * time.Minute
const busyCPU = 1.0

type activity struct {
	cpu                                       float64
	at, firstSeen, lastActive, lastConnection time.Time
	connections                               map[string]bool
	start                                     string
}
type Collector struct{ prior map[int]activity }

func within(path, root string) bool {
	if root == string(filepath.Separator) {
		return filepath.IsAbs(path)
	}
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}
func assignment(cwd string, projects, worktrees []string) (string, string) {
	// Linux appends this marker to /proc/<pid>/cwd after Orca has removed a
	// worktree. Keep its original identity instead of exposing the trash path.
	deleted := strings.HasSuffix(cwd, " (deleted)")
	cwd = strings.TrimSuffix(cwd, " (deleted)")
	if at := strings.Index(cwd, "/.orca-worktree-trash/"); at >= 0 {
		root := cwd[:at]
		allowed := false
		for _, configured := range worktrees {
			allowed = allowed || within(root, configured)
		}
		if allowed {
			parts := strings.Split(strings.TrimPrefix(cwd[at:], "/.orca-worktree-trash/"), "/")
			if len(parts) > 0 && parts[0] != "" {
				return filepath.Base(root), "deleted worktree (" + parts[0] + ")"
			}
			return filepath.Base(root), "deleted worktree"
		}
	}
	best, work := "", false
	for _, root := range projects {
		if within(cwd, root) && len(root) > len(best) {
			best = root
			work = false
		}
	}
	for _, root := range worktrees {
		if within(cwd, root) && len(root) > len(best) {
			best = root
			work = true
		}
	}
	if best == "" {
		return "", ""
	}
	parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(cwd, best), "/"), "/")
	if deleted && work {
		name := "deleted worktree"
		if len(parts) >= 2 && parts[1] != "" {
			name += " (" + parts[1] + ")"
		}
		return parts[0], name
	}
	if work {
		if len(parts) >= 2 && parts[0] != "" {
			return parts[0], parts[1]
		}
		return filepath.Base(best), parts[0]
	}
	if len(parts) > 0 && parts[0] != "" {
		return parts[0], ""
	}
	return filepath.Base(best), ""
}
func sockets(procRoot string) (map[string]int, bool) {
	out := map[string]int{}
	measured := false
	for _, name := range []string{"tcp", "tcp6"} {
		b, e := os.ReadFile(filepath.Join(procRoot, "net", name))
		if e != nil {
			continue
		}
		measured = true
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) < 10 || f[3] != "0A" {
				continue
			}
			pair := strings.Split(f[1], ":")
			if len(pair) != 2 {
				continue
			}
			p, e := strconv.ParseInt(pair[1], 16, 32)
			if e == nil && p > 0 {
				out[f[9]] = int(p)
			}
		}
	}
	return out, measured
}
func stat(procRoot string, pid int) (string, string, float64, float64, bool) {
	b, e := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "stat"))
	if e != nil {
		return "", "", 0, 0, false
	}
	raw := string(b)
	end := strings.LastIndexByte(raw, ')')
	start := strings.IndexByte(raw, '(')
	if start < 0 || end < 0 {
		return "", "", 0, 0, false
	}
	f := strings.Fields(raw[end+1:])
	if len(f) < 22 {
		return "", "", 0, 0, false
	}
	u, e1 := strconv.ParseFloat(f[11], 64)
	s, e2 := strconv.ParseFloat(f[12], 64)
	rss, e3 := strconv.ParseFloat(f[21], 64)
	if e1 != nil || e2 != nil || e3 != nil {
		return "", "", 0, 0, false
	}
	return raw[start+1 : end], f[19], (u + s) / 100, rss * float64(os.Getpagesize()), true
}
func (c *Collector) Measure(procRoot string, projectRoots, worktreeRoots []string, sessions []model.AgentSession, now time.Time) []model.ProjectProcess {
	if len(projectRoots)+len(worktreeRoots) == 0 {
		return nil
	}
	if c.prior == nil {
		c.prior = map[int]activity{}
	}
	listening, listenMeasured := sockets(procRoot)
	entries, e := os.ReadDir(procRoot)
	if e != nil {
		return nil
	}
	agentPIDs := map[int]bool{}
	for _, s := range sessions {
		if s.PID > 0 && s.State != "ended" {
			agentPIDs[s.PID] = true
		}
	}
	seen := map[int]bool{}
	out := []model.ProjectProcess{}
	for _, entry := range entries {
		pid, e := strconv.Atoi(entry.Name())
		if e != nil || pid <= 0 {
			continue
		}
		base := filepath.Join(procRoot, entry.Name())
		cwd, e := os.Readlink(filepath.Join(base, "cwd"))
		if e != nil {
			continue
		}
		project, worktree := assignment(cwd, projectRoots, worktreeRoots)
		if project == "" {
			continue
		}
		name, start, cpu, memory, ok := stat(procRoot, pid)
		if !ok {
			continue
		}
		fds, fdErr := os.ReadDir(filepath.Join(base, "fd"))
		ports := []int{}
		connections := map[string]bool{}
		portSet := map[int]bool{}
		owned := map[string]bool{}
		for _, fd := range fds {
			target, e := os.Readlink(filepath.Join(base, "fd", fd.Name()))
			if e != nil || !strings.HasPrefix(target, "socket:[") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")
			owned[inode] = true
			if port := listening[inode]; port > 0 && !portSet[port] {
				ports = append(ports, port)
				portSet[port] = true
			}
		}
		sort.Ints(ports)
		// The current connection set is read from the per-process TCP table. A new
		// established inode resets idle time; listening sockets do not.
		tableMeasured := false
		for _, table := range []string{"tcp", "tcp6"} {
			b, e := os.ReadFile(filepath.Join(base, "net", table))
			if e != nil {
				continue
			}
			tableMeasured = true
			for _, line := range strings.Split(string(b), "\n") {
				f := strings.Fields(line)
				if len(f) >= 10 && f[3] == "01" && owned[f[9]] {
					connections[f[9]] = true
				}
			}
		}
		previous, has := c.prior[pid]
		if previous.start != start {
			has = false
		}
		delta := now.Sub(previous.at).Seconds()
		rate := float64(0)
		if has && delta > 0 && delta < 3600 && cpu >= previous.cpu {
			rate = (cpu - previous.cpu) / delta * 100
		}
		active := !has || rate > busyCPU
		newConnection := !has
		if has {
			for inode := range connections {
				if !previous.connections[inode] {
					active = true
					newConnection = true
					break
				}
			}
		}
		last := previous.lastActive
		if active || last.IsZero() {
			last = now
		}
		lastConnection := previous.lastConnection
		if newConnection || lastConnection.IsZero() {
			lastConnection = now
		}
		firstSeen := previous.firstSeen
		if !has || firstSeen.IsZero() {
			firstSeen = now
		}
		c.prior[pid] = activity{cpu: cpu, at: now, firstSeen: firstSeen, lastActive: last, lastConnection: lastConnection, connections: connections, start: start}
		idleFor := now.Sub(last)
		kind := "process"
		if len(ports) > 0 {
			kind = "dev_server"
		}
		if agentPIDs[pid] {
			kind = "agent"
		}
		if kind == "process" && now.Sub(firstSeen) < 5*time.Minute {
			seen[pid] = true
			continue
		}
		coverage := fdErr == nil && tableMeasured
		out = append(out, model.ProjectProcess{PID: pid, Name: name, Project: project, Worktree: worktree, Kind: kind, CPUPercent: rate, MemoryBytes: memory, Ports: ports, PortsMeasured: fdErr == nil && listenMeasured, ConnectionsMeasured: coverage, IdleMinutes: idleFor.Minutes(), ConnectionIdleMinutes: now.Sub(lastConnection).Minutes(), Idle: has && coverage && idleFor >= idleAfter, StopCommand: fmt.Sprintf("kill -TERM -- %d", pid)})
		seen[pid] = true
	}
	for pid := range c.prior {
		if !seen[pid] {
			delete(c.prior, pid)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Project == out[j].Project {
			return out[i].PID < out[j].PID
		}
		return out[i].Project < out[j].Project
	})
	return out
}
