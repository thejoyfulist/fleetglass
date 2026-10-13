package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentNoEditMinutes(t *testing.T) {
	c, err := Load("")
	if err != nil || c.AgentNoEditMinutes != 20 {
		t.Fatalf("default: %d %v", c.AgentNoEditMinutes, err)
	}
	for _, tc := range []struct {
		raw   string
		valid bool
		want  int
	}{
		{`{"agent_no_edit_minutes":1}`, true, 1},
		{`{"agent_no_edit_minutes":0}`, false, 0},
		{`{"agent_no_edit_minutes":1441}`, false, 0},
	} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(tc.raw), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if (err == nil) != tc.valid || err == nil && got.AgentNoEditMinutes != tc.want {
			t.Fatalf("%s: %+v %v", tc.raw, got, err)
		}
	}
}

func TestHTTPProbeConfigAndStatusListenerDefault(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.StatusListen != "" {
		t.Fatal("status listener must be off by default")
	}
	for _, v := range []struct {
		json  string
		valid bool
	}{
		{`{"http_probes":[{"name":"site","display_name":"Site","url":"https://example.test"}]}`, true},
		{`{"http_probes":[{"name":"site","display_name":"Site","url":"https://user:password@example.test"}]}`, false},
		{`{"http_probes":[{"name":"site","display_name":"Site","url":"http://example.test","interval_seconds":29}]}`, false},
		{`{"http_probes":[{"name":"site","display_name":"Site","url":"http://example.test","method":"HEAD","body_keyword":"ready"}]}`, false},
	} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(v.json), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if (err == nil) != v.valid {
			t.Fatalf("config accepted=%t: %v", v.valid, err)
		}
		if err == nil && (got.HTTPProbes[0].IntervalSeconds != 60 || got.HTTPProbes[0].TimeoutMS != 5000) {
			t.Fatalf("defaults: %+v", got.HTTPProbes[0])
		}
		if err != nil && strings.Contains(err.Error(), "password") {
			t.Fatal("secret appeared in validation error")
		}
	}
}

func TestDNSProbeNamesValidation(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		valid bool
	}{
		{`{"dns_probe_names":["demo.example","resolver.test"]}`, true},
		{`{"dns_probe_names":["http://demo.example"]}`, false},
		{`{"dns_probe_names":["192.0.2.1"]}`, false},
		{`{"dns_probe_names":["bad name"]}`, false},
	} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(tc.raw), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(path)
		if (err == nil) != tc.valid {
			t.Fatalf("configuration accepted=%t: %v", tc.valid, err)
		}
	}
}

func TestSubscriptionCurrenciesAndLegacyUSD(t *testing.T) {
	for _, tc := range []struct {
		data     string
		currency string
		amount   float64
		valid    bool
	}{
		{`{"ai_subscriptions":[{"kind":"claude","monthly_usd":20}]}`, "USD", 20, true},
		{`{"ai_subscriptions":[{"kind":"claude","monthly":150,"currency":"EUR"}]}`, "EUR", 150, true},
		{`{"ai_subscriptions":[{"kind":"claude","monthly":150,"currency":"EUR","monthly_usd":150}]}`, "", 0, false},
		{`{"ai_subscriptions":[{"kind":"claude","monthly":150,"currency":"eur"}]}`, "", 0, false},
		{`{"ai_subscriptions":[{"kind":"claude","monthly":150,"currency":"FOO"}]}`, "", 0, false},
	} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(tc.data), 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(path)
		if (err == nil) != tc.valid {
			t.Fatalf("config %s: %v", tc.data, err)
		}
		if err == nil {
			amount, currency := cfg.AISubscriptions[0].Amount()
			if amount != tc.amount || currency != tc.currency {
				t.Fatalf("amount %v %s", amount, currency)
			}
		}
	}
}
func TestMCPDisabledByDefaultAndRequiresPrivateToken(t *testing.T) {
	c, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if c.MCPListen != "" || c.MCPToken != "" {
		t.Fatal("MCP enabled by default")
	}
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte(strings.Repeat("x", 40)), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		listen string
		valid  bool
	}{{"127.0.0.1:17879", true}, {"0.0.0.0:17879", false}, {"localhost:17879", false}} {
		path := filepath.Join(dir, "config.json")
		body := `{"mcp_listen":"` + tc.listen + `","mcp_token_file":"` + tokenPath + `"}`
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := Load(path)
		if (err == nil) != tc.valid {
			t.Fatalf("listen %s: %v", tc.listen, err)
		}
		if err == nil && got.MCPToken != strings.Repeat("x", 40) {
			t.Fatal("token not loaded")
		}
	}
}

func TestReadAccessAddress(t *testing.T) {
	for _, c := range []struct {
		address, token string
		valid          bool
	}{
		{"127.0.0.1:7878", "", true}, {"[::1]:7878", "", true},
		{"0.0.0.0:7878", "", false}, {":7878", "", false},
		{"192.0.2.1:7878", "", false}, {"localhost:7878", "", false},
		{"0.0.0.0:7878", "read-secret", true},
	} {
		err := (Config{Listen: c.address, ReadToken: c.token}).ValidateReadAccess()
		if (err == nil) != c.valid {
			t.Fatalf("%q with token=%t: %v", c.address, c.token != "", err)
		}
	}
}

func TestAllowedHostsConfig(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		valid       bool
	}{
		{"bare name", `{"allowed_hosts":["cockpit.example"]}`, true},
		{"name and port", `{"allowed_hosts":["cockpit.example:8444"]}`, true},
		{"wildcard", `{"allowed_hosts":["*.example"]}`, false},
		{"URL", `{"allowed_hosts":["https://cockpit.example"]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(tc.value), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%t: %v", tc.valid, err)
			}
		})
	}
}

func TestRemoteAgentRequiresHTTPS(t *testing.T) {
	for _, tc := range []struct {
		url   string
		valid bool
	}{
		{"http://127.0.0.1:7878", true},
		{"http://remote.example:7878", false},
		{"https://remote.example", true},
	} {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(`{"server_url":"`+tc.url+`"}`), 0600); err != nil {
			t.Fatal(err)
		}
		_, err := Load(path)
		if (err == nil) != tc.valid {
			t.Fatalf("url %q valid=%t: %v", tc.url, tc.valid, err)
		}
	}
}
