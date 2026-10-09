package rapl

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Domain is one CPU package energy counter. IDs are powercap directory names.
type Domain struct {
	ID     string `json:"id"`
	Energy uint64 `json:"energy_uj"`
	Max    uint64 `json:"max_energy_range_uj"`
}

type Sample struct {
	Domains []Domain `json:"domains"`
}

var packageName = regexp.MustCompile(`^package-[0-9]+$`)
var domainDir = regexp.MustCompile(`^intel-rapl:[0-9]+$`)

func readUint(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
}

func Read(sysRoot string) (Sample, error) {
	dirs, err := os.ReadDir(filepath.Join(sysRoot, "class/powercap"))
	if err != nil {
		return Sample{}, err
	}
	out := Sample{Domains: []Domain{}}
	for _, entry := range dirs {
		if !domainDir.MatchString(entry.Name()) {
			continue
		}
		dir := filepath.Join(sysRoot, "class/powercap", entry.Name())
		name, err := os.ReadFile(filepath.Join(dir, "name"))
		if err != nil {
			return Sample{}, err
		}
		if !packageName.MatchString(strings.TrimSpace(string(name))) {
			continue
		}
		energy, err := readUint(filepath.Join(dir, "energy_uj"))
		if err != nil {
			return Sample{}, err
		}
		max, err := readUint(filepath.Join(dir, "max_energy_range_uj"))
		if err != nil {
			return Sample{}, err
		}
		if max == 0 || energy >= max {
			return Sample{}, fmt.Errorf("invalid RAPL counter")
		}
		out.Domains = append(out.Domains, Domain{ID: entry.Name(), Energy: energy, Max: max})
	}
	if len(out.Domains) == 0 {
		return Sample{}, fmt.Errorf("no CPU package RAPL domains")
	}
	return out, nil
}

type Tracker struct {
	previous map[string]Domain
	at       time.Time
}

// Power returns per-package watts and their sum once every current package has a valid delta.
func (t *Tracker) Power(sample Sample, now time.Time) (map[string]float64, bool) {
	watts, _, _, ok := t.Measure(sample, now)
	return watts, ok
}

// Measure returns the measured package energy interval as well as average watts.
// Long gaps and implausible deltas are discarded rather than charged as usage.
func (t *Tracker) Measure(sample Sample, now time.Time) (map[string]float64, float64, float64, bool) {
	current := make(map[string]Domain, len(sample.Domains))
	for _, d := range sample.Domains {
		current[d.ID] = d
	}
	watts := map[string]float64{}
	seconds := now.Sub(t.at).Seconds()
	valid := !t.at.IsZero() && seconds > 0 && seconds <= 600 && len(current) > 0
	energyKWh := 0.0
	if valid {
		for id, d := range current {
			old, ok := t.previous[id]
			if !ok || old.Max != d.Max || old.Energy >= old.Max || d.Energy >= d.Max {
				valid = false
				break
			}
			var delta uint64
			if d.Energy >= old.Energy {
				delta = d.Energy - old.Energy
			} else {
				delta = old.Max - old.Energy + d.Energy
			}
			watts[id] = float64(delta) / 1e6 / seconds
			if watts[id] > 1000 {
				valid = false
				break
			}
			energyKWh += float64(delta) / 3.6e12
		}
	}
	t.previous, t.at = current, now
	if !valid {
		return nil, 0, 0, false
	}
	return watts, energyKWh, seconds, true
}
