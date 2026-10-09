package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func ingestContainerImageChange(tx *sql.Tx, host string, at time.Time, r model.Result) error {
	if !strings.HasPrefix(r.Name, "container:") || r.State == model.Unknown {
		return nil
	}
	current := r.Fields["image_hash"]
	if current == "" {
		return nil
	}
	var raw string
	var priorAt int64
	e := tx.QueryRow(`SELECT at_ns,fields FROM samples WHERE host_id=? AND name=? ORDER BY at_ns DESC LIMIT 1`, host, r.Name).Scan(&priorAt, &raw)
	if e == sql.ErrNoRows {
		return nil
	}
	if e != nil {
		return e
	}
	if priorAt >= at.UnixNano() {
		return nil
	}
	var prior map[string]string
	if json.Unmarshal([]byte(raw), &prior) != nil {
		return nil
	}
	old := prior["image_hash"]
	if old == "" || old == current {
		return nil
	}
	sum := sha256.Sum256([]byte(host + "\x00" + r.Name + "\x00" + current + "\x00" + at.UTC().Format(time.RFC3339Nano)))
	label := "Image changed: " + strings.TrimPrefix(r.Name, "container:")
	if len(label) > 160 {
		label = label[:160]
	}
	_, e = tx.Exec(`INSERT OR IGNORE INTO changes(host_id,id,at_ns,kind,text) VALUES(?,?,?,?,?)`, host, hex.EncodeToString(sum[:]), at.UnixNano(), "Container", label)
	return e
}

var changeID = regexp.MustCompile(`^[a-f0-9]{64}$`)
var changeKinds = map[string]bool{"APT": true, "Service": true, "Boot": true, "Git": true, "Container": true, "Marker": true, "Heat": true}

func ingestChanges(tx *sql.Tx, b model.Batch) error {
	if len(b.Changes) > 500 {
		return errors.New("too many changes")
	}
	for _, c := range b.Changes {
		if !changeID.MatchString(c.ID) || !changeKinds[c.Type] || c.At.IsZero() || c.At.After(b.At.Add(5*time.Minute)) || b.At.Sub(c.At) > 365*24*time.Hour || len(c.Text) > 160 {
			return errors.New("invalid change")
		}
		if _, e := tx.Exec(`INSERT OR IGNORE INTO changes(host_id,id,at_ns,kind,text) VALUES(?,?,?,?,?)`, b.HostID, c.ID, c.At.UnixNano(), c.Type, c.Text); e != nil {
			return e
		}
	}
	return nil
}

func (s *Store) Changes(host, rangeName, kind string) ([]model.Change, error) {
	var window time.Duration
	var e error
	if rangeName == "1y" {
		window = 365 * 24 * time.Hour
	} else {
		window, e = duration(rangeName)
	}
	if e != nil {
		return nil, e
	}
	if kind != "" && !changeKinds[kind] {
		return nil, errors.New("invalid change type")
	}
	q := `SELECT id,at_ns,kind,text FROM changes WHERE host_id=? AND at_ns>=?`
	args := []any{host, time.Now().Add(-window).UnixNano()}
	if kind != "" {
		q += ` AND kind=?`
		args = append(args, kind)
	}
	q += ` ORDER BY at_ns DESC LIMIT 500`
	rows, e := s.DB.Query(q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []model.Change{}
	for rows.Next() {
		var c model.Change
		var ns int64
		if e = rows.Scan(&c.ID, &ns, &c.Type, &c.Text); e != nil {
			return nil, e
		}
		c.At = time.Unix(0, ns).UTC()
		model.AnnotateChange(&c)
		out = append(out, c)
	}
	return out, rows.Err()
}

func ingestEnergy(tx *sql.Tx, b model.Batch) error {
	for _, r := range b.Results {
		if r.Name != "cpu_package_power" || r.State != model.OK {
			continue
		}
		if _, ok := r.Values["energy_kwh"]; !ok {
			continue
		}
		kwh, seconds := r.Values["energy_kwh"], r.Values["energy_seconds"]
		if math.IsNaN(kwh) || math.IsInf(kwh, 0) || math.IsNaN(seconds) || math.IsInf(seconds, 0) || kwh < 0 || kwh > 1 || seconds <= 0 || seconds > 600 {
			return errors.New("invalid energy interval")
		}
		end := b.At.UTC()
		start := end.Add(-time.Duration(seconds * float64(time.Second)))
		for start.Before(end) {
			day := start.Format("2006-01-02")
			boundary := start.Truncate(24 * time.Hour).Add(24 * time.Hour)
			partEnd := end
			if boundary.Before(end) {
				partEnd = boundary
			}
			part := partEnd.Sub(start).Seconds()
			if _, e := tx.Exec(`INSERT OR IGNORE INTO energy_intervals(host_id,at_ns,day,kwh,seconds) VALUES(?,?,?,?,?)`, b.HostID, b.At.UnixNano(), day, kwh*part/seconds, part); e != nil {
				return e
			}
			start = partEnd
		}
	}
	return nil
}

type Energy struct {
	TodayKWh     float64 `json:"today_kwh"`
	MonthKWh     float64 `json:"month_kwh"`
	TodayCost    float64 `json:"today_cost"`
	MonthCost    float64 `json:"month_cost"`
	TodaySeconds float64 `json:"today_seconds"`
	MonthSeconds float64 `json:"month_seconds"`
	PricePerKWh  float64 `json:"price_per_kwh"`
	Currency     string  `json:"currency"`
	Label        string  `json:"label"`
}

func (s *Store) Energy(host string, now time.Time, price float64, currency string) (Energy, error) {
	today := now.UTC().Format("2006-01-02")
	month := now.UTC().Format("2006-01")
	out := Energy{PricePerKWh: price, Currency: currency, Label: "CPU package only"}
	rows, e := s.DB.Query(`SELECT day,SUM(kwh),SUM(seconds) FROM energy_intervals WHERE host_id=? AND day>=? AND day<=? GROUP BY day`, host, month+"-01", today)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var day string
		var kwh, seconds float64
		if e = rows.Scan(&day, &kwh, &seconds); e != nil {
			return out, e
		}
		out.MonthKWh += kwh
		out.MonthSeconds += seconds
		if day == today {
			out.TodayKWh = kwh
			out.TodaySeconds = seconds
		}
	}
	out.TodayCost = out.TodayKWh * price
	out.MonthCost = out.MonthKWh * price
	return out, rows.Err()
}
