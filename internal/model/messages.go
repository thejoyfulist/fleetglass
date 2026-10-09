package model

import (
	"regexp"
	"strings"
)

var upgradedPackages = regexp.MustCompile(`^Upgraded ([0-9]+) packages: (.+)$`)
var technicalIdentifier = regexp.MustCompile(`^[a-z][a-z0-9]*(?:_[a-z0-9]+)+$`)

// AnnotateResult exposes stable presentation codes while retaining observed details.
func AnnotateResult(r *Result) {
	switch {
	case strings.HasPrefix(r.Name, "tls:") && hasCertificateDays(r.Values):
		if r.Values["days_remaining"] < 0 {
			r.DetailCode = "certificate_expired"
		} else {
			r.DetailCode = "certificate_expires"
		}
	case strings.HasPrefix(r.Name, "systemd:") && strings.HasPrefix(r.Detail, "active ("):
		r.DetailCode = "systemd_active"
		r.DetailParams = map[string]string{"substate": strings.TrimSuffix(strings.TrimPrefix(r.Detail, "active ("), ")")}
	case strings.HasPrefix(r.Name, "container:") && r.Fields["runtime_state"] == "running" && (r.Fields["health"] == "healthy" || r.Fields["health"] == "unhealthy" || r.Fields["health"] == "starting"):
		r.DetailCode = "container_status"
		r.DetailParams = map[string]string{"state": "running", "health": r.Fields["health"]}
	case strings.HasPrefix(r.Name, "container:") && strings.Contains(r.Detail, "; "):
		state, health, _ := strings.Cut(r.Detail, "; ")
		if state == "running" && (health == "healthy" || health == "unhealthy" || health == "starting") {
			r.DetailCode = "container_status"
			r.DetailParams = map[string]string{"state": state, "health": health}
		}
	case (strings.HasPrefix(r.Name, "systemd:") || strings.HasPrefix(r.Name, "container:")) && (r.Detail == "active" || r.Detail == "healthy"):
		r.DetailCode = r.Detail
	case r.Detail == "Observed":
		r.DetailCode = "observed"
	case strings.HasPrefix(r.Name, "status:") && r.Detail == "configured status value":
		r.DetailCode = "configured_status_value"
	case strings.HasPrefix(r.Name, "status:") && r.Detail == "recent timestamp":
		r.DetailCode = "recent_timestamp"
	case strings.HasPrefix(r.Name, "log_marker:") && r.Detail == "matching log entry observed":
		r.DetailCode = "log_entry_observed"
	case strings.HasPrefix(r.Name, "internal:") && r.Detail == "TCP connect succeeded":
		r.DetailCode = "tcp_connect_succeeded"
	}
}

func hasCertificateDays(values map[string]float64) bool {
	_, ok := values["days_remaining"]
	return ok
}

// AnnotateChange separates Fleetglass wording from observed names and packages.
func AnnotateChange(c *Change) {
	switch c.Type {
	case "APT":
		if parts := upgradedPackages.FindStringSubmatch(c.Text); parts != nil {
			c.ChangeCode = "apt_upgraded"
			c.ChangeParams = map[string]string{"count": parts[1], "packages": parts[2]}
		}
	case "Service":
		for _, suffix := range []string{"restarted", "failed"} {
			if name, ok := strings.CutSuffix(c.Text, " "+suffix); ok {
				c.ChangeCode = "service_" + suffix
				c.ChangeParams = map[string]string{"name": name}
				break
			}
		}
	case "Boot":
		if c.Text == "Machine restarted" {
			c.ChangeCode = "machine_restarted"
		}
	case "Container":
		if name, ok := strings.CutPrefix(c.Text, "Image changed: "); ok {
			c.ChangeCode = "container_image_changed"
			c.ChangeParams = map[string]string{"name": name}
		}
	case "Marker":
		c.ChangeCode = "marker"
		name := c.Text
		if technicalIdentifier.MatchString(name) {
			name = strings.ReplaceAll(name, "_", " ")
		}
		c.ChangeParams = map[string]string{"name": name}
	}
}
