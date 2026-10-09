package store

import (
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type TrafficTotal struct {
	Name    string  `json:"name"`
	TodayRX float64 `json:"today_rx_bytes"`
	TodayTX float64 `json:"today_tx_bytes"`
	WeekRX  float64 `json:"week_rx_bytes"`
	WeekTX  float64 `json:"week_tx_bytes"`
	MonthRX float64 `json:"month_rx_bytes"`
	MonthTX float64 `json:"month_tx_bytes"`
}

func ingestNetworkTraffic(tx *sql.Tx, b model.Batch) error {
	for _, r := range b.Results {
		if !strings.HasPrefix(r.Name, "network:") || r.Values == nil {
			continue
		}
		rx, ok1 := r.Values["rx_bytes"]
		txBytes, ok2 := r.Values["tx_bytes"]
		if !ok1 || !ok2 {
			continue
		}
		var exists int
		err := tx.QueryRow(`SELECT 1 FROM samples WHERE host_id=? AND name=? AND at_ns=?`, b.HostID, r.Name, b.At.UnixNano()).Scan(&exists)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return err
		}
		var raw string
		var ns int64
		err = tx.QueryRow(`SELECT vals,at_ns FROM samples WHERE host_id=? AND name=? AND at_ns<? ORDER BY at_ns DESC LIMIT 1`, b.HostID, r.Name, b.At.UnixNano()).Scan(&raw, &ns)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		if b.At.Sub(time.Unix(0, ns)) > time.Hour {
			continue
		}
		var prev map[string]float64
		if json.Unmarshal([]byte(raw), &prev) != nil {
			continue
		}
		oldRX, ok1 := prev["rx_bytes"]
		oldTX, ok2 := prev["tx_bytes"]
		if !ok1 || !ok2 || rx < oldRX || txBytes < oldTX {
			continue
		}
		_, err = tx.Exec(`INSERT INTO network_daily VALUES(?,?,?,?,?) ON CONFLICT(host_id,name,day) DO UPDATE SET rx_bytes=rx_bytes+excluded.rx_bytes,tx_bytes=tx_bytes+excluded.tx_bytes`, b.HostID, r.Name, b.At.UTC().Format("2006-01-02"), rx-oldRX, txBytes-oldTX)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) Traffic(host string, now time.Time) ([]TrafficTotal, error) {
	rows, err := s.DB.Query(`SELECT name,day,rx_bytes,tx_bytes FROM network_daily WHERE host_id=? AND day>=? ORDER BY name`, host, now.UTC().AddDate(0, 0, -29).Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all := map[string]*TrafficTotal{}
	today := now.UTC().Format("2006-01-02")
	week := now.UTC().AddDate(0, 0, -6).Format("2006-01-02")
	for rows.Next() {
		var name, day string
		var rx, tx float64
		if err = rows.Scan(&name, &day, &rx, &tx); err != nil {
			return nil, err
		}
		v := all[name]
		if v == nil {
			v = &TrafficTotal{Name: name}
			all[name] = v
		}
		v.MonthRX += rx
		v.MonthTX += tx
		if day >= week {
			v.WeekRX += rx
			v.WeekTX += tx
		}
		if day == today {
			v.TodayRX += rx
			v.TodayTX += tx
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := []TrafficTotal{}
	for _, v := range all {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
