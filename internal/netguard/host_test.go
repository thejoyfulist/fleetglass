package netguard

import "testing"

func TestLoopbackAndConfiguredHosts(t *testing.T) {
	for _, host := range []string{"localhost", "localhost:7878", "127.0.0.1", "127.0.0.1:7878", "::1", "[::1]", "[::1]:7878"} {
		if !AllowedHost(host, nil) {
			t.Errorf("rejected loopback host %q", host)
		}
	}
	for _, host := range []string{"attacker.example", "attacker.example:7878", "localhost:bad", "127.0.0.1:bad", "https://localhost"} {
		if AllowedHost(host, nil) {
			t.Errorf("accepted unauthorized host %q", host)
		}
	}
	if !AllowedHost("cockpit.example:8444", []string{"cockpit.example"}) || !AllowedHost("cockpit.example:8444", []string{"cockpit.example:8444"}) {
		t.Fatal("configured host rejected")
	}
	if AllowedHost("cockpit.example:7878", []string{"cockpit.example:8444"}) {
		t.Fatal("configured port ignored")
	}
}
