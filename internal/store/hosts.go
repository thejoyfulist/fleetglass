package store

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"time"
)

var hostName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func ValidHostName(name string) bool { return hostName.MatchString(name) }

// HostLiveness reflects the fast, fifteen-second reporting cadence.
func HostLiveness(last, now time.Time) string {
	if last.IsZero() || last.UnixNano() <= 0 || now.Sub(last) >= 24*time.Hour {
		return "offline"
	}
	if now.Sub(last) >= 45*time.Second {
		return "silent"
	}
	return "live"
}

func (s *Store) AddHost(name string, labels map[string]string, token string) error {
	if !ValidHostName(name) || token == "" {
		return errors.New("invalid host or token")
	}
	b, err := json.Marshal(labels)
	if err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(token))
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`INSERT INTO hosts(id,labels,at_ns) VALUES(?,?,0) ON CONFLICT(id) DO NOTHING`, name, string(b)); err != nil {
		return err
	}
	if len(labels) > 0 {
		if _, err = tx.Exec(`UPDATE hosts SET labels=? WHERE id=?`, string(b), name); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(`INSERT INTO host_tokens(host_id,token_hash) VALUES(?,?)`, name, hash[:]); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AuthorizeHost(name, token string) (bool, error) {
	if token == "" || !ValidHostName(name) {
		return false, nil
	}
	var hash []byte
	var revoked int
	err := s.DB.QueryRow(`SELECT token_hash,revoked FROM host_tokens WHERE host_id=?`, name).Scan(&hash, &revoked)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	got := sha256.Sum256([]byte(token))
	return revoked == 0 && subtle.ConstantTimeCompare(got[:], hash) == 1, nil
}

func (s *Store) RevokeHost(name string) error {
	r, err := s.DB.Exec(`UPDATE host_tokens SET revoked=1 WHERE host_id=? AND revoked=0`, name)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errors.New("active host token not found")
	}
	return nil
}

func (s *Store) RemoveHost(name string, purge bool) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM host_tokens WHERE host_id=?`, name); err != nil {
		return err
	}
	if purge {
		for _, table := range []string{"samples", "aggregates", "ai_sessions", "ai_usage", "ai_usage_hourly", "ai_usage_daily", "ai_quotas", "ai_quotas_hourly", "ai_quotas_daily", "changes", "energy_intervals", "probe_samples", "probe_definitions", "worktree_latest"} {
			if _, err = tx.Exec(`DELETE FROM `+table+` WHERE host_id=?`, name); err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec(`DELETE FROM hosts WHERE id=?`, name); err != nil {
		return err
	}
	return tx.Commit()
}

type HostRegistration struct {
	Name     string            `json:"name"`
	Labels   map[string]string `json:"labels,omitempty"`
	Liveness string            `json:"liveness"`
	Revoked  bool              `json:"revoked"`
}

func (s *Store) HostRegistrations() ([]HostRegistration, error) {
	rows, err := s.DB.Query(`SELECT h.id,h.labels,h.at_ns,COALESCE(t.revoked,0) FROM hosts h LEFT JOIN host_tokens t ON t.host_id=h.id ORDER BY h.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HostRegistration{}
	for rows.Next() {
		var h HostRegistration
		var labels string
		var at int64
		var revoked int
		if err := rows.Scan(&h.Name, &labels, &at, &revoked); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(labels), &h.Labels)
		h.Liveness = HostLiveness(time.Unix(0, at), time.Now())
		h.Revoked = revoked != 0
		out = append(out, h)
	}
	return out, rows.Err()
}
