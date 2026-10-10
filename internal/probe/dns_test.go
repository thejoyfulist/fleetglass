package probe

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type resolverStub struct {
	addresses []string
	err       error
	delay     time.Duration
}

func (r resolverStub) LookupHost(ctx context.Context, _ string) ([]string, error) {
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return r.addresses, r.err
}
func TestDNSProbeStates(t *testing.T) {
	for _, tc := range []struct {
		resolver resolverStub
		state    model.State
	}{
		{resolverStub{addresses: []string{"192.0.2.1"}}, model.OK},
		{resolverStub{addresses: []string{"192.0.2.1"}, delay: 260 * time.Millisecond}, model.Warn},
		{resolverStub{err: errors.New("lookup failed")}, model.Crit},
		{resolverStub{}, model.Crit},
	} {
		got := DNS(context.Background(), []string{"demo.example"}, tc.resolver)
		if len(got) != 1 || got[0].State != tc.state || got[0].Name != "dns:demo.example" {
			t.Fatalf("DNS: %+v", got)
		}
	}
	if got := DNS(context.Background(), nil, resolverStub{}); len(got) != 0 {
		t.Fatalf("disabled DNS: %+v", got)
	}
}
