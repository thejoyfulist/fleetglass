package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type Insight struct {
	Kind       string    `json:"kind"`
	App        string    `json:"app"`
	AppKind    string    `json:"app_kind"`
	Detail     string    `json:"detail"`
	DetailCode string    `json:"detail_code"`
	Since      time.Time `json:"since"`
}
type StorageSize struct {
	model.StorageEntry
	Growth7D *int64    `json:"growth_7d_bytes"`
	At       time.Time `json:"at"`
}
type Observations struct {
	Projects []model.ProjectProcess `json:"projects"`
	Storage  []StorageSize          `json:"storage"`
	Insights []Insight              `json:"insights"`
	Heat     []model.Change         `json:"heat"`
}

func ingestObservations(tx *sql.Tx, b model.Batch) error {
	for _, p := range b.Projects {
		if p.PID <= 0 || p.Name == "" || len(p.Name) > 256 || len(p.Project) > 512 || len(p.Worktree) > 512 || p.CPUPercent < 0 || math.IsNaN(p.CPUPercent) || math.IsInf(p.CPUPercent, 0) || p.MemoryBytes < 0 || p.IdleMinutes < 0 || p.ConnectionIdleMinutes < 0 {
			return errors.New("invalid project process")
		}
		for _, port := range p.Ports {
			if port <= 0 || port > 65535 {
				return errors.New("invalid project port")
			}
		}
		raw, _ := json.Marshal(p)
		if _, err := tx.Exec(`INSERT INTO project_latest VALUES(?,?,?,?) ON CONFLICT(host_id,pid) DO UPDATE SET at_ns=excluded.at_ns,data=excluded.data WHERE excluded.at_ns>=project_latest.at_ns`, b.HostID, p.PID, b.At.UnixNano(), string(raw)); err != nil {
			return err
		}
	}
	for _, s := range b.Storage {
		if s.Root == "" || s.Path == "" || !(s.Path == s.Root || s.Root == "/" && strings.HasPrefix(s.Path, "/") || strings.HasPrefix(s.Path, s.Root+"/")) || s.Depth < 0 || s.Depth > 2 || s.SizeBytes < 0 {
			return errors.New("invalid storage entry")
		}
		complete := 0
		if s.Complete {
			complete = 1
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO storage_history VALUES(?,?,?,?,?,?,?)`, b.HostID, s.Root, s.Path, b.At.UnixNano(), s.SizeBytes, complete, s.Depth); err != nil {
			return err
		}
	}
	hot := false
	trigger := ""
	for _, r := range b.Results {
		if r.State != model.OK && r.State != model.Warn && r.State != model.Crit {
			continue
		}
		sensorName := strings.ToLower(r.Name)
		if strings.HasPrefix(sensorName, "sensor:") && (strings.Contains(sensorName, "package") || strings.Contains(sensorName, "tctl") || strings.Contains(sensorName, "tdie")) && r.Values["celsius"] >= 80 {
			hot = true
			trigger = "package temperature"
		}
		if r.Name == "cpu_package_power" && r.Values["watts"] >= 45 {
			hot = true
			if trigger == "" {
				trigger = "CPU package power"
			}
		}
	}
	if hot && len(b.Apps) > 0 {
		apps := append([]model.AppSample(nil), b.Apps...)
		sort.Slice(apps, func(i, j int) bool { return apps[i].CPUPercent > apps[j].CPUPercent })
		names := []string{}
		for _, a := range apps {
			if a.CPUPercent > 0 && a.Kind != "other" {
				names = append(names, a.Name)
				if len(names) == 3 {
					break
				}
			}
		}
		if len(names) > 0 {
			at := b.At.Truncate(15 * time.Minute)
			id := fmt.Sprintf("heat:%s:%d", b.HostID, at.Unix())
			text := trigger + " high; top CPU: " + strings.Join(names, ", ")
			if _, err := tx.Exec(`INSERT OR IGNORE INTO changes VALUES(?,?,?,?,?)`, b.HostID, id, at.UnixNano(), "Heat", text); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *Store) Observations(host string, now time.Time) (Observations, error) {
	out := Observations{Projects: []model.ProjectProcess{}, Storage: []StorageSize{}, Insights: []Insight{}, Heat: []model.Change{}}
	rows, err := s.DB.Query(`SELECT data FROM project_latest WHERE host_id=? AND at_ns>=? ORDER BY pid`, host, now.Add(-time.Minute).UnixNano())
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			break
		}
		var p model.ProjectProcess
		if err = json.Unmarshal([]byte(raw), &p); err != nil {
			break
		}
		out.Projects = append(out.Projects, p)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return out, err
	}
	rows, err = s.DB.Query(`SELECT root,path,at_ns,size_bytes,complete,depth FROM storage_history WHERE host_id=? AND at_ns=(SELECT MAX(at_ns) FROM storage_history WHERE host_id=?) ORDER BY root,size_bytes DESC`, host, host)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var x StorageSize
		var ns int64
		var complete int
		if err = rows.Scan(&x.Root, &x.Path, &ns, &x.SizeBytes, &complete, &x.Depth); err != nil {
			break
		}
		x.At = time.Unix(0, ns).UTC()
		x.Complete = complete == 1
		out.Storage = append(out.Storage, x)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return out, err
	}
	for i := range out.Storage {
		x := &out.Storage[i]
		if !x.Complete {
			continue
		}
		var old int64
		e := s.DB.QueryRow(`SELECT size_bytes FROM storage_history WHERE host_id=? AND path=? AND at_ns<=? AND complete=1 ORDER BY at_ns DESC LIMIT 1`, host, x.Path, now.Add(-7*24*time.Hour).UnixNano()).Scan(&old)
		if e == nil {
			growth := x.SizeBytes - old
			x.Growth7D = &growth
		} else if e != sql.ErrNoRows {
			return out, e
		}
	}
	rows, err = s.DB.Query(`SELECT id,at_ns,kind,text FROM changes WHERE host_id=? AND kind='Heat' AND at_ns>=? ORDER BY at_ns DESC LIMIT 20`, host, now.Add(-7*24*time.Hour).UnixNano())
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var c model.Change
		var ns int64
		if err = rows.Scan(&c.ID, &ns, &c.Type, &c.Text); err != nil {
			break
		}
		c.At = time.Unix(0, ns).UTC()
		model.AnnotateChange(&c)
		out.Heat = append(out.Heat, c)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return out, err
	}
	insights, err := s.insights(host, now, out.Projects)
	if err != nil {
		return out, err
	}
	out.Insights = insights
	return out, nil
}
func (s *Store) insights(host string, now time.Time, projects []model.ProjectProcess) ([]Insight, error) {
	rows, err := s.DB.Query(`SELECT kind,name,bucket_ns,data FROM app_minute WHERE host_id=? AND bucket_ns>=? ORDER BY kind,name,bucket_ns`, host, now.Add(-65*time.Minute).UnixNano())
	if err != nil {
		return nil, err
	}
	type point struct {
		at  time.Time
		app model.AppSample
	}
	groups := map[string][]point{}
	for rows.Next() {
		var kind, name, raw string
		var ns int64
		if err = rows.Scan(&kind, &name, &ns, &raw); err != nil {
			break
		}
		var a model.AppSample
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			break
		}
		groups[kind+":"+name] = append(groups[kind+":"+name], point{time.Unix(0, ns).UTC(), a})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return nil, err
	}
	out := []Insight{}
	add := func(kind string, a model.AppSample, since time.Time, detail string) {
		out = append(out, Insight{Kind: kind, App: a.Name, AppKind: a.Kind, Since: since, Detail: detail, DetailCode: kind})
	}
	for _, points := range groups {
		if len(points) < 2 {
			continue
		}
		a := points[len(points)-1].app
		if now.Sub(points[len(points)-1].at) > 2*time.Minute {
			continue
		}
		for _, rule := range []struct {
			kind      string
			duration  time.Duration
			threshold float64
			measure   func(model.AppSample) float64
			detail    string
		}{
			{"cpu", 10 * time.Minute, 80, func(a model.AppSample) float64 { return a.CPUPercent }, "CPU above 80% of one core for 10 minutes"},
			{"disk", 15 * time.Minute, 10 * 1024 * 1024, func(a model.AppSample) float64 { return a.DiskWriteRate }, "Disk writes above 10 MiB/s for 15 minutes"},
			{"network", 15 * time.Minute, 10 * 1024 * 1024, func(a model.AppSample) float64 {
				if a.NetworkRXRate == nil || a.NetworkTXRate == nil {
					return 0
				}
				return *a.NetworkRXRate + *a.NetworkTXRate
			}, "Network traffic above 10 MiB/s for 15 minutes"},
		} {
			since := points[len(points)-1].at
			for i := len(points) - 1; i >= 0; i-- {
				p := points[i]
				if rule.measure(p.app) < rule.threshold || since.Sub(p.at) > 2*time.Minute {
					break
				}
				since = p.at
			}
			if points[len(points)-1].at.Sub(since) >= rule.duration {
				add(rule.kind, a, since, rule.detail)
			}
		}
		memoryPoints := points
		for len(memoryPoints) > 0 && memoryPoints[0].at.Before(now.Add(-time.Hour)) {
			memoryPoints = memoryPoints[1:]
		}
		if len(memoryPoints) < 30 {
			continue
		}
		first := memoryPoints[0]
		last := memoryPoints[len(memoryPoints)-1]
		if last.at.Sub(first.at) >= 59*time.Minute && last.app.MemoryBytes-first.app.MemoryBytes >= 100*1024*1024 {
			steady := true
			previous := first.app.MemoryBytes
			previousAt := first.at
			for _, p := range memoryPoints[1:] {
				if p.at.Sub(previousAt) > 2*time.Minute || p.app.MemoryBytes < previous*0.95 {
					steady = false
					break
				}
				previous = p.app.MemoryBytes
				previousAt = p.at
			}
			if steady {
				add("memory", a, first.at, "Memory grew steadily by at least 100 MiB in one hour")
			}
		}
	}
	owners := map[int]model.AppSample{}
	appRows, e := s.DB.Query(`SELECT data FROM app_latest WHERE host_id=? AND at_ns>=?`, host, now.Add(-time.Minute).UnixNano())
	if e != nil {
		return nil, e
	}
	for appRows.Next() {
		var raw string
		if e = appRows.Scan(&raw); e != nil {
			break
		}
		var app model.AppSample
		if e = json.Unmarshal([]byte(raw), &app); e != nil {
			break
		}
		for _, process := range app.Processes {
			owners[process.PID] = app
		}
	}
	if e == nil {
		e = appRows.Err()
	}
	appRows.Close()
	if e != nil {
		return nil, e
	}
	for _, p := range projects {
		owner, known := owners[p.PID]
		if !known {
			owner = model.AppSample{Name: p.Name, Kind: "project"}
		}
		if p.Kind == "dev_server" && p.ConnectionsMeasured && p.IdleMinutes >= 180 {
			add("idle_dev_server", owner, now.Add(-time.Duration(p.IdleMinutes)*time.Minute), p.Name+" idle for at least 3 hours")
		}
		if p.Kind == "dev_server" && p.ConnectionsMeasured && p.CPUPercent >= 80 && p.ConnectionIdleMinutes >= 30 {
			add("runaway_dev_server", owner, now.Add(-time.Duration(p.ConnectionIdleMinutes)*time.Minute), p.Name+" above 80% CPU with no new connection for 30 minutes")
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind == out[j].Kind {
			return out[i].App < out[j].App
		}
		return out[i].Kind < out[j].Kind
	})
	return out, nil
}
