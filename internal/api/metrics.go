package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

// metrics exposes the same read access boundary as the private API.
func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
	hosts, err := a.Store.Hosts()
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	fmt.Fprintln(w, "# HELP fleetglass_host_live Host reported within the last 45 seconds.\n# TYPE fleetglass_host_live gauge")
	fmt.Fprintln(w, "# HELP fleetglass_host_metric Host resource measurement.\n# TYPE fleetglass_host_metric gauge")
	fmt.Fprintln(w, "# HELP fleetglass_probe_state Probe state, 1 when active.\n# TYPE fleetglass_probe_state gauge")
	fmt.Fprintln(w, "# HELP fleetglass_app_metric App resource measurement.\n# TYPE fleetglass_app_metric gauge")
	fmt.Fprintln(w, "# HELP fleetglass_ai_tokens AI tokens observed in the last hour.\n# TYPE fleetglass_ai_tokens gauge")
	for _, host := range hosts {
		id := promLabel(host.HostID)
		live := 0
		if host.Liveness == "live" {
			live = 1
		}
		fmt.Fprintf(w, "fleetglass_host_live{host=%s} %d\n", id, live)
		o, err := a.Store.Overview(host.HostID)
		if err != nil {
			continue
		}
		for _, result := range o.Results {
			for _, state := range []model.State{model.OK, model.Warn, model.Crit, model.Unknown, model.Stale} {
				active := 0
				if result.State == state {
					active = 1
				}
				fmt.Fprintf(w, "fleetglass_probe_state{host=%s,probe=%s,state=%s} %d\n", id, promLabel(result.Name), promLabel(string(state)), active)
			}
			var name, field string
			switch result.Name {
			case "cpu":
				name, field = "cpu", "busy_percent"
			case "memory":
				name, field = "memory", "used_bytes"
			}
			if value, ok := result.Values[field]; name != "" && ok && result.State != model.Stale {
				fmt.Fprintf(w, "fleetglass_host_metric{host=%s,metric=%s} %s\n", id, promLabel(name), strconv.FormatFloat(value, 'f', -1, 64))
			}
		}
		apps, err := a.Store.Apps(host.HostID, "1h", time.Now().UTC())
		if err == nil {
			for _, app := range apps.Apps {
				labels := fmt.Sprintf("host=%s,app=%s,kind=%s", id, promLabel(app.Name), promLabel(app.Kind))
				fmt.Fprintf(w, "fleetglass_app_metric{%s,metric=\"cpu_percent\"} %s\n", labels, strconv.FormatFloat(app.CPUPercent, 'f', -1, 64))
				fmt.Fprintf(w, "fleetglass_app_metric{%s,metric=\"memory_bytes\"} %s\n", labels, strconv.FormatFloat(app.MemoryBytes, 'f', -1, 64))
			}
		}
		usage, err := a.Store.Usage(host.HostID, "1h", "agent")
		if err == nil {
			for _, group := range usage {
				name, _ := group["name"].(string)
				tokens, _ := group["tokens"].(model.Tokens)
				for _, item := range []struct {
					name  string
					value int64
				}{{"input", tokens.Input}, {"output", tokens.Output}, {"cache_write", tokens.CacheWrite}, {"cache_read", tokens.CacheRead}} {
					fmt.Fprintf(w, "fleetglass_ai_tokens{host=%s,agent=%s,type=%s} %d\n", id, promLabel(name), promLabel(item.name), item.value)
				}
			}
		}
	}
}

func promLabel(value string) string {
	value = strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' || r == 127 {
			return ' '
		}
		return r
	}, value)
	return "\"" + strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\"", "\\\"").Replace(value) + "\""
}
