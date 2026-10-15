package forecast

import (
	"github.com/thejoyfulist/fleetglass/internal/model"
	"math"
	"sort"
	"time"
)

// Quota uses the first and last observations in the last quarter of a window.
// A reset boundary or fewer than two recent observations leaves the forecast unknown.
func Quota(current model.Quota, history []model.Quota, now time.Time) *model.QuotaForecast {
	if current.WindowMinutes <= 0 || !current.ResetsAt.After(now) {
		return nil
	}
	duration := time.Duration(current.WindowMinutes) * time.Minute
	start := current.ResetsAt.Add(-duration)
	recent := now.Add(-duration / 4)
	if recent.Before(start) {
		recent = start
	}
	values := make([]model.Quota, 0, len(history))
	for _, q := range history {
		if q.Method == current.Method && q.ResetsAt.Equal(current.ResetsAt) && !q.ObservedAt.Before(recent) && !q.ObservedAt.After(now) && q.Used >= 0 && q.Used <= 100 {
			values = append(values, q)
		}
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ObservedAt.Before(values[j].ObservedAt) })
	if len(values) < 2 {
		return nil
	}
	first, last := values[0], values[len(values)-1]
	hours := last.ObservedAt.Sub(first.ObservedAt).Hours()
	if hours <= 0 {
		return nil
	}
	pace := math.Max(0, (last.Used-first.Used)/hours)
	if len(values) >= 3 {
		previous := values[len(values)-2]
		if recentHours := last.ObservedAt.Sub(previous.ObservedAt).Hours(); recentHours > 0 {
			pace = (pace + math.Max(0, (last.Used-previous.Used)/recentHours)) / 2
		}
	}
	even := 100 / duration.Hours()
	f := &model.QuotaForecast{PacePPH: pace, EvenRatio: pace / even, PercentAtReset: math.Min(100, last.Used+pace*current.ResetsAt.Sub(last.ObservedAt).Hours()), Daily: []model.QuotaDay{}}
	if last.Used >= 100 {
		f.RunsOutAt = last.ObservedAt
		for _, observed := range values {
			if observed.Used >= 100 {
				f.RunsOutAt = observed.ObservedAt
				break
			}
		}
		f.LockedHours = current.ResetsAt.Sub(f.RunsOutAt).Hours()
	} else if pace > 0 {
		run := last.ObservedAt.Add(time.Duration((100 - last.Used) / pace * float64(time.Hour)))
		if run.Before(current.ResetsAt) {
			f.RunsOutAt = run
			f.LockedHours = current.ResetsAt.Sub(run).Hours()
		}
	}
	days := map[string]float64{}
	all := append([]model.Quota(nil), history...)
	sort.Slice(all, func(i, j int) bool { return all[i].ObservedAt.Before(all[j].ObservedAt) })
	for i := 1; i < len(all); i++ {
		a, b := all[i-1], all[i]
		if a.Method != current.Method || b.Method != current.Method || !a.ResetsAt.Equal(current.ResetsAt) || !b.ResetsAt.Equal(current.ResetsAt) || b.ObservedAt.Before(start) {
			continue
		}
		day := b.ObservedAt.UTC().Format("2006-01-02")
		days[day] += math.Max(0, b.Used-a.Used)
	}
	keys := make([]string, 0, len(days))
	for k := range days {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, day := range keys {
		f.Daily = append(f.Daily, model.QuotaDay{Day: day, UsedPP: days[day], AllowedPP: 100 / duration.Hours() * 24})
	}
	return f
}
