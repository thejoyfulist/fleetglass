package ai

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

func ApplyHealth(sessions []model.AgentSession, processes []Process, quotas []model.Quota, now time.Time) {
	limited := map[string]bool{}
	for _, q := range quotas {
		if q.Used >= 100 && q.ResetsAt.After(now) {
			limited[q.Kind] = true
		}
	}
	alive := map[int]bool{}
	for _, p := range processes {
		alive[p.PID] = true
	}
	for i := range sessions {
		s := &sessions[i]
		s.ReasonCode = ""
		if s.Source == "process" {
			s.State = "running"
			s.Reason = "process alive; session activity unavailable"
			s.ReasonCode = "process_alive"
			continue
		}
		if (s.PID == 0 || !alive[s.PID]) && now.Sub(s.At) > 30*time.Minute {
			s.PID = 0
			s.CPU = 0
			s.RSS = 0
			s.State = "ended"
			s.Reason = "no process matched; no activity for over 30 minutes"
			s.ReasonCode = "inactive_ended"
			continue
		}
		if limited[s.Kind] {
			s.State = "rate-limited"
			s.Reason = "quota reached"
			s.ReasonCode = "quota_reached"
			continue
		}
		if s.State == "erroring" || s.State == "rate-limited" {
			continue
		}
		if s.PID != 0 && alive[s.PID] && s.LastTurnEnded && now.Sub(s.At) > 5*time.Minute {
			s.State = "waiting-for-input"
			s.Reason = "process alive, no recent activity"
			s.ReasonCode = "awaiting_input"
		} else if (s.PID == 0 || !alive[s.PID]) && s.LastTurnEnded && now.Sub(s.At) >= 2*time.Minute {
			s.PID = 0
			s.CPU = 0
			s.RSS = 0
			s.State = "ended"
			s.Reason = "last turn ended; no process matched"
			s.ReasonCode = "turn_ended"
		} else if now.Sub(s.At) < 2*time.Minute {
			s.State = "working"
			s.Reason = "recent activity"
			s.ReasonCode = "recent_activity"
		} else {
			s.State = "unknown"
			s.Reason = "no recent activity"
			s.ReasonCode = "inactive_unknown"
		}
	}
}
func StaleWorktrees(roots []string, processes []Process, now time.Time, age time.Duration) []model.AgentSession {
	out := []model.AgentSession{}
	if age <= 0 {
		age = 72 * time.Hour
	}
	for _, root := range roots {
		if !safePath(root) {
			continue
		}
		projects, e := os.ReadDir(root)
		if e != nil {
			continue
		}
		for _, project := range projects {
			if !project.IsDir() || project.Type()&os.ModeSymlink != 0 {
				continue
			}
			path := filepath.Join(root, project.Name())
			trees, e := os.ReadDir(path)
			if e != nil {
				continue
			}
			for _, tree := range trees {
				if !tree.IsDir() || tree.Type()&os.ModeSymlink != 0 {
					continue
				}
				worktree := filepath.Join(path, tree.Name())
				info, e := tree.Info()
				if e != nil || now.Sub(info.ModTime()) < age {
					continue
				}
				occupied := false
				for _, p := range processes {
					if p.CWD == worktree || strings.HasPrefix(p.CWD, worktree+string(filepath.Separator)) {
						occupied = true
						break
					}
				}
				if occupied {
					continue
				}
				if !unchangedSince(worktree, now.Add(-age)) {
					continue
				}
				out = append(out, model.AgentSession{ID: Hash(worktree), Kind: "unknown", Project: project.Name(), Worktree: tree.Name(), At: info.ModTime().UTC(), State: "stale-worktree", Reason: "no agent process and no directory change for 3 days", ReasonCode: "stale_worktree", Source: "worktree"})
			}
		}
	}
	return out
}

var changed = errors.New("changed or scan limit reached")

func unchangedSince(root string, cutoff time.Time) bool {
	count := 0
	deadline := time.Now().Add(200 * time.Millisecond)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return changed
		}
		count++
		if count > 10000 || time.Now().After(deadline) {
			return changed
		}
		info, e := d.Info()
		if e != nil || info.ModTime().After(cutoff) {
			return changed
		}
		return nil
	})
	return err == nil
}
