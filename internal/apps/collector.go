package apps

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type counters struct{ cpu, read, write, rx, tx float64 }
type snapshot struct {
	counters
	at time.Time
}

// Collector reads only process accounting and cgroup files. It never reads
// cmdline, environ or process file descriptors.
type Collector struct {
	previous map[string]snapshot
	today    map[string]counters
	day      string
}

type process struct {
	pid, ppid             int
	name                  string
	path                  string
	cpu, rss, read, write float64
}

func parseProcess(procRoot string, pid int) (process, bool) {
	base := filepath.Join(procRoot, strconv.Itoa(pid))
	stat, err := os.ReadFile(filepath.Join(base, "stat"))
	if err != nil {
		return process{}, false
	}
	end := strings.LastIndexByte(string(stat), ')')
	if end < 0 {
		return process{}, false
	}
	f := strings.Fields(string(stat[end+1:]))
	if len(f) < 22 {
		return process{}, false
	}
	ppid, e1 := strconv.Atoi(f[1])
	u, e2 := strconv.ParseFloat(f[11], 64)
	s, e3 := strconv.ParseFloat(f[12], 64)
	rss, e4 := strconv.ParseFloat(f[21], 64)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return process{}, false
	}
	// Linux /proc exposes process CPU time in USER_HZ (100 ticks/second).
	p := process{pid: pid, ppid: ppid, name: string(stat[strings.IndexByte(string(stat), '(')+1 : end]), cpu: (u + s) * 10000, rss: rss * float64(os.Getpagesize())}
	if b, err := os.ReadFile(filepath.Join(base, "io")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			fields := strings.Fields(line)
			if len(fields) != 2 {
				continue
			}
			v, _ := strconv.ParseFloat(fields[1], 64)
			switch fields[0] {
			case "read_bytes:":
				p.read = v
			case "write_bytes:":
				p.write = v
			}
		}
	}
	if b, err := os.ReadFile(filepath.Join(base, "cgroup")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "0::/") {
				p.path = strings.TrimPrefix(line, "0::")
				break
			}
			if p.path == "" {
				parts := strings.SplitN(line, ":", 3)
				if len(parts) == 3 && strings.HasPrefix(parts[2], "/") {
					p.path = parts[2]
				}
			}
		}
	}
	return p, true
}

func classification(p process, sessions map[int]string, parents map[int]int, containers map[string]string) (string, string) {
	for pid, seen := p.pid, 0; pid > 0 && seen < 64; pid, seen = parents[pid], seen+1 {
		if name := sessions[pid]; name != "" {
			return "agent", name
		}
	}
	for _, segment := range strings.Split(p.path, "/") {
		if strings.HasPrefix(segment, "docker-") && strings.HasSuffix(segment, ".scope") {
			id := strings.TrimSuffix(strings.TrimPrefix(segment, "docker-"), ".scope")
			if len(id) > 12 {
				id = id[:12]
			}
			if name := containers[id]; name != "" {
				return "container", name
			}
			return "container", "Container " + id
		}
		if len(segment) >= 64 && strings.Trim(segment, "0123456789abcdef") == "" {
			if name := containers[segment[:12]]; name != "" {
				return "container", name
			}
			return "container", "Container " + segment[:12]
		}
	}
	segments := strings.Split(p.path, "/")
	for i := len(segments) - 1; i >= 0; i-- {
		segment := segments[i]
		if strings.HasSuffix(segment, ".service") || strings.HasSuffix(segment, ".scope") {
			return "unit", segment
		}
	}
	return "other", "Other / not attributed"
}

func readCgroup(sysRoot, path string) (counters, float64, float64, bool) {
	if path == "" || path == "/" || strings.Contains(path, "..") {
		return counters{}, 0, 0, false
	}
	base := filepath.Join(sysRoot, "fs/cgroup", path)
	b, err := os.ReadFile(filepath.Join(base, "cpu.stat"))
	if err != nil {
		return counters{}, 0, 0, false
	}
	c := counters{}
	foundCPU := false
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == "usage_usec" {
			c.cpu, _ = strconv.ParseFloat(f[1], 64)
			foundCPU = true
		}
	}
	if !foundCPU {
		return counters{}, 0, 0, false
	}
	memory := float64(0)
	b, err = os.ReadFile(filepath.Join(base, "memory.current"))
	if err != nil {
		return counters{}, 0, 0, false
	}
	memory, err = strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	if err != nil {
		return counters{}, 0, 0, false
	}
	cache := float64(0)
	if b, err = os.ReadFile(filepath.Join(base, "memory.stat")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) == 2 && f[0] == "file" {
				cache, _ = strconv.ParseFloat(f[1], 64)
				break
			}
		}
	} else {
		return counters{}, 0, 0, false
	}
	if b, err = os.ReadFile(filepath.Join(base, "io.stat")); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			for _, f := range strings.Fields(line)[min(1, len(strings.Fields(line))):] {
				kv := strings.SplitN(f, "=", 2)
				if len(kv) != 2 {
					continue
				}
				v, _ := strconv.ParseFloat(kv[1], 64)
				switch kv[0] {
				case "rbytes":
					c.read += v
				case "wbytes":
					c.write += v
				}
			}
		}
	} else {
		return counters{}, 0, 0, false
	}
	return c, memory, cache, true
}

func netCounters(procRoot string, pid int) (float64, float64, bool) {
	b, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "net/dev"))
	if err != nil {
		return 0, 0, false
	}
	var rx, tx float64
	for _, line := range strings.Split(string(b), "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "lo" {
			continue
		}
		f := strings.Fields(parts[1])
		if len(f) < 16 {
			continue
		}
		a, e1 := strconv.ParseFloat(f[0], 64)
		z, e2 := strconv.ParseFloat(f[8], 64)
		if e1 == nil && e2 == nil {
			rx += a
			tx += z
		}
	}
	return rx, tx, true
}

// Measure attributes each process once. Cgroup counters replace process sums
// where readable; an unreadable namespace leaves network explicitly unknown.
func (c *Collector) Measure(procRoot, sysRoot string, sessions []model.AgentSession, results []model.Result, powerWatts float64, now time.Time) []model.AppSample {
	if c.previous == nil {
		c.previous = map[string]snapshot{}
	}
	if c.today == nil {
		c.today = map[string]counters{}
	}
	day := now.UTC().Format("2006-01-02")
	if day != c.day {
		c.today = map[string]counters{}
		c.day = day
	}
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil
	}
	processes := []process{}
	parents := map[int]int{}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		p, ok := parseProcess(procRoot, pid)
		if !ok {
			continue
		}
		processes = append(processes, p)
		parents[pid] = p.ppid
	}
	sessionNames := map[int]string{}
	containerNames := map[string]string{}
	for _, r := range results {
		if strings.HasPrefix(r.Name, "container:") {
			if id := r.Fields["container_id"]; len(id) == 12 {
				containerNames[id] = strings.SplitN(r.Name, ":", 3)[2]
			}
		}
	}
	for _, s := range sessions {
		if s.PID <= 0 || s.State == "ended" {
			continue
		}
		name := strings.Title(s.Kind) + " session"
		if s.Project != "" {
			name += " · " + s.Project
		}
		if len(s.ID) >= 8 {
			name += " · " + s.ID[:8]
		}
		sessionNames[s.PID] = name
	}
	type group struct {
		sample model.AppSample
		paths  map[string]bool
		pid    int
		raw    counters
	}
	groups := map[string]*group{}
	pathOwner := map[string]string{}
	for _, p := range processes {
		kind, name := classification(p, sessionNames, parents, containerNames)
		key := kind + ":" + name
		g := groups[key]
		if g == nil {
			g = &group{sample: model.AppSample{Name: name, Kind: kind}, paths: map[string]bool{}}
			groups[key] = g
		}
		g.sample.Processes = append(g.sample.Processes, model.AppProcess{Name: p.name, PID: p.pid})
		g.raw.cpu += p.cpu
		g.raw.read += p.read
		g.raw.write += p.write
		g.sample.MemoryBytes += p.rss
		if p.path != "" {
			g.paths[p.path] = true
			if owner, exists := pathOwner[p.path]; exists && owner != key {
				pathOwner[p.path] = ""
			} else if !exists {
				pathOwner[p.path] = key
			}
		}
		if g.pid == 0 {
			g.pid = p.pid
		}
	}
	out := make([]model.AppSample, 0, len(groups))
	totalCPU := float64(0)
	for key, g := range groups {
		v := g.raw
		sort.Slice(g.sample.Processes, func(i, j int) bool { return g.sample.Processes[i].PID < g.sample.Processes[j].PID })
		g.sample.CounterSource = "proc"
		if g.sample.Kind != "other" {
			var cg counters
			memory, cache := float64(0), float64(0)
			complete := len(g.paths) > 0
			for path := range g.paths {
				if pathOwner[path] != key {
					complete = false
					break
				}
				for other := range pathOwner {
					if other != path && strings.HasPrefix(other, path+"/") {
						complete = false
						break
					}
				}
				if !complete {
					break
				}
				x, m, fileCache, ok := readCgroup(sysRoot, path)
				if !ok {
					complete = false
					break
				}
				cg.cpu += x.cpu
				cg.read += x.read
				cg.write += x.write
				memory += m
				cache += fileCache
			}
			if complete {
				v = cg
				g.sample.CounterSource = "cgroup"
				if memory > 0 {
					g.sample.MemoryBytes = memory
				}
				g.sample.PageCacheBytes = cache
			}
		}
		if g.sample.Kind == "container" {
			hostNS, _ := os.Readlink(filepath.Join(procRoot, "1/ns/net"))
			appNS, _ := os.Readlink(filepath.Join(procRoot, strconv.Itoa(g.pid), "ns/net"))
			if appNS != "" && appNS != hostNS {
				if rx, tx, ok := netCounters(procRoot, g.pid); ok {
					v.rx = rx
					v.tx = tx
				}
			}
		}
		prev, has := c.previous[key]
		dt := now.Sub(prev.at).Seconds()
		if has && dt > 0 && dt < 3600 {
			g.sample.RatesReady = true
			if v.cpu >= prev.cpu {
				g.sample.CPUPercent = (v.cpu - prev.cpu) / dt / 10000
			}
			if v.read >= prev.read {
				g.sample.DiskReadRate = (v.read - prev.read) / dt
				c.today[key] = add(c.today[key], counters{read: v.read - prev.read})
			}
			if v.write >= prev.write {
				g.sample.DiskWriteRate = (v.write - prev.write) / dt
				c.today[key] = add(c.today[key], counters{write: v.write - prev.write})
			}
			if g.sample.Kind == "container" && v.rx >= prev.rx && v.tx >= prev.tx && (v.rx > 0 || v.tx > 0) {
				rx := (v.rx - prev.rx) / dt
				tx := (v.tx - prev.tx) / dt
				g.sample.NetworkRXRate = &rx
				g.sample.NetworkTXRate = &tx
			}
		}
		g.sample.DiskReadBytes = c.today[key].read
		g.sample.DiskWriteBytes = c.today[key].write
		g.sample.DiskReadCounter = v.read
		g.sample.DiskWriteCounter = v.write
		c.previous[key] = snapshot{counters: v, at: now}
		totalCPU += g.sample.CPUPercent
		out = append(out, g.sample)
	}
	if powerWatts > 0 && totalCPU > 0 {
		for i := range out {
			x := powerWatts * out[i].CPUPercent / totalCPU
			out[i].EnergyWatts = &x
		}
	}
	for key, previous := range c.previous {
		if now.Sub(previous.at) > time.Hour {
			delete(c.previous, key)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CPUPercent == out[j].CPUPercent {
			return out[i].Name < out[j].Name
		}
		return out[i].CPUPercent > out[j].CPUPercent
	})
	return out
}

func add(a, b counters) counters { a.read += b.read; a.write += b.write; return a }
