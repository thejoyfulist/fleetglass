package ai

import (
	"fmt"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

// AssessActivity uses observed tool, token, and cached repository activity.
// A flag suggests review; it is not proof of a loop.
func AssessActivity(s *model.AgentSession, now time.Time, threshold int) {
	s.Flag = false
	s.MinutesSinceFileEdit = nil
	if threshold <= 0 {
		threshold = 20
	}
	base := s.LastFileEditAt
	trackingOnly := (base.IsZero() || s.FirstToolAt.After(base)) && s.FirstToolAt.After(s.StartedAt)
	if base.IsZero() {
		base = s.StartedAt
	}
	if s.FirstToolAt.After(base) {
		base = s.FirstToolAt
	}
	if !base.IsZero() {
		minutes := max(0, int(now.Sub(base).Minutes()))
		s.MinutesSinceFileEdit = &minutes
	}
	if s.LastToolAt.IsZero() {
		s.Activity = "unknown"
		return
	}
	if s.State == "ended" || s.State == "idle" || s.State == "waiting-for-input" || now.Sub(s.At) > 2*time.Minute {
		s.Activity = "idle"
		return
	}
	if s.FirstToolAt.IsZero() && s.LastFileEditAt.IsZero() {
		s.MinutesSinceFileEdit = nil
		s.Activity = "working"
		return
	}
	if !trackingOnly && !s.LastFileEditAt.IsZero() && s.MinutesSinceFileEdit != nil && *s.MinutesSinceFileEdit < threshold {
		s.Activity = "editing"
		return
	}
	if base.IsZero() {
		s.Activity = "unknown"
		return
	}
	minutes := max(0, int(now.Sub(base).Minutes()))
	if minutes >= threshold && s.SpeedTokensPerSecond != nil && *s.SpeedTokensPerSecond > 0 && !s.LastUsageAt.IsZero() && now.Sub(s.LastUsageAt) <= 2*time.Minute {
		if trackingOnly {
			s.Activity = fmt.Sprintf("working, no file changes since tracking began %dm ago (possibly looping)", minutes)
		} else {
			s.Activity = fmt.Sprintf("working, no file changes for %dm (possibly looping)", minutes)
		}
		s.Flag = true
		return
	}
	if trackingOnly {
		s.Activity = fmt.Sprintf("working, no file changes since tracking began %dm ago", minutes)
	} else {
		s.Activity = "working"
	}
}
