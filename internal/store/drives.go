package store

import (
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func ingestDriveTraffic(tx *sql.Tx, b *model.Batch) error {
	for i := range b.Results {
		r := &b.Results[i]
		if !strings.HasPrefix(r.Name, "drive:") || r.Values == nil {
			continue
		}
		read, ok1 := r.Values["read_bytes"]
		write, ok2 := r.Values["write_bytes"]
		if !ok1 || !ok2 {
			continue
		}
		var existing int
		err := tx.QueryRow(`SELECT 1 FROM samples WHERE host_id=? AND name=? AND at_ns=?`, b.HostID, r.Name, b.At.UnixNano()).Scan(&existing)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == sql.ErrNoRows {
			var raw string
			err = tx.QueryRow(`SELECT vals FROM samples WHERE host_id=? AND name=? AND at_ns<? ORDER BY at_ns DESC LIMIT 1`, b.HostID, r.Name, b.At.UnixNano()).Scan(&raw)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			if err == nil {
				var previous map[string]float64
				if json.Unmarshal([]byte(raw), &previous) == nil {
					oldRead, rOK := previous["read_bytes"]
					oldWrite, wOK := previous["write_bytes"]
					if rOK && wOK && read >= oldRead && write >= oldWrite {
						_, err = tx.Exec(`INSERT INTO drive_io_daily VALUES(?,?,?,?,?) ON CONFLICT(host_id,name,day) DO UPDATE SET read_bytes=read_bytes+excluded.read_bytes,write_bytes=write_bytes+excluded.write_bytes`, b.HostID, r.Name, b.At.UTC().Format("2006-01-02"), read-oldRead, write-oldWrite)
						if err != nil {
							return err
						}
					}
				}
			}
		}
		var today float64
		err = tx.QueryRow(`SELECT write_bytes FROM drive_io_daily WHERE host_id=? AND name=? AND day=?`, b.HostID, r.Name, b.At.UTC().Format("2006-01-02")).Scan(&today)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		r.Values["written_today_bytes"] = today
	}
	return nil
}
