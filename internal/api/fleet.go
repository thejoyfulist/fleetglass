package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type fleetHost struct {
	HostID     string            `json:"host_id"`
	Labels     map[string]string `json:"labels,omitempty"`
	At         time.Time         `json:"at"`
	Liveness   string            `json:"liveness"`
	Normal     int               `json:"normal"`
	Checks     int               `json:"checks"`
	CPU        *float64          `json:"cpu,omitempty"`
	Memory     *float64          `json:"memory,omitempty"`
	Disk       *float64          `json:"disk,omitempty"`
	Agents     int               `json:"agents"`
	Worst      string            `json:"worst,omitempty"`
	WorstState string            `json:"worst_state,omitempty"`
	Usage      []map[string]any  `json:"usage"`
	Costs      []map[string]any  `json:"costs"`
}

func percent(r model.Result) *float64 {
	if r.State == model.Stale {
		return nil
	}
	if v, ok := r.Values["busy_percent"]; ok {
		return &v
	}
	used, u := r.Values["used_bytes"]
	total, t := r.Values["total_bytes"]
	if u && t && total > 0 {
		v := 100 * used / total
		return &v
	}
	return nil
}

func (a *API) fleet(w http.ResponseWriter, r *http.Request) {
	hosts, err := a.Store.Hosts()
	if err != nil {
		http.Error(w, "storage error", 500)
		return
	}
	out := make([]fleetHost, 0, len(hosts))
	for _, h := range hosts {
		o, err := a.Store.Overview(h.HostID)
		if err != nil {
			http.Error(w, "storage error", 500)
			return
		}
		row := fleetHost{HostID: h.HostID, Labels: h.Labels, At: h.At, Liveness: h.Liveness, Usage: []map[string]any{}, Costs: []map[string]any{}}
		for _, result := range o.Results {
			if result.Name == "cpu" {
				row.CPU = percent(result)
			}
			if result.Name == "memory" {
				row.Memory = percent(result)
			}
			if strings.HasPrefix(result.Name, "disk:") && row.Disk == nil {
				row.Disk = percent(result)
			}
			if result.State == model.Stale || result.State == model.Unknown {
				continue
			}
			row.Checks++
			if result.State == model.OK {
				row.Normal++
			} else if row.Worst == "" || result.State == model.Crit && row.WorstState != string(model.Crit) {
				row.Worst = result.Detail
				row.WorstState = string(result.State)
				if row.Worst == "" {
					row.Worst = result.Name
				}
			}
		}
		for _, session := range o.Agents {
			if session.State != "ended" && session.PID != 0 {
				row.Agents++
			}
		}
		row.Usage, err = a.Store.Usage(h.HostID, "24h", "project")
		if err != nil {
			http.Error(w, "storage error", 500)
			return
		}
		row.Costs, err = a.Store.Costs(h.HostID, "24h", a.AIPrices, a.AISubscriptions)
		if err != nil {
			http.Error(w, "storage error", 500)
			return
		}
		for _, cost := range row.Costs {
			a.decorateCost(r, cost)
		}
		out = append(out, row)
	}
	writeJSON(w, out)
}
