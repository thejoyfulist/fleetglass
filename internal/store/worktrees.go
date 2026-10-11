package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func ingestWorktrees(tx *sql.Tx, batch model.Batch) error {
	for _, tree := range batch.Worktrees {
		if !filepath.IsAbs(tree.Root) || filepath.Clean(tree.Root) != tree.Root || tree.Project == "" || tree.Name == "" || strings.Contains(tree.Project, "/") || strings.Contains(tree.Name, "/") || tree.Path != filepath.Join(tree.Root, tree.Project, tree.Name) || tree.SizeBytes < 0 || tree.DirtyFiles < 0 || tree.AgentProcesses < 0 || tree.Processes < 0 || tree.AgentProcesses > tree.Processes {
			return errors.New("invalid worktree")
		}
		var oldRaw string
		var oldAt int64
		err := tx.QueryRow(`SELECT at_ns,data FROM worktree_latest WHERE host_id=? AND path=?`, batch.HostID, tree.Path).Scan(&oldAt, &oldRaw)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if oldAt > batch.At.UnixNano() {
			continue
		}
		if err == nil {
			var old model.WorktreeEntry
			if json.Unmarshal([]byte(oldRaw), &old) == nil {
				tree.LastChangeAt = old.LastChangeAt
				if old.Head != "" && tree.Head != "" && old.Head != tree.Head || old.StatusComplete && tree.StatusComplete && old.DirtyFiles != tree.DirtyFiles || !old.IndexMTime.IsZero() && !tree.IndexMTime.IsZero() && !old.IndexMTime.Equal(tree.IndexMTime) {
					tree.LastChangeAt = batch.At
				}
			}
		}
		data, err := json.Marshal(tree)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO worktree_latest VALUES(?,?,?,?) ON CONFLICT(host_id,path) DO UPDATE SET at_ns=excluded.at_ns,data=excluded.data WHERE excluded.at_ns>=worktree_latest.at_ns`, batch.HostID, tree.Path, batch.At.UnixNano(), string(data)); err != nil {
			return err
		}
	}
	return nil
}

type WorktreesView struct {
	Worktrees        []model.WorktreeEntry `json:"worktrees"`
	ReclaimableBytes int64                 `json:"reclaimable_bytes"`
	At               time.Time             `json:"at"`
}

func (s *Store) Worktrees(host string, now time.Time) (WorktreesView, error) {
	out := WorktreesView{Worktrees: []model.WorktreeEntry{}}
	rows, err := s.DB.Query(`SELECT at_ns,data FROM worktree_latest WHERE host_id=? AND at_ns>=? ORDER BY path`, host, now.Add(-2*time.Hour).UnixNano())
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var ns int64
		var data string
		if err = rows.Scan(&ns, &data); err != nil {
			return out, err
		}
		var tree model.WorktreeEntry
		if json.Unmarshal([]byte(data), &tree) != nil {
			continue
		}
		out.Worktrees = append(out.Worktrees, tree)
		if tree.Reclaimable {
			out.ReclaimableBytes += tree.SizeBytes
		}
		at := time.Unix(0, ns).UTC()
		if at.After(out.At) {
			out.At = at
		}
	}
	return out, rows.Err()
}
