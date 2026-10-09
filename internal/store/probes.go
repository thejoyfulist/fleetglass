package store

import (
	"database/sql"
	"errors"
	"math"
	"sort"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type ProbePoint struct {
	At        time.Time   `json:"at"`
	LatencyMS float64     `json:"latency_ms"`
	State     model.State `json:"state"`
}
type ProbeDay struct {
	Date     string      `json:"date"`
	State    model.State `json:"state"`
	Uptime   *float64    `json:"uptime"`
	Coverage float64     `json:"coverage"`
}
type ProbeLatency struct {
	P50MS *float64 `json:"p50_ms"`
	P95MS *float64 `json:"p95_ms"`
}
type ProbeSummary struct {
	Name            string                  `json:"name"`
	DisplayName     string                  `json:"display_name"`
	Kind            string                  `json:"kind"`
	State           model.State             `json:"state"`
	Uptime24H       *float64                `json:"uptime_24h"`
	Uptime7D        *float64                `json:"uptime_7d"`
	Uptime30D       *float64                `json:"uptime_30d"`
	Coverage24H     float64                 `json:"coverage_24h"`
	Coverage7D      float64                 `json:"coverage_7d"`
	Coverage30D     float64                 `json:"coverage_30d"`
	Days            []ProbeDay              `json:"days"`
	P50MS           *float64                `json:"p50_ms"`
	P95MS           *float64                `json:"p95_ms"`
	Percentiles     map[string]ProbeLatency `json:"percentiles"`
	StatusCode      int                     `json:"status_code,omitempty"`
	LatencyMS       float64                 `json:"latency_ms,omitempty"`
	DNSMS           float64                 `json:"dns_ms,omitempty"`
	ConnectMS       float64                 `json:"connect_ms,omitempty"`
	TLSMS           float64                 `json:"tls_ms,omitempty"`
	FirstByteMS     float64                 `json:"first_byte_ms,omitempty"`
	CertificateDays *int                    `json:"certificate_days,omitempty"`
	Points          []ProbePoint            `json:"points"`
}
type PublicProbe struct {
	DisplayName string      `json:"display_name"`
	State       model.State `json:"state"`
	Uptime30D   *float64    `json:"uptime_30d"`
	Coverage30D float64     `json:"coverage_30d"`
	Days        []ProbeDay  `json:"days"`
	LatencyMS   *int        `json:"latency_ms,omitempty"`
}

func ingestProbes(tx *sql.Tx, b model.Batch) error {
	if b.ProbeNames != nil {
		keep := map[string]bool{}
		for _, name := range *b.ProbeNames {
			if name == "" || keep[name] {
				return errors.New("invalid probe names")
			}
			keep[name] = true
		}
		rows, err := tx.Query(`SELECT name FROM probe_definitions WHERE host_id=?`, b.HostID)
		if err != nil {
			return err
		}
		remove := []string{}
		for rows.Next() {
			var name string
			if err = rows.Scan(&name); err != nil {
				rows.Close()
				return err
			}
			if !keep[name] {
				remove = append(remove, name)
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, name := range remove {
			if _, err = tx.Exec(`DELETE FROM probe_definitions WHERE host_id=? AND name=?`, b.HostID, name); err != nil {
				return err
			}
		}
	}
	for _, p := range b.Probes {
		if b.ProbeNames != nil {
			found := false
			for _, name := range *b.ProbeNames {
				if name == p.Name {
					found = true
					break
				}
			}
			if !found {
				return errors.New("unconfigured probe sample")
			}
		}
		if p.Name == "" || p.DisplayName == "" || p.IntervalSeconds < 30 || p.IntervalSeconds > 86400 || p.At.IsZero() || p.At.After(b.At.Add(time.Minute)) || p.At.Before(b.At.Add(-time.Hour)) {
			return errors.New("invalid probe sample")
		}
		if p.State != model.OK && p.State != model.Crit && p.State != model.Unknown {
			return errors.New("invalid probe state")
		}
		if p.Kind != "http" && p.Kind != "tcp" {
			return errors.New("invalid probe kind")
		}
		if math.IsNaN(p.LatencyMS) || math.IsInf(p.LatencyMS, 0) || p.LatencyMS < 0 {
			return errors.New("invalid probe latency")
		}
		public := 0
		if p.Public {
			public = 1
		}
		if _, err := tx.Exec(`INSERT INTO probe_definitions VALUES(?,?,?,?,?) ON CONFLICT(host_id,name) DO UPDATE SET display_name=excluded.display_name,kind=excluded.kind,public=excluded.public`, b.HostID, p.Name, p.DisplayName, p.Kind, public); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT OR REPLACE INTO probe_samples VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, b.HostID, p.Name, p.At.UnixNano(), p.State, p.LatencyMS, p.StatusCode, p.DNSMS, p.ConnectMS, p.TLSMS, p.FirstByteMS, p.CertificateDays, p.IntervalSeconds); err != nil {
			return err
		}
	}
	return nil
}

type storedProbe struct {
	at                           time.Time
	state                        model.State
	latency                      float64
	status                       int
	dns, connect, tls, firstByte float64
	cert                         sql.NullInt64
	interval                     time.Duration
}

func (s *Store) Probes(host string, now time.Time) ([]ProbeSummary, error) {
	defs, err := s.DB.Query(`SELECT name,display_name,kind FROM probe_definitions WHERE host_id=? ORDER BY display_name`, host)
	if err != nil {
		return nil, err
	}
	defer defs.Close()
	out := []ProbeSummary{}
	for defs.Next() {
		var p ProbeSummary
		if err = defs.Scan(&p.Name, &p.DisplayName, &p.Kind); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err = defs.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err = s.fillProbe(host, &out[i], now); err != nil {
			return nil, err
		}
	}
	return out, nil
}
func (s *Store) fillProbe(host string, p *ProbeSummary, now time.Time) error {
	since := now.Add(-30 * 24 * time.Hour)
	rows, err := s.DB.Query(`SELECT at_ns,state,latency_ms,status_code,dns_ms,connect_ms,tls_ms,first_byte_ms,certificate_days,interval_s FROM probe_samples WHERE host_id=? AND name=? AND at_ns>=? AND at_ns<=? ORDER BY at_ns`, host, p.Name, since.Add(-24*time.Hour).UnixNano(), now.UnixNano())
	if err != nil {
		return err
	}
	defer rows.Close()
	samples := []storedProbe{}
	for rows.Next() {
		var x storedProbe
		var ns, interval int64
		if err = rows.Scan(&ns, &x.state, &x.latency, &x.status, &x.dns, &x.connect, &x.tls, &x.firstByte, &x.cert, &interval); err != nil {
			return err
		}
		x.at = time.Unix(0, ns).UTC()
		x.interval = time.Duration(interval) * time.Second
		samples = append(samples, x)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	p.State = model.Unknown
	if len(samples) > 0 {
		last := samples[len(samples)-1]
		if now.Sub(last.at) <= last.interval+last.interval/2 {
			p.State = last.state
			p.StatusCode = last.status
			p.LatencyMS = last.latency
			p.DNSMS = last.dns
			p.ConnectMS = last.connect
			p.TLSMS = last.tls
			p.FirstByteMS = last.firstByte
			if last.cert.Valid {
				v := int(last.cert.Int64)
				p.CertificateDays = &v
			}
		}
	}
	p.Uptime24H, p.Coverage24H = availability(samples, now.Add(-24*time.Hour), now)
	p.Uptime7D, p.Coverage7D = availability(samples, now.Add(-7*24*time.Hour), now)
	p.Uptime30D, p.Coverage30D = availability(samples, since, now)
	p.Days = []ProbeDay{}
	for j := 29; j >= 0; j-- {
		start := now.UTC().Truncate(24*time.Hour).AddDate(0, 0, -j)
		end := start.Add(24 * time.Hour)
		if end.After(now) {
			end = now
		}
		pct, coverage := availability(samples, start, end)
		state := model.Unknown
		up, down := availabilityParts(samples, start, end)
		if down > 0 {
			state = model.Crit
		} else if pct != nil && *pct < 99 {
			state = model.Warn
		} else if up > 0 {
			state = model.OK
		}
		p.Days = append(p.Days, ProbeDay{Date: start.Format("2006-01-02"), State: state, Uptime: pct, Coverage: coverage})
	}
	p.Percentiles = map[string]ProbeLatency{}
	for _, period := range []struct {
		name     string
		duration time.Duration
	}{{"1h", time.Hour}, {"12h", 12 * time.Hour}, {"24h", 24 * time.Hour}, {"7d", 7 * 24 * time.Hour}, {"30d", 30 * 24 * time.Hour}} {
		latencies := []float64{}
		cutoff := now.Add(-period.duration)
		for _, x := range samples {
			if !x.at.Before(cutoff) && x.state == model.OK {
				latencies = append(latencies, x.latency)
			}
		}
		sort.Float64s(latencies)
		stats := ProbeLatency{}
		if len(latencies) > 0 {
			a := percentile(latencies, .5)
			b := percentile(latencies, .95)
			stats.P50MS = &a
			stats.P95MS = &b
		}
		p.Percentiles[period.name] = stats
	}
	p.P50MS = p.Percentiles["30d"].P50MS
	p.P95MS = p.Percentiles["30d"].P95MS
	p.Points = []ProbePoint{}
	for _, x := range samples {
		if x.at.Before(since) {
			continue
		}
		p.Points = append(p.Points, ProbePoint{At: x.at, LatencyMS: x.latency, State: x.state})
	}
	if len(p.Points) > 360 {
		older, middle, recent := []ProbePoint{}, []ProbePoint{}, []ProbePoint{}
		for _, point := range p.Points {
			switch {
			case !point.At.Before(now.Add(-time.Hour)):
				recent = append(recent, point)
			case !point.At.Before(now.Add(-12 * time.Hour)):
				middle = append(middle, point)
			default:
				older = append(older, point)
			}
		}
		p.Points = append(append(reduceProbePoints(older, 120), reduceProbePoints(middle, 120)...), reduceProbePoints(recent, 120)...)
	}
	return nil
}
func reduceProbePoints(points []ProbePoint, limit int) []ProbePoint {
	if len(points) <= limit {
		return points
	}
	step := int(math.Ceil(float64(len(points)) / float64(limit)))
	out := make([]ProbePoint, 0, limit)
	for j := 0; j < len(points); j += step {
		end := j + step
		if end > len(points) {
			end = len(points)
		}
		chosen := points[j]
		for _, point := range points[j:end] {
			if point.State == model.Crit || chosen.State != model.Crit && (point.State == model.Unknown || chosen.State == model.OK && point.LatencyMS > chosen.LatencyMS) {
				chosen = point
			}
		}
		out = append(out, chosen)
	}
	return out
}
func availability(samples []storedProbe, start, end time.Time) (*float64, float64) {
	if !end.After(start) {
		return nil, 0
	}
	up, down := availabilityParts(samples, start, end)
	known := up + down
	if known == 0 {
		return nil, 0
	}
	v := 100 * up / known
	return &v, known / end.Sub(start).Seconds()
}
func availabilityParts(samples []storedProbe, start, end time.Time) (float64, float64) {
	var up, down float64
	for i, x := range samples {
		finish := x.at.Add(x.interval)
		if i+1 < len(samples) && samples[i+1].at.Before(finish) {
			finish = samples[i+1].at
		}
		if finish.After(end) {
			finish = end
		}
		begin := x.at
		if begin.Before(start) {
			begin = start
		}
		if !finish.After(begin) {
			continue
		}
		seconds := finish.Sub(begin).Seconds()
		if x.state == model.OK {
			up += seconds
		} else if x.state == model.Crit {
			down += seconds
		}
	}
	return up, down
}
func percentile(sorted []float64, q float64) float64 {
	i := q * float64(len(sorted)-1)
	low := int(i)
	high := int(math.Ceil(i))
	return sorted[low] + (sorted[high]-sorted[low])*(i-float64(low))
}

func (s *Store) PublicStatus(now time.Time) ([]PublicProbe, error) {
	rows, err := s.DB.Query(`SELECT host_id,name,display_name,kind FROM probe_definitions WHERE public=1 ORDER BY display_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type def struct {
		host string
		p    ProbeSummary
	}
	defs := []def{}
	for rows.Next() {
		var d def
		if err = rows.Scan(&d.host, &d.p.Name, &d.p.DisplayName, &d.p.Kind); err != nil {
			return nil, err
		}
		defs = append(defs, d)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := []PublicProbe{}
	for _, d := range defs {
		if err = s.fillProbe(d.host, &d.p, now); err != nil {
			return nil, err
		}
		pub := PublicProbe{DisplayName: d.p.DisplayName, State: d.p.State, Uptime30D: d.p.Uptime30D, Coverage30D: d.p.Coverage30D, Days: d.p.Days}
		if d.p.State == model.OK {
			v := int(math.Round(d.p.LatencyMS/10) * 10)
			pub.LatencyMS = &v
		}
		out = append(out, pub)
	}
	return out, nil
}
