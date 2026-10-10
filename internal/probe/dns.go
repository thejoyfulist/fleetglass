package probe

import (
	"context"
	"net"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type DNSResolver interface {
	LookupHost(context.Context, string) ([]string, error)
}

func DNS(ctx context.Context, names []string, resolver DNSResolver) []model.Result {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	out := make([]model.Result, 0, len(names))
	for _, name := range names {
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		start := time.Now()
		addresses, err := resolver.LookupHost(probeCtx, name)
		elapsed := time.Since(start).Seconds() * 1000
		cancel()
		state := model.OK
		if err != nil || len(addresses) == 0 || elapsed >= 1000 {
			state = model.Crit
		} else if elapsed >= 250 {
			state = model.Warn
		}
		detail := "resolved"
		if err != nil || len(addresses) == 0 {
			detail = "resolution failed"
		}
		out = append(out, model.Result{Name: "dns:" + name, State: state, Detail: detail, Values: map[string]float64{"latency_ms": elapsed}, Fields: map[string]string{"name": name}})
	}
	return out
}
