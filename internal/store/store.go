package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
	_ "modernc.org/sqlite"
)

type Store struct{ DB *sql.DB }

func Open(path string) (*Store, error) {
	db, e := sql.Open("sqlite", path)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(1)
	for _, q := range []string{
		`PRAGMA journal_mode=WAL`,
		`CREATE TABLE IF NOT EXISTS hosts (id TEXT PRIMARY KEY,labels TEXT NOT NULL,at_ns INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS host_tokens (host_id TEXT PRIMARY KEY,token_hash BLOB NOT NULL,revoked INTEGER NOT NULL DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS samples (host_id TEXT NOT NULL, name TEXT NOT NULL, at_ns INTEGER NOT NULL, state TEXT NOT NULL, detail TEXT NOT NULL, vals TEXT NOT NULL, PRIMARY KEY(host_id,name,at_ns))`,
		`CREATE INDEX IF NOT EXISTS samples_time ON samples(at_ns)`,
		`CREATE TABLE IF NOT EXISTS aggregates (period TEXT NOT NULL,host_id TEXT NOT NULL,name TEXT NOT NULL,metric TEXT NOT NULL,bucket_ns INTEGER NOT NULL,n INTEGER NOT NULL,sum REAL NOT NULL,min REAL NOT NULL,max REAL NOT NULL,PRIMARY KEY(period,host_id,name,metric,bucket_ns))`,
		`CREATE TABLE IF NOT EXISTS ai_sessions(host_id TEXT NOT NULL,id TEXT NOT NULL,at_ns INTEGER NOT NULL,source TEXT NOT NULL,data TEXT NOT NULL,PRIMARY KEY(host_id,id))`,
		`CREATE TABLE IF NOT EXISTS ai_usage(host_id TEXT NOT NULL,id TEXT NOT NULL,session_id TEXT NOT NULL,at_ns INTEGER NOT NULL,source TEXT NOT NULL,data TEXT NOT NULL,PRIMARY KEY(host_id,id))`,
		`CREATE INDEX IF NOT EXISTS ai_usage_time ON ai_usage(host_id,at_ns)`,
		`CREATE TABLE IF NOT EXISTS ai_usage_hourly(host_id TEXT NOT NULL,bucket_ns INTEGER NOT NULL,kind TEXT NOT NULL,model TEXT NOT NULL,project TEXT NOT NULL,data TEXT NOT NULL,PRIMARY KEY(host_id,bucket_ns,kind,model,project))`,
		`CREATE TABLE IF NOT EXISTS ai_usage_daily(host_id TEXT NOT NULL,bucket_ns INTEGER NOT NULL,kind TEXT NOT NULL,model TEXT NOT NULL,project TEXT NOT NULL,data TEXT NOT NULL,PRIMARY KEY(host_id,bucket_ns,kind,model,project))`,
		`CREATE TABLE IF NOT EXISTS ai_quotas(host_id TEXT NOT NULL,kind TEXT NOT NULL,window TEXT NOT NULL,at_ns INTEGER NOT NULL,data TEXT NOT NULL,PRIMARY KEY(host_id,kind,window,at_ns))`,
		`CREATE TABLE IF NOT EXISTS ai_quotas_hourly(host_id TEXT NOT NULL,kind TEXT NOT NULL,window TEXT NOT NULL,bucket_ns INTEGER NOT NULL,at_ns INTEGER NOT NULL,data TEXT NOT NULL,PRIMARY KEY(host_id,kind,window,bucket_ns))`,
		`CREATE TABLE IF NOT EXISTS ai_quotas_daily(host_id TEXT NOT NULL,kind TEXT NOT NULL,window TEXT NOT NULL,bucket_ns INTEGER NOT NULL,at_ns INTEGER NOT NULL,data TEXT NOT NULL,PRIMARY KEY(host_id,kind,window,bucket_ns))`,
		`CREATE TABLE IF NOT EXISTS changes(host_id TEXT NOT NULL,id TEXT NOT NULL,at_ns INTEGER NOT NULL,kind TEXT NOT NULL,text TEXT NOT NULL,PRIMARY KEY(host_id,id))`,
		`CREATE INDEX IF NOT EXISTS changes_time ON changes(host_id,at_ns)`,
		`CREATE TABLE IF NOT EXISTS energy_intervals(host_id TEXT NOT NULL,at_ns INTEGER NOT NULL,day TEXT NOT NULL,kwh REAL NOT NULL,seconds REAL NOT NULL,PRIMARY KEY(host_id,at_ns,day))`,
		`CREATE INDEX IF NOT EXISTS energy_day ON energy_intervals(host_id,day)`,
		`CREATE TABLE IF NOT EXISTS probe_samples(host_id TEXT NOT NULL,name TEXT NOT NULL,at_ns INTEGER NOT NULL,state TEXT NOT NULL,latency_ms REAL NOT NULL,status_code INTEGER NOT NULL,dns_ms REAL NOT NULL,connect_ms REAL NOT NULL,tls_ms REAL NOT NULL,first_byte_ms REAL NOT NULL,certificate_days INTEGER,interval_s INTEGER NOT NULL,PRIMARY KEY(host_id,name,at_ns))`,
		`CREATE INDEX IF NOT EXISTS probe_samples_time ON probe_samples(at_ns)`,
		`CREATE TABLE IF NOT EXISTS probe_definitions(host_id TEXT NOT NULL,name TEXT NOT NULL,display_name TEXT NOT NULL,kind TEXT NOT NULL,public INTEGER NOT NULL,PRIMARY KEY(host_id,name))`,
		`CREATE TABLE IF NOT EXISTS app_latest(host_id TEXT NOT NULL,name TEXT NOT NULL,kind TEXT NOT NULL,at_ns INTEGER NOT NULL,data TEXT NOT NULL,PRIMARY KEY(host_id,kind,name))`,
		`CREATE TABLE IF NOT EXISTS app_minute(host_id TEXT NOT NULL,name TEXT NOT NULL,kind TEXT NOT NULL,bucket_ns INTEGER NOT NULL,data TEXT NOT NULL,PRIMARY KEY(host_id,kind,name,bucket_ns))`,
		`CREATE INDEX IF NOT EXISTS app_minute_time ON app_minute(host_id,bucket_ns)`,
		`CREATE TABLE IF NOT EXISTS app_hourly(host_id TEXT NOT NULL,name TEXT NOT NULL,kind TEXT NOT NULL,bucket_ns INTEGER NOT NULL,n INTEGER NOT NULL,data TEXT NOT NULL,PRIMARY KEY(host_id,kind,name,bucket_ns))`,
		`CREATE INDEX IF NOT EXISTS app_hourly_time ON app_hourly(host_id,bucket_ns)`,
		`CREATE TABLE IF NOT EXISTS network_daily(host_id TEXT NOT NULL,name TEXT NOT NULL,day TEXT NOT NULL,rx_bytes REAL NOT NULL,tx_bytes REAL NOT NULL,PRIMARY KEY(host_id,name,day))`,
		`CREATE TABLE IF NOT EXISTS app_io_daily(host_id TEXT NOT NULL,name TEXT NOT NULL,kind TEXT NOT NULL,day TEXT NOT NULL,read_bytes REAL NOT NULL,write_bytes REAL NOT NULL,PRIMARY KEY(host_id,kind,name,day))`,
		`CREATE TABLE IF NOT EXISTS drive_io_daily(host_id TEXT NOT NULL,name TEXT NOT NULL,day TEXT NOT NULL,read_bytes REAL NOT NULL,write_bytes REAL NOT NULL,PRIMARY KEY(host_id,name,day))`,
		`CREATE TABLE IF NOT EXISTS project_latest(host_id TEXT NOT NULL,pid INTEGER NOT NULL,at_ns INTEGER NOT NULL,data TEXT NOT NULL,PRIMARY KEY(host_id,pid))`,
		`CREATE TABLE IF NOT EXISTS worktree_latest(host_id TEXT NOT NULL,path TEXT NOT NULL,at_ns INTEGER NOT NULL,data TEXT NOT NULL,PRIMARY KEY(host_id,path))`,
		`CREATE TABLE IF NOT EXISTS storage_history(host_id TEXT NOT NULL,root TEXT NOT NULL,path TEXT NOT NULL,at_ns INTEGER NOT NULL,size_bytes INTEGER NOT NULL,complete INTEGER NOT NULL,depth INTEGER NOT NULL,PRIMARY KEY(host_id,path,at_ns))`,
		`CREATE INDEX IF NOT EXISTS storage_history_time ON storage_history(host_id,at_ns)`,
	} {
		if _, e = db.Exec(q); e != nil {
			db.Close()
			return nil, e
		}
	}
	columns, e := db.Query(`PRAGMA table_info(samples)`)
	if e != nil {
		db.Close()
		return nil, e
	}
	hasCadence, hasFields := false, false
	for columns.Next() {
		var cid, notNull, primaryKey int
		var name, kind string
		var defaultValue sql.NullString
		if e = columns.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); e != nil {
			columns.Close()
			db.Close()
			return nil, e
		}
		hasCadence = hasCadence || name == "cadence_s"
		hasFields = hasFields || name == "fields"
	}
	e = columns.Err()
	columns.Close()
	if e != nil {
		db.Close()
		return nil, e
	}
	if !hasCadence {
		if _, e = db.Exec(`ALTER TABLE samples ADD COLUMN cadence_s INTEGER NOT NULL DEFAULT 15`); e != nil {
			db.Close()
			return nil, e
		}
		if _, e = db.Exec(`UPDATE samples SET cadence_s=300 WHERE name='ports' OR name LIKE 'port:%' OR name LIKE 'tls:%' OR name LIKE 'smart:%' OR name LIKE 'helper:%'`); e != nil {
			db.Close()
			return nil, e
		}
	}
	if !hasFields {
		if _, e = db.Exec(`ALTER TABLE samples ADD COLUMN fields TEXT NOT NULL DEFAULT '{}'`); e != nil {
			db.Close()
			return nil, e
		}
	}
	return &Store{db}, nil
}
func (s *Store) Close() error { return s.DB.Close() }
func (s *Store) Ingest(b model.Batch) error {
	if b.HostID == "" || b.At.IsZero() {
		return errors.New("host_id and at required")
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	labels, _ := json.Marshal(b.Labels)
	_, e = tx.Exec(`INSERT INTO hosts(id,labels,at_ns) VALUES(?,?,?) ON CONFLICT(id) DO UPDATE SET labels=CASE WHEN excluded.labels='null' OR excluded.labels='{}' THEN hosts.labels ELSE excluded.labels END,at_ns=excluded.at_ns`, b.HostID, string(labels), b.At.UnixNano())
	if e != nil {
		return e
	}
	if e = ingestNetworkTraffic(tx, b); e != nil {
		return e
	}
	if e = ingestDriveTraffic(tx, &b); e != nil {
		return e
	}
	for _, r := range b.Results {
		if r.Name == "" {
			return errors.New("result name required")
		}
		switch r.State {
		case model.OK, model.Warn, model.Crit, model.Unknown:
		default:
			return errors.New("invalid state")
		}
		v, _ := json.Marshal(r.Values)
		fields, _ := json.Marshal(r.Fields)
		if e = ingestContainerImageChange(tx, b.HostID, b.At, r); e != nil {
			return e
		}
		cadence := r.CadenceSeconds
		if cadence != 15 && cadence != 300 {
			cadence = cadenceForName(r.Name)
		}
		_, e = tx.Exec(`INSERT OR REPLACE INTO samples(host_id,name,at_ns,state,detail,vals,cadence_s,fields) VALUES(?,?,?,?,?,?,?,?)`, b.HostID, r.Name, b.At.UnixNano(), r.State, r.Detail, string(v), cadence, string(fields))
		if e != nil {
			return e
		}
	}
	if e := ingestChanges(tx, b); e != nil {
		return e
	}
	if e := ingestEnergy(tx, b); e != nil {
		return e
	}
	if e := ingestAI(tx, b.HostID, b.AI); e != nil {
		return e
	}
	if e := ingestProbes(tx, b); e != nil {
		return e
	}
	if e := ingestApps(tx, b); e != nil {
		return e
	}
	if e := ingestObservations(tx, b); e != nil {
		return e
	}
	if e := ingestWorktrees(tx, b); e != nil {
		return e
	}
	return tx.Commit()
}
func (s *Store) Hosts() ([]model.Overview, error) {
	rows, e := s.DB.Query(`SELECT id,labels,at_ns FROM hosts ORDER BY id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.Overview{}
	for rows.Next() {
		var h model.Overview
		var labels string
		var ns int64
		if e := rows.Scan(&h.HostID, &labels, &ns); e != nil {
			return nil, e
		}
		json.Unmarshal([]byte(labels), &h.Labels)
		h.At = time.Unix(0, ns).UTC()
		h.Liveness = HostLiveness(h.At, time.Now())
		out = append(out, h)
	}
	return out, rows.Err()
}
func (s *Store) Overview(id string) (model.Overview, error) {
	var h model.Overview
	h.HostID = id
	var labels string
	var ns int64
	e := s.DB.QueryRow(`SELECT labels,at_ns FROM hosts WHERE id=?`, id).Scan(&labels, &ns)
	if e != nil {
		return h, e
	}
	json.Unmarshal([]byte(labels), &h.Labels)
	h.At = time.Unix(0, ns).UTC()
	h.Liveness = HostLiveness(h.At, time.Now())
	rows, e := s.DB.Query(`SELECT x.name,x.state,x.detail,x.vals,x.at_ns,x.cadence_s,x.fields FROM samples x JOIN (SELECT name,MAX(at_ns) at_ns FROM samples WHERE host_id=? GROUP BY name) y ON x.name=y.name AND x.at_ns=y.at_ns WHERE x.host_id=? AND x.at_ns>=? ORDER BY x.name`, id, id, time.Now().Add(-24*time.Hour).UnixNano())
	if e != nil {
		return h, e
	}
	defer rows.Close()
	h.Results = []model.Result{}
	for rows.Next() {
		var r model.Result
		var vals string
		var fields string
		var atNS int64
		if e := rows.Scan(&r.Name, &r.State, &r.Detail, &vals, &atNS, &r.CadenceSeconds, &fields); e != nil {
			return h, e
		}
		r.At = time.Unix(0, atNS).UTC()
		if r.CadenceSeconds != 15 && r.CadenceSeconds != 300 {
			r.CadenceSeconds = cadenceForName(r.Name)
		}
		if time.Since(r.At) > time.Duration(3*r.CadenceSeconds)*time.Second {
			r.State = model.Stale
		}
		json.Unmarshal([]byte(vals), &r.Values)
		json.Unmarshal([]byte(fields), &r.Fields)
		model.AnnotateResult(&r)
		h.Results = append(h.Results, r)
	}
	if err := rows.Err(); err != nil {
		return h, err
	}
	rows.Close()
	latest := map[string]time.Time{}
	for _, r := range h.Results {
		if r.State == model.Unknown || r.State == model.Stale {
			continue
		}
		if r.Name == "recent_errors" || r.Name == "security:sudo" || r.Name == "security:failed_logins" || r.Name == "security:sensitive_files" || strings.HasPrefix(r.Name, "ports:") || strings.HasPrefix(r.Name, "docker:") || strings.HasPrefix(r.Name, "repositories:") {
			latest[r.Name] = r.At
		}
	}
	filtered := h.Results[:0]
	for _, r := range h.Results {
		parent := ""
		switch {
		case strings.HasPrefix(r.Name, "recent_error:"), strings.HasPrefix(r.Name, "oom:"):
			parent = "recent_errors"
		case strings.HasPrefix(r.Name, "security:sudo_user:"):
			parent = "security:sudo"
		case strings.HasPrefix(r.Name, "security:failed_source:"):
			parent = "security:failed_logins"
		case strings.HasPrefix(r.Name, "security:file_change:"):
			parent = "security:sensitive_files"
		case strings.HasPrefix(r.Name, "container:"):
			parts := strings.SplitN(r.Name, ":", 3)
			if len(parts) > 1 {
				parent = "docker:" + parts[1]
			}
		case strings.HasPrefix(r.Name, "port:"):
			parts := strings.Split(r.Name, ":")
			if len(parts) > 2 {
				file := parts[1]
				if parts[2] == "ipv6" {
					file += "6"
				}
				parent = "ports:" + file
			}
		case strings.HasPrefix(r.Name, "repository:"):
			parts := strings.SplitN(r.Name, ":", 3)
			if len(parts) > 1 {
				parent = "repositories:" + parts[1]
			}
		}
		if at, ok := latest[parent]; parent != "" && ok && !r.At.Equal(at) {
			continue
		}
		filtered = append(filtered, r)
	}
	h.Results = filtered
	h.Agents, _ = s.Agents(id)
	return h, rows.Err()
}

func cadenceForName(name string) int {
	if name == "ports" || strings.HasPrefix(name, "port:") || strings.HasPrefix(name, "tls:") || strings.HasPrefix(name, "smart:") || strings.HasPrefix(name, "helper:") {
		return 300
	}
	return 15
}

type Point struct {
	At    time.Time `json:"at"`
	Value float64   `json:"value"`
}

func (s *Store) Series(id, metric, rangeName string) ([]Point, error) {
	return s.SeriesNamed(id, metric, rangeName, "")
}

// SeriesNamed limits a metric to one probe so similarly named values from
// memory, filesystems, and network interfaces are not mixed on a chart.
func (s *Store) SeriesNamed(id, metric, rangeName, name string) ([]Point, error) {
	dur, ok := map[string]time.Duration{"1h": time.Hour, "12h": 12 * time.Hour, "24h": 24 * time.Hour, "7d": 7 * 24 * time.Hour, "30d": 30 * 24 * time.Hour}[rangeName]
	if !ok {
		return nil, fmt.Errorf("invalid range")
	}
	now := time.Now()
	cutoff := now.Add(-dur).UnixNano()
	hourBoundary := now.Add(-time.Hour).UnixNano()
	dayBoundary := now.Add(-24 * time.Hour).UnixNano()
	out := []Point{}
	addRaw := func(start int64) error {
		rows, e := s.DB.Query(`SELECT at_ns,vals FROM samples WHERE host_id=? AND at_ns>=? AND (?='' OR name=?) ORDER BY at_ns`, id, start, name, name)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var ns int64
			var raw string
			if e := rows.Scan(&ns, &raw); e != nil {
				return e
			}
			var vals map[string]float64
			json.Unmarshal([]byte(raw), &vals)
			if v, ok := vals[metric]; ok {
				out = append(out, Point{time.Unix(0, ns).UTC(), v})
			}
		}
		return rows.Err()
	}
	addAgg := func(period string, start, end int64) error {
		rows, e := s.DB.Query(`SELECT bucket_ns,sum,n FROM aggregates WHERE host_id=? AND metric=? AND period=? AND bucket_ns>=? AND bucket_ns<? AND (?='' OR name=?) ORDER BY bucket_ns`, id, metric, period, start, end, name, name)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var ns int64
			var sum float64
			var n int
			if e := rows.Scan(&ns, &sum, &n); e != nil {
				return e
			}
			out = append(out, Point{time.Unix(0, ns).UTC(), sum / float64(n)})
		}
		return rows.Err()
	}
	if rangeName == "30d" {
		if e := addAgg("day", cutoff, dayBoundary); e != nil {
			return nil, e
		}
		if e := addAgg("hour", dayBoundary, hourBoundary); e != nil {
			return nil, e
		}
	}
	if rangeName == "7d" {
		if e := addAgg("hour", cutoff, hourBoundary); e != nil {
			return nil, e
		}
	}
	start := cutoff
	if rangeName == "7d" || rangeName == "30d" {
		start = hourBoundary
	}
	if e := addRaw(start); e != nil {
		return nil, e
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}
func (s *Store) Maintain(now time.Time) error {
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if _, e = tx.Exec(`DELETE FROM app_latest WHERE at_ns<?`, now.Add(-48*time.Hour).UnixNano()); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM project_latest WHERE at_ns<?`, now.Add(-48*time.Hour).UnixNano()); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM worktree_latest WHERE at_ns<?`, now.Add(-48*time.Hour).UnixNano()); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM storage_history WHERE at_ns<?`, now.Add(-31*24*time.Hour).UnixNano()); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM app_minute WHERE bucket_ns<?`, now.Add(-24*time.Hour).UnixNano()); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM app_hourly WHERE bucket_ns<?`, now.Add(-30*24*time.Hour).UnixNano()); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM network_daily WHERE day<?`, now.Add(-30*24*time.Hour).UTC().Format("2006-01-02")); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM app_io_daily WHERE day<?`, now.Add(-30*24*time.Hour).UTC().Format("2006-01-02")); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM drive_io_daily WHERE day<?`, now.Add(-30*24*time.Hour).UTC().Format("2006-01-02")); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM probe_samples WHERE at_ns<?`, now.Add(-31*24*time.Hour).UnixNano()); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM changes WHERE at_ns<?`, now.AddDate(-1, 0, 0).UnixNano()); e != nil {
		return e
	}
	if _, e = tx.Exec(`DELETE FROM energy_intervals WHERE day<?`, now.AddDate(-2, 0, 0).UTC().Format("2006-01-02")); e != nil {
		return e
	}
	hourCut := now.Add(-90 * 24 * time.Hour).UnixNano()
	hourEnd := now.Add(-time.Hour).UnixNano()
	if _, e = tx.Exec(`DELETE FROM aggregates WHERE period='hour' AND bucket_ns>=?`, now.Add(-7*24*time.Hour).UnixNano()); e != nil {
		return e
	}
	rows, e := tx.Query(`SELECT host_id,name,at_ns,vals FROM samples WHERE at_ns>=? AND at_ns<?`, hourCut, hourEnd)
	if e != nil {
		return e
	}
	type key struct {
		host, name, metric string
		bucket             int64
	}
	type agg struct {
		n             int
		sum, min, max float64
	}
	hourly := map[key]agg{}
	for rows.Next() {
		var host, name, raw string
		var ns int64
		if e = rows.Scan(&host, &name, &ns, &raw); e != nil {
			rows.Close()
			return e
		}
		var vals map[string]float64
		json.Unmarshal([]byte(raw), &vals)
		for metric, v := range vals {
			k := key{host, name, metric, ns / int64(time.Hour) * int64(time.Hour)}
			x := hourly[k]
			if x.n == 0 || v < x.min {
				x.min = v
			}
			if x.n == 0 || v > x.max {
				x.max = v
			}
			x.n++
			x.sum += v
			hourly[k] = x
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for k, x := range hourly {
		if _, e = tx.Exec(`INSERT OR REPLACE INTO aggregates VALUES('hour',?,?,?,?,?,?,?,?)`, k.host, k.name, k.metric, k.bucket, x.n, x.sum, x.min, x.max); e != nil {
			return e
		}
	}
	dayCut := now.Add(-90 * 24 * time.Hour).UnixNano()
	dayEnd := now.Add(-24 * time.Hour).UnixNano()
	if _, e = tx.Exec(`DELETE FROM aggregates WHERE period='day' AND bucket_ns>=?`, dayCut); e != nil {
		return e
	}
	rows, e = tx.Query(`SELECT host_id,name,metric,bucket_ns,n,sum,min,max FROM aggregates WHERE period='hour' AND bucket_ns>=? AND bucket_ns<?`, dayCut, dayEnd)
	if e != nil {
		return e
	}
	daily := map[key]agg{}
	for rows.Next() {
		var host, name, metric string
		var ns int64
		var x agg
		if e = rows.Scan(&host, &name, &metric, &ns, &x.n, &x.sum, &x.min, &x.max); e != nil {
			rows.Close()
			return e
		}
		k := key{host, name, metric, ns / int64(24*time.Hour) * int64(24*time.Hour)}
		v := daily[k]
		if v.n == 0 || x.min < v.min {
			v.min = x.min
		}
		if v.n == 0 || x.max > v.max {
			v.max = x.max
		}
		v.n += x.n
		v.sum += x.sum
		daily[k] = v
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for k, x := range daily {
		if _, e = tx.Exec(`INSERT INTO aggregates VALUES('day',?,?,?,?,?,?,?,?)`, k.host, k.name, k.metric, k.bucket, x.n, x.sum, x.min, x.max); e != nil {
			return e
		}
	}
	for _, q := range []struct {
		table, period string
		cutoff        time.Duration
	}{{"samples", "", 7 * 24 * time.Hour}, {"aggregates", "hour", 90 * 24 * time.Hour}, {"aggregates", "day", 2 * 365 * 24 * time.Hour}} {
		if q.period == "" {
			_, e = tx.Exec(`DELETE FROM samples WHERE at_ns<?`, now.Add(-q.cutoff).UnixNano())
		} else {
			_, e = tx.Exec(`DELETE FROM aggregates WHERE period=? AND bucket_ns<?`, q.period, now.Add(-q.cutoff).UnixNano())
		}
		if e != nil {
			return e
		}
	}
	if e := maintainAI(tx, now); e != nil {
		return e
	}
	return tx.Commit()
}
