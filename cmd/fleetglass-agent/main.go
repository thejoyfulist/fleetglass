package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/ai"
	"github.com/thejoyfulist/fleetglass/internal/apps"
	"github.com/thejoyfulist/fleetglass/internal/config"
	"github.com/thejoyfulist/fleetglass/internal/model"
	"github.com/thejoyfulist/fleetglass/internal/probe"
	projectprobe "github.com/thejoyfulist/fleetglass/internal/projects"
	"github.com/thejoyfulist/fleetglass/internal/rapl"
	"github.com/thejoyfulist/fleetglass/internal/version"
)

type agent struct {
	c                config.Config
	once             bool
	machine          *probe.Machine
	client           *http.Client
	mu               sync.Mutex
	receiver         *ai.Receiver
	logs             *ai.LogReader
	tracker          ai.Tracker
	lineage          ai.LineageTracker
	appCollector     apps.Collector
	projectCollector projectprobe.Collector
	storageAt        time.Time
	storagePending   chan []model.StorageEntry
	worktreeAt       time.Time
	worktreePending  chan []model.WorktreeEntry
	staleAt          time.Time
	stale            []model.AgentSession
	lastLogScan      time.Time
	fileHashes       map[string]string
	dockerRestarts   map[string]float64
	dockerEvents     []restartEvent
	pendingChanges   []model.Change
	logSessions      []model.AgentSession
	logQuotas        []model.Quota
	probeNext        map[string]time.Time
	probeFailures    map[string]int
	pendingProbes    []model.ProbeSample
	retryAt          time.Time
	retryDelay       time.Duration
}

func (a *agent) probeState(name string, state model.State) model.State {
	if state == model.OK {
		a.probeFailures[name] = 0
		return model.OK
	}
	if state == model.Unknown {
		return model.Unknown
	}
	a.probeFailures[name]++
	if a.probeFailures[name] < 2 {
		return model.Unknown
	}
	return model.Crit
}

// checkState retains a warning on the first failed check and reports a
// confirmed outage after a second consecutive failure, including read errors.
func (a *agent) checkState(name string, state model.State) model.State {
	if state == model.OK {
		a.probeFailures[name] = 0
		return state
	}
	a.probeFailures[name]++
	if a.probeFailures[name] < 2 {
		return model.Warn
	}
	return model.Crit
}

func (a *agent) isolated(name string, fn func(context.Context) []model.Result) []model.Result {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	ch := make(chan []model.Result, 1)
	go func() {
		defer func() {
			if recover() != nil {
				ch <- []model.Result{{Name: name, State: model.Unknown, Detail: "probe panic"}}
			}
		}()
		ch <- fn(ctx)
	}()
	select {
	case v := <-ch:
		return v
	case <-ctx.Done():
		return []model.Result{{Name: name, State: model.Unknown, Detail: "probe timeout"}}
	}
}
func (a *agent) collect(slow bool) []model.Result {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := []model.Result{}
	add := func(name string, fn func(context.Context) []model.Result) {
		cadence := 15
		if name == "ports" || name == "tls" || name == "dns" || name == "helper" || name == "journal" || name == "updates" || name == "sensitive" || name == "managed" {
			cadence = 300
		}
		for _, result := range a.isolated(name, fn) {
			result.CadenceSeconds = cadence
			if (strings.HasPrefix(result.Name, "backup:") || strings.HasPrefix(result.Name, "status:")) && result.State != model.Warn {
				result.State = a.checkState(result.Name, result.State)
			} else if result.State == model.Warn {
				a.probeFailures[result.Name] = 0
			}
			out = append(out, result)
		}
	}
	add("machine", a.machine.Fast)
	add("disk", func(context.Context) []model.Result { return a.machine.Disks() })
	add("drive", func(context.Context) []model.Result { return a.machine.DriveIO() })
	add("network", func(context.Context) []model.Result { return a.machine.Network() })
	add("gpu", func(context.Context) []model.Result { return a.machine.GPU() })
	add("sensors", func(context.Context) []model.Result { return a.machine.Sensors() })
	add("power", func(ctx context.Context) []model.Result { return []model.Result{a.power(ctx)} })
	add("systemd", (probe.Services{Units: a.c.SystemdUnits}).Check)
	add("docker", (probe.Docker{Sockets: a.c.DockerSockets, HelperSocket: a.c.HelperSocket}).Check)
	now := time.Now()
	for i, p := range a.c.HTTPProbes {
		if _, ok := a.probeNext[p.Name]; !ok && i > 0 {
			a.probeNext[p.Name] = now.Add(time.Duration(i*15) * time.Second)
		}
		if now.Before(a.probeNext[p.Name]) {
			continue
		}
		a.probeNext[p.Name] = now.Add(time.Duration(p.IntervalSeconds) * time.Second)
		sample := probe.HTTPCheck(context.Background(), p, now.UTC())
		sample.State = a.probeState(p.Name, sample.State)
		a.pendingProbes = append(a.pendingProbes, sample)
	}
	if slow {
		if len(a.c.DNSProbeNames) > 0 {
			add("dns", func(ctx context.Context) []model.Result { return probe.DNS(ctx, a.c.DNSProbeNames, nil) })
		}
		add("ports", func(context.Context) []model.Result { return probe.Ports(a.c.ProcRoot) })
		add("tls", func(ctx context.Context) []model.Result { return probe.TLS(ctx, a.c.TLSDomains) })
		add("helper", a.helper)
		add("journal", func(ctx context.Context) []model.Result { return probe.Journal(ctx, nil) })
		add("updates", func(ctx context.Context) []model.Result {
			return []model.Result{probe.Updates(ctx, "/var/lib/apt/lists", "/var/lib/dpkg/status", "/var/run/reboot-required")}
		})
		add("sensitive", a.sensitive)
		add("managed", a.managed)
		for _, p := range a.c.InternalProbes {
			out = append(out, a.collectInternal(p, now))
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		changes, states := a.changes(ctx)
		a.pendingChanges = append(a.pendingChanges, changes...)
		out = append(out, states...)
		cancel()
	}
	return a.withDockerRestarts(out)
}

func (a *agent) collectInternal(p config.InternalProbe, now time.Time) model.Result {
	key := "internal:" + p.Name
	checked := a.isolated(key, func(ctx context.Context) []model.Result {
		return []model.Result{probe.Internal(ctx, p)}
	})
	r := checked[0]
	r.CadenceSeconds = 300
	// The history and Overview must report the same confirmed outage.
	r.State = a.checkState(key, r.State)
	name := p.DisplayName
	if name == "" {
		name = p.Name
	}
	historyState := model.Unknown
	if r.State == model.OK || r.State == model.Crit {
		historyState = r.State
	}
	a.pendingProbes = append(a.pendingProbes, model.ProbeSample{Name: key, DisplayName: name, Public: p.Public, Kind: p.Kind, State: historyState, IntervalSeconds: 300, At: now.UTC(), LatencyMS: r.Values["latency_ms"], StatusCode: int(r.Values["status_code"])})
	return r
}
func (a *agent) helper(ctx context.Context) []model.Result {
	transport := &http.Transport{DialContext: func(c context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(c, "unix", a.c.HelperSocket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	out := []model.Result{}
	for i := range a.c.SMARTDevices {
		req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://helper/smart/%d", i), nil)
		resp, e := client.Do(req)
		if e != nil {
			out = append(out, model.Result{Name: fmt.Sprintf("smart:%d", i), State: model.Unknown, Detail: "helper unavailable"})
			continue
		}
		var data struct {
			Healthy     *bool    `json:"healthy"`
			Wear        *float64 `json:"wear_percent"`
			Temperature *float64 `json:"temperature_celsius"`
			Hours       *float64 `json:"power_on_hours"`
			Errors      *float64 `json:"media_errors"`
			Shutdowns   *float64 `json:"unsafe_shutdowns"`
		}
		if resp.StatusCode != 200 {
			out = append(out, model.Result{Name: fmt.Sprintf("smart:%d", i), State: model.Unknown, Detail: "SMART unavailable"})
			resp.Body.Close()
			continue
		}
		e = json.NewDecoder(resp.Body).Decode(&data)
		resp.Body.Close()
		if e != nil {
			out = append(out, model.Result{Name: fmt.Sprintf("smart:%d", i), State: model.Unknown, Detail: "invalid SMART response"})
			continue
		}
		state := model.OK
		if data.Healthy == nil {
			state = model.Unknown
		} else if !*data.Healthy {
			state = model.Crit
		}
		vals := map[string]float64{}
		for _, x := range []struct {
			name string
			p    *float64
		}{{"wear_percent", data.Wear}, {"temperature_celsius", data.Temperature}, {"power_on_hours", data.Hours}, {"media_errors", data.Errors}, {"unsafe_shutdowns", data.Shutdowns}} {
			if x.p != nil {
				vals[x.name] = *x.p
			}
		}
		out = append(out, model.Result{Name: fmt.Sprintf("smart:%d", i), State: state, Detail: "disk health", Values: vals})
	}
	return out
}

func (a *agent) power(ctx context.Context) model.Result {
	sample, err := rapl.Read(a.c.SysRoot)
	if err == nil {
		return a.machine.PowerSample(sample)
	}
	if a.c.HelperSocket == "" {
		return model.Result{Name: "cpu_package_power", State: model.Unknown, Detail: "RAPL unavailable"}
	}
	transport := &http.Transport{DialContext: func(c context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(c, "unix", a.c.HelperSocket)
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://helper/rapl", nil)
	resp, err := client.Do(req)
	if err != nil {
		return model.Result{Name: "cpu_package_power", State: model.Unknown, Detail: "RAPL helper unavailable"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return model.Result{Name: "cpu_package_power", State: model.Unknown, Detail: "RAPL helper unavailable"}
	}
	if err := json.NewDecoder(http.MaxBytesReader(nil, resp.Body, 1<<20)).Decode(&sample); err != nil {
		return model.Result{Name: "cpu_package_power", State: model.Unknown, Detail: "invalid RAPL helper response"}
	}
	return a.machine.PowerSample(sample)
}
func (a *agent) send(results []model.Result) {
	now := time.Now().UTC()
	data := a.receiver.Drain()
	if a.logs != nil {
		if a.lastLogScan.IsZero() || now.Sub(a.lastLogScan) >= 30*time.Second {
			v := a.logs.Scan(now)
			a.lastLogScan = now
			a.logSessions = v.Sessions
			a.logQuotas = v.Quotas
			data.Usage = append(data.Usage, v.Usage...)
		}
		data.Sessions = append(data.Sessions, a.logSessions...)
		data.Quotas = append(data.Quotas, a.logQuotas...)
	}
	logProjects := map[string]string{}
	for _, s := range data.Sessions {
		if s.Source == "log" {
			logProjects[s.ID] = s.Project
		}
	}
	for i := range data.Sessions {
		if data.Sessions[i].Source == "otlp" && data.Sessions[i].Project == "" {
			data.Sessions[i].Project = logProjects[data.Sessions[i].ID]
		}
	}
	data.Sessions = ai.MergeSessions(data.Sessions)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	processes := ai.HelperProcesses(ctx, a.c.HelperSocket)
	cancel()
	ai.HydrateComms(a.c.ProcRoot, processes)
	ai.Enrich(data.Sessions, processes, a.c.WorktreeRoots, a.c.ProcRoot, now)
	data.Sessions = ai.ProcessSessions(data.Sessions, processes, a.c.WorktreeRoots, a.c.ProcRoot, now)
	projects := map[string]string{}
	for _, s := range data.Sessions {
		if s.Project != "" && s.Project != "unmatched" {
			projects[s.ID] = s.Project
		}
	}
	for i := range data.Usage {
		if data.Usage[i].Project == "" {
			data.Usage[i].Project = projects[data.Usage[i].Session]
		}
	}
	usage := a.tracker.Measure(a.c.ProcRoot, processes, now)
	for i := range data.Sessions {
		if v, ok := usage[data.Sessions[i].PID]; ok && data.Sessions[i].PID != 0 {
			data.Sessions[i].CPU = v.CPU
			data.Sessions[i].RSS = v.RSS
			data.Sessions[i].CPUSeconds = v.CPUSeconds
			data.Sessions[i].PeakRSSBytes = v.PeakRSSBytes
			ai.NormalizeProvenance(&data.Sessions[i])
		}
	}
	ai.ApplyHealth(data.Sessions, processes, data.Quotas, now)
	if now.Sub(a.staleAt) >= 5*time.Minute {
		a.stale = ai.StaleWorktrees(a.c.WorktreeRoots, processes, now, 72*time.Hour)
		a.staleAt = now
	}
	data.Sessions = append(data.Sessions, a.stale...)
	powerWatts := float64(0)
	for _, r := range results {
		if r.Name == "cpu_package_power" && r.State == model.OK {
			powerWatts = r.Values["watts"]
		}
	}
	appSamples := a.appCollector.Measure(a.c.ProcRoot, a.c.SysRoot, data.Sessions, results, powerWatts, now)
	projectSamples := a.collectProjects(data.Sessions, now)
	projectSamples = a.lineage.Annotate(a.c.ProcRoot, data.Sessions, projectSamples, now)
	var worktrees []model.WorktreeEntry
	if a.worktreePending != nil {
		select {
		case worktrees = <-a.worktreePending:
			a.worktreePending = nil
		default:
		}
	}
	if !a.once && len(a.c.WorktreeRoots) > 0 && a.worktreePending == nil && (a.worktreeAt.IsZero() || now.Sub(a.worktreeAt) >= time.Hour) {
		a.worktreeAt = now
		a.worktreePending = make(chan []model.WorktreeEntry, 1)
		ch := a.worktreePending
		roots := append([]string(nil), a.c.WorktreeRoots...)
		processSnapshot := append([]model.ProjectProcess(nil), projectSamples...)
		staleSnapshot := append([]model.AgentSession(nil), a.stale...)
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			ch <- projectprobe.ScanWorktrees(ctx, roots, processSnapshot, staleSnapshot)
		}()
	}
	var storage []model.StorageEntry
	if a.storagePending != nil {
		select {
		case storage = <-a.storagePending:
			a.storagePending = nil
		default:
		}
	}
	if !a.once && len(a.c.StorageRoots) > 0 && a.storagePending == nil && (a.storageAt.IsZero() || now.Sub(a.storageAt) >= time.Hour) {
		a.storageAt = now
		a.storagePending = make(chan []model.StorageEntry, 1)
		ch := a.storagePending
		go func() { ch <- a.scanStorage() }()
	}
	a.mu.Lock()
	changes := append([]model.Change(nil), a.pendingChanges...)
	a.mu.Unlock()
	a.mu.Lock()
	probes := append([]model.ProbeSample(nil), a.pendingProbes...)
	a.mu.Unlock()
	probeNames := make([]string, 0, len(a.c.HTTPProbes)+len(a.c.InternalProbes))
	for _, p := range a.c.HTTPProbes {
		probeNames = append(probeNames, p.Name)
	}
	for _, p := range a.c.InternalProbes {
		probeNames = append(probeNames, "internal:"+p.Name)
	}
	b, _ := json.Marshal(model.Batch{HostID: a.c.HostID, Labels: a.c.Labels, At: now, Results: results, AI: &data, Changes: changes, Probes: probes, ProbeNames: &probeNames, Apps: appSamples, Projects: projectSamples, Storage: storage, Worktrees: worktrees})
	queued := a.enqueueAndFlush(b)
	if queued {
		a.mu.Lock()
		if len(changes) <= len(a.pendingChanges) {
			a.pendingChanges = a.pendingChanges[len(changes):]
		}
		if len(probes) <= len(a.pendingProbes) {
			a.pendingProbes = a.pendingProbes[len(probes):]
		}
		a.mu.Unlock()
	}
}
func main() {
	path := flag.String("config", "", "JSON config path")
	once := flag.Bool("once", false, "collect and send once")
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()
	if *showVersion {
		fmt.Println(version.String())
		return
	}
	c, e := config.Load(*path)
	if e != nil {
		log.Fatal(e)
	}
	if missing := config.OptionalKeysAbsent(*path); len(missing) > 0 {
		log.Printf("optional config keys available: %s", strings.Join(missing, ", "))
	}
	if c.Token == "" {
		log.Fatal("ingest token required")
	}
	if c.HostID == "" {
		name, _ := os.Hostname()
		c.HostID = name
	}
	if e := ai.ValidateListen(c.OTLPListen, c.OTLPToken); e != nil {
		log.Fatal(e)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if c.CAFile != "" {
		pem, err := os.ReadFile(c.CAFile)
		if err != nil {
			log.Fatal(err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			log.Fatal(err)
		}
		if !roots.AppendCertsFromPEM(pem) {
			log.Fatal("invalid CA file")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	}
	a := &agent{c: c, once: *once, machine: probe.NewMachine(c.ProcRoot, c.SysRoot), client: &http.Client{Timeout: 10 * time.Second, Transport: transport}, receiver: ai.NewReceiver(), probeNext: map[string]time.Time{}, probeFailures: map[string]int{}}
	if len(c.AILogRoots) > 0 {
		roots := make([]ai.Root, 0, len(c.AILogRoots))
		for _, v := range c.AILogRoots {
			roots = append(roots, ai.Root{Kind: v.Kind, Path: v.Path})
		}
		a.logs = ai.NewLogReader(roots)
		a.logs.WorktreeRoots = append([]string(nil), c.WorktreeRoots...)
	}
	if !*once {
		server := &http.Server{Addr: c.OTLPListen, Handler: a.receiver.Handler(c.OTLPToken), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
		go func() {
			if e := server.ListenAndServe(); e != nil && e != http.ErrServerClosed {
				log.Printf("OTLP listener unavailable")
			}
		}()
	}
	a.send(a.collect(true))
	if *once {
		return
	}
	fast := time.NewTicker(15 * time.Second)
	slow := time.NewTicker(5 * time.Minute)
	defer fast.Stop()
	defer slow.Stop()
	for {
		select {
		case <-fast.C:
			a.send(a.collect(false))
		case <-slow.C:
			a.send(a.collect(true))
		}
	}
}
