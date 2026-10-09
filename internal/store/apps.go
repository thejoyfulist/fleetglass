package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type AppPoint struct {
	At time.Time `json:"at"`
	model.AppSample
}
type AppTopHour struct {
	At      time.Time `json:"at"`
	CPU     string    `json:"cpu"`
	Memory  string    `json:"memory"`
	Disk    string    `json:"disk"`
	Network string    `json:"network"`
}
type AppsView struct {
	Apps     []model.AppSample `json:"apps"`
	History  []AppPoint        `json:"history"`
	TopHours []AppTopHour      `json:"top_hours"`
	Range    string            `json:"range"`
}

func validApp(a model.AppSample) bool {
	if a.Name == "" || len(a.Name) > 512 || (a.Kind != "agent" && a.Kind != "container" && a.Kind != "unit" && a.Kind != "other") {
		return false
	}
	for _, p := range a.Processes {
		if p.PID <= 0 || p.Name == "" || len(p.Name) > 256 {
			return false
		}
	}
	for _, v := range []float64{a.CPUPercent, a.MemoryBytes, a.PageCacheBytes, a.DiskReadRate, a.DiskWriteRate, a.DiskReadBytes, a.DiskWriteBytes, a.DiskReadCounter, a.DiskWriteCounter} {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
			return false
		}
	}
	if a.CounterSource != "" && a.CounterSource != "proc" && a.CounterSource != "cgroup" {
		return false
	}
	return true
}
func weighted(old, new float64, n int) float64 { return old + (new-old)/float64(n+1) }
func mergeApp(old, new model.AppSample, n int) model.AppSample {
	old.Processes = nil
	old.CPUPercent = weighted(old.CPUPercent, new.CPUPercent, n)
	old.RatesReady = new.RatesReady
	old.MemoryBytes = weighted(old.MemoryBytes, new.MemoryBytes, n)
	old.PageCacheBytes = weighted(old.PageCacheBytes, new.PageCacheBytes, n)
	old.DiskReadRate = weighted(old.DiskReadRate, new.DiskReadRate, n)
	old.DiskWriteRate = weighted(old.DiskWriteRate, new.DiskWriteRate, n)
	old.DiskReadBytes = new.DiskReadBytes
	old.DiskWriteBytes = new.DiskWriteBytes
	old.DiskReadCounter = new.DiskReadCounter
	old.DiskWriteCounter = new.DiskWriteCounter
	old.CounterSource = new.CounterSource
	if new.NetworkRXRate != nil && new.NetworkTXRate != nil {
		if old.NetworkRXRate == nil {
			old.NetworkRXRate = new.NetworkRXRate
			old.NetworkTXRate = new.NetworkTXRate
		} else {
			rx := weighted(*old.NetworkRXRate, *new.NetworkRXRate, n)
			tx := weighted(*old.NetworkTXRate, *new.NetworkTXRate, n)
			old.NetworkRXRate = &rx
			old.NetworkTXRate = &tx
		}
	}
	if new.EnergyWatts != nil {
		if old.EnergyWatts == nil {
			old.EnergyWatts = new.EnergyWatts
		} else {
			v := weighted(*old.EnergyWatts, *new.EnergyWatts, n)
			old.EnergyWatts = &v
		}
	}
	return old
}
func ingestApps(tx *sql.Tx, b model.Batch) error {
	if b.Apps == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, a := range b.Apps {
		key := a.Kind + ":" + a.Name
		if !validApp(a) || seen[key] {
			return errors.New("invalid app sample")
		}
		seen[key] = true
		at := b.At.UnixNano()
		minute := b.At.Truncate(time.Minute).UnixNano()
		hour := b.At.Truncate(time.Hour).UnixNano()
		var previousRaw string
		var previousAt int64
		previousErr := tx.QueryRow(`SELECT at_ns,data FROM app_latest WHERE host_id=? AND kind=? AND name=?`, b.HostID, a.Kind, a.Name).Scan(&previousAt, &previousRaw)
		if previousErr != nil && previousErr != sql.ErrNoRows {
			return previousErr
		}
		// Buffered retries carry the same observation timestamp. Applying one
		// twice would inflate both the hourly mean and daily byte counters.
		if previousErr == nil && at <= previousAt {
			continue
		}
		if a.CounterSource != "" {
			var err error
			if previousErr == nil && b.At.Sub(time.Unix(0, previousAt)) <= time.Hour {
				var previous model.AppSample
				if json.Unmarshal([]byte(previousRaw), &previous) != nil {
					return errors.New("invalid previous app sample")
				}
				if previous.CounterSource == a.CounterSource && a.DiskReadCounter >= previous.DiskReadCounter && a.DiskWriteCounter >= previous.DiskWriteCounter {
					_, err = tx.Exec(`INSERT INTO app_io_daily VALUES(?,?,?,?,?,?) ON CONFLICT(host_id,kind,name,day) DO UPDATE SET read_bytes=read_bytes+excluded.read_bytes,write_bytes=write_bytes+excluded.write_bytes`, b.HostID, a.Name, a.Kind, b.At.UTC().Format("2006-01-02"), a.DiskReadCounter-previous.DiskReadCounter, a.DiskWriteCounter-previous.DiskWriteCounter)
					if err != nil {
						return err
					}
				}
			}
			err = tx.QueryRow(`SELECT read_bytes,write_bytes FROM app_io_daily WHERE host_id=? AND kind=? AND name=? AND day=?`, b.HostID, a.Kind, a.Name, b.At.UTC().Format("2006-01-02")).Scan(&a.DiskReadBytes, &a.DiskWriteBytes)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			if err == sql.ErrNoRows {
				a.DiskReadBytes = 0
				a.DiskWriteBytes = 0
			}
		}
		data, _ := json.Marshal(a)
		if _, err := tx.Exec(`INSERT INTO app_latest VALUES(?,?,?,?,?) ON CONFLICT(host_id,kind,name) DO UPDATE SET at_ns=excluded.at_ns,data=excluded.data WHERE excluded.at_ns>=app_latest.at_ns`, b.HostID, a.Name, a.Kind, at, string(data)); err != nil {
			return err
		}
		a.Processes = nil
		data, _ = json.Marshal(a)
		if _, err := tx.Exec(`INSERT INTO app_minute VALUES(?,?,?,?,?) ON CONFLICT(host_id,kind,name,bucket_ns) DO UPDATE SET data=excluded.data`, b.HostID, a.Name, a.Kind, minute, string(data)); err != nil {
			return err
		}
		var n int
		var raw string
		err := tx.QueryRow(`SELECT n,data FROM app_hourly WHERE host_id=? AND kind=? AND name=? AND bucket_ns=?`, b.HostID, a.Kind, a.Name, hour).Scan(&n, &raw)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		avg := a
		if err == nil {
			var old model.AppSample
			if json.Unmarshal([]byte(raw), &old) != nil {
				return errors.New("invalid stored app sample")
			}
			avg = mergeApp(old, a, n)
		}
		data, _ = json.Marshal(avg)
		if _, err = tx.Exec(`INSERT INTO app_hourly VALUES(?,?,?,?,?,?) ON CONFLICT(host_id,kind,name,bucket_ns) DO UPDATE SET n=excluded.n,data=excluded.data`, b.HostID, a.Name, a.Kind, hour, n+1, string(data)); err != nil {
			return err
		}
	}
	return nil
}

func appRange(name string) (time.Duration, bool, bool) {
	switch name {
	case "1h":
		return time.Hour, false, true
	case "12h":
		return 12 * time.Hour, false, true
	case "24h":
		return 24 * time.Hour, false, true
	case "7d":
		return 7 * 24 * time.Hour, true, true
	case "30d":
		return 30 * 24 * time.Hour, true, true
	default:
		return 0, false, false
	}
}

func (s *Store) Apps(host, rangeName string, now time.Time) (AppsView, error) {
	span, hourly, ok := appRange(rangeName)
	if !ok {
		return AppsView{}, errors.New("invalid app range")
	}
	v := AppsView{Apps: []model.AppSample{}, History: []AppPoint{}, TopHours: []AppTopHour{}, Range: rangeName}
	rows, err := s.DB.Query(`SELECT data FROM app_latest WHERE host_id=? AND at_ns>=? ORDER BY name`, host, now.Add(-time.Minute).UnixNano())
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			break
		}
		var a model.AppSample
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			break
		}
		v.Apps = append(v.Apps, a)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return v, err
	}
	// CPU uses one-core percentages, matching top/htop. Reconcile shares
	// against the measured host totals, including unattributed work.
	o, err := s.Overview(host)
	if err == nil {
		var cpu, mem, diskRead, diskWrite, netRX, netTX float64
		hasCPU, hasMemory, hasDisk, hasNetwork := false, false, false, false
		for _, r := range o.Results {
			if r.Name == "cpu" {
				cores := r.Values["core_count"]
				if cores < 1 {
					cores = 1
				}
				cpu = r.Values["busy_percent"] * cores
				hasCPU = r.State == model.OK || r.State == model.Warn || r.State == model.Crit
			}
			if r.Name == "memory" {
				mem = r.Values["used_bytes"]
				hasMemory = r.State == model.OK || r.State == model.Warn || r.State == model.Crit
			}
			if len(r.Name) > 6 && r.Name[:6] == "drive:" && r.State == model.OK {
				diskRead += r.Values["read_bytes_per_second"]
				diskWrite += r.Values["write_bytes_per_second"]
				hasDisk = true
			}
			if len(r.Name) > 8 && r.Name[:8] == "network:" && r.State == model.OK && r.Name != "network:lo" {
				netRX += r.Values["rx_bytes_per_second"]
				netTX += r.Values["tx_bytes_per_second"]
				hasNetwork = true
			}
		}
		otherIndex := -1
		for i, a := range v.Apps {
			if a.Kind == "other" {
				otherIndex = i
			}
		}
		if otherIndex < 0 {
			v.Apps = append(v.Apps, model.AppSample{Name: "Other / not attributed", Kind: "other", RatesReady: true})
			otherIndex = len(v.Apps) - 1
		}
		reconcile := func(total float64, enabled bool, read func(model.AppSample) float64, write func(*model.AppSample, float64)) {
			if !enabled {
				return
			}
			sum := float64(0)
			for _, a := range v.Apps {
				sum += read(a)
			}
			factor := float64(1)
			if sum > total && sum > 0 {
				factor = total / sum
			}
			for i := range v.Apps {
				write(&v.Apps[i], read(v.Apps[i])*factor)
			}
			write(&v.Apps[otherIndex], read(v.Apps[otherIndex])+math.Max(0, total-sum*factor))
		}
		reconcile(cpu, hasCPU, func(a model.AppSample) float64 { return a.CPUPercent }, func(a *model.AppSample, x float64) { a.CPUPercent = x })
		reconcile(mem, hasMemory, func(a model.AppSample) float64 { return a.MemoryBytes }, func(a *model.AppSample, x float64) { a.MemoryBytes = x })
		reconcile(diskRead, hasDisk, func(a model.AppSample) float64 { return a.DiskReadRate }, func(a *model.AppSample, x float64) { a.DiskReadRate = x })
		reconcile(diskWrite, hasDisk, func(a model.AppSample) float64 { return a.DiskWriteRate }, func(a *model.AppSample, x float64) { a.DiskWriteRate = x })
		reconcile(netRX, hasNetwork, func(a model.AppSample) float64 {
			if a.NetworkRXRate != nil {
				return *a.NetworkRXRate
			}
			return 0
		}, func(a *model.AppSample, x float64) {
			if a.NetworkRXRate != nil || a.Kind == "other" {
				a.NetworkRXRate = &x
			}
		})
		reconcile(netTX, hasNetwork, func(a model.AppSample) float64 {
			if a.NetworkTXRate != nil {
				return *a.NetworkTXRate
			}
			return 0
		}, func(a *model.AppSample, x float64) {
			if a.NetworkTXRate != nil || a.Kind == "other" {
				a.NetworkTXRate = &x
			}
		})
	}
	table := "app_minute"
	unit := int64(time.Minute)
	stride := int64(1)
	if hourly {
		table = "app_hourly"
		unit = int64(time.Hour)
	}
	switch rangeName {
	case "12h":
		stride = 6
	case "24h":
		stride = 12
	case "7d":
		stride = 2
	case "30d":
		stride = 6
	}
	rows, err = s.DB.Query(`SELECT bucket_ns,data FROM `+table+` WHERE host_id=? AND bucket_ns>=? AND (CAST(bucket_ns / ? AS INTEGER) % ?=0 OR bucket_ns>=?) ORDER BY bucket_ns,name`, host, now.Add(-span).UnixNano(), unit, stride, now.Add(-2*time.Duration(unit)).UnixNano())
	if err != nil {
		return v, err
	}
	for rows.Next() {
		var ns int64
		var raw string
		if err = rows.Scan(&ns, &raw); err != nil {
			break
		}
		var a model.AppSample
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			break
		}
		v.History = append(v.History, AppPoint{At: time.Unix(0, ns).UTC(), AppSample: a})
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return v, err
	}
	// Limit browser payload while keeping all apps and the newest point.
	byApp := map[string][]AppPoint{}
	for _, p := range v.History {
		key := p.Kind + ":" + p.Name
		byApp[key] = append(byApp[key], p)
	}
	v.History = v.History[:0]
	for _, points := range byApp {
		step := (len(points) + 119) / 120
		for i := 0; i < len(points); i += step {
			v.History = append(v.History, points[i])
		}
		if len(points) > 0 && (len(v.History) == 0 || !v.History[len(v.History)-1].At.Equal(points[len(points)-1].At)) {
			v.History = append(v.History, points[len(points)-1])
		}
	}
	sort.Slice(v.History, func(i, j int) bool {
		if v.History[i].At.Equal(v.History[j].At) {
			return v.History[i].Name < v.History[j].Name
		}
		return v.History[i].At.Before(v.History[j].At)
	})
	rows, err = s.DB.Query(`SELECT bucket_ns,data FROM app_hourly WHERE host_id=? AND bucket_ns>=? ORDER BY bucket_ns,name`, host, now.Add(-48*time.Hour).UnixNano())
	if err != nil {
		return v, err
	}
	type winner struct {
		name  string
		value float64
	}
	type top struct{ cpu, memory, disk, network winner }
	hours := map[int64]top{}
	for rows.Next() {
		var ns int64
		var raw string
		if err = rows.Scan(&ns, &raw); err != nil {
			break
		}
		var a model.AppSample
		if err = json.Unmarshal([]byte(raw), &a); err != nil {
			break
		}
		t := hours[ns]
		if a.CPUPercent > t.cpu.value {
			t.cpu = winner{a.Name, a.CPUPercent}
		}
		if a.MemoryBytes > t.memory.value {
			t.memory = winner{a.Name, a.MemoryBytes}
		}
		if x := a.DiskReadRate + a.DiskWriteRate; x > t.disk.value {
			t.disk = winner{a.Name, x}
		}
		net := float64(0)
		if a.NetworkRXRate != nil {
			net += *a.NetworkRXRate
		}
		if a.NetworkTXRate != nil {
			net += *a.NetworkTXRate
		}
		if net > t.network.value {
			t.network = winner{a.Name, net}
		}
		hours[ns] = t
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return v, err
	}
	for ns, t := range hours {
		v.TopHours = append(v.TopHours, AppTopHour{At: time.Unix(0, ns).UTC(), CPU: t.cpu.name, Memory: t.memory.name, Disk: t.disk.name, Network: t.network.name})
	}
	sort.Slice(v.TopHours, func(i, j int) bool { return v.TopHours[i].At.Before(v.TopHours[j].At) })
	return v, nil
}
