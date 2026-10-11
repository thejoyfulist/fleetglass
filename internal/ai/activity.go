package ai

import (
	"fmt"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

// AssessActivity uses observed tool calls and recent session activity only.
// A flag suggests review; it is not proof of a loop.
func AssessActivity(s *model.AgentSession, now time.Time, threshold int) {
	s.Flag = false
	s.MinutesSinceFileEdit = nil
	if threshold <= 0 {
		threshold = 20
	}
	if !s.LastFileEditAt.IsZero() {
		minutes := max(0, int(now.Sub(s.LastFileEditAt).Minutes()))
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
	if s.MinutesSinceFileEdit != nil && *s.MinutesSinceFileEdit < threshold {
		s.Activity = "editing"
		return
	}
	base := s.LastFileEditAt
	if base.IsZero() {
		base = s.StartedAt
	}
	if base.IsZero() {
		s.Activity = "unknown"
		return
	}
	minutes := max(0, int(now.Sub(base).Minutes()))
	if minutes >= threshold && (s.SpeedTokensPerSecond != nil && *s.SpeedTokensPerSecond > 0 && !s.LastUsageAt.IsZero() && now.Sub(s.LastUsageAt) <= 2*time.Minute || now.Sub(s.LastToolAt) <= 2*time.Minute) {
		s.Activity = fmt.Sprintf("working, no file changes for %dm (possibly looping)", minutes)
		s.Flag = true
		return
	}
	s.Activity = "working"
}
