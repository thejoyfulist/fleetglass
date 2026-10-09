package model

import "testing"

func TestPresentationMetadataPreservesObservedValues(t *testing.T) {
	r := Result{Name: "tls:example.test", Detail: "expires in 20.7 days", Values: map[string]float64{"days_remaining": 20.7}}
	AnnotateResult(&r)
	if r.DetailCode != "certificate_expires" || r.Values["days_remaining"] != 20.7 {
		t.Fatalf("certificate metadata: %+v", r)
	}
	for _, detail := range []string{"active", "healthy", "Observed"} {
		r = Result{Name: "container:local:web", Detail: detail}
		AnnotateResult(&r)
		if r.DetailCode == "" {
			t.Fatalf("missing code for %q", detail)
		}
	}
	r = Result{Name: "container:local:web", Detail: "user supplied status"}
	AnnotateResult(&r)
	if r.DetailCode != "" {
		t.Fatalf("observed text was classified: %+v", r)
	}
	r = Result{Name: "systemd:caddy", Detail: "active (running)"}
	AnnotateResult(&r)
	if r.DetailCode != "systemd_active" || r.DetailParams["substate"] != "running" {
		t.Fatalf("systemd state: %+v", r)
	}
	r = Result{Name: "container:local:web", Detail: "running; Up 2 hours (healthy)", Fields: map[string]string{"runtime_state": "running", "health": "healthy"}}
	AnnotateResult(&r)
	if r.DetailCode != "container_status" || r.DetailParams["health"] != "healthy" {
		t.Fatalf("container health: %+v", r)
	}
	r = Result{Name: "log_marker:backup_verified", Detail: "matching log entry observed"}
	AnnotateResult(&r)
	if r.DetailCode != "log_entry_observed" {
		t.Fatalf("marker detail: %+v", r)
	}

	c := Change{Type: "APT", Text: "Upgraded 3 packages: curl, openssl, git"}
	AnnotateChange(&c)
	if c.ChangeCode != "apt_upgraded" || c.ChangeParams["count"] != "3" || c.ChangeParams["packages"] != "curl, openssl, git" {
		t.Fatalf("package metadata: %+v", c)
	}
	c = Change{Type: "Service", Text: "caddy.service restarted"}
	AnnotateChange(&c)
	if c.ChangeCode != "service_restarted" || c.ChangeParams["name"] != "caddy.service" {
		t.Fatalf("service metadata: %+v", c)
	}
	c = Change{Type: "Marker", Text: "demo_guard_alert"}
	AnnotateChange(&c)
	if c.ChangeParams["name"] != "demo guard alert" {
		t.Fatalf("marker display name: %+v", c)
	}
}
