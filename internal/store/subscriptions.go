package store

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/model"
)

type SubscriptionComparison struct {
	Kind                 string   `json:"kind"`
	Month                string   `json:"month"`
	APIEquivalentUSD     float64  `json:"api_equivalent_usd"`
	SubscriptionUSD      *float64 `json:"subscription_usd"`
	SubscriptionAmount   *float64 `json:"subscription_amount"`
	SubscriptionCurrency string   `json:"subscription_currency,omitempty"`
	Coverage             string   `json:"coverage"`
	UnpricedTokens       int64    `json:"unpriced_tokens"`
}

func (s *Store) SubscriptionComparisons(host string, now time.Time, overrides map[string]model.AIPrice, subscriptions []model.AISubscription) ([]SubscriptionComparison, error) {
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	rows, err := s.DB.Query(`SELECT data FROM ai_usage WHERE host_id=? AND at_ns>=? UNION ALL SELECT data FROM ai_usage_hourly WHERE host_id=? AND bucket_ns>=? UNION ALL SELECT data FROM ai_usage_daily WHERE host_id=? AND bucket_ns>=?`, host, start.UnixNano(), host, start.UnixNano(), host, start.UnixNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	prices := effectivePrices(overrides)
	groups := map[string]*SubscriptionComparison{}
	for _, kind := range []string{"claude", "codex"} {
		groups[kind] = &SubscriptionComparison{Kind: kind, Month: start.Format("2006-01"), Coverage: "complete"}
	}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var usage model.AIUsage
		if json.Unmarshal([]byte(raw), &usage) != nil || usage.Model == "<synthetic>" {
			continue
		}
		group := groups[usage.Kind]
		if group == nil {
			continue
		}
		if price, ok := modelPrice(prices, usage.Model); ok {
			group.APIEquivalentUSD += (float64(usage.Tokens.Input)*price.Input + float64(usage.Tokens.Output)*price.Output + float64(usage.Tokens.CacheWrite)*price.CacheWrite + float64(usage.Tokens.CacheRead)*price.CacheRead) / 1e6
		} else {
			group.UnpricedTokens += usage.Tokens.Input + usage.Tokens.Output + usage.Tokens.CacheWrite + usage.Tokens.CacheRead
			group.Coverage = "partial"
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for _, sub := range subscriptions {
		if group := groups[sub.Kind]; group != nil {
			value, currency := sub.Amount()
			if value <= 0 {
				continue
			}
			group.SubscriptionAmount = &value
			group.SubscriptionCurrency = currency
			if currency == "USD" {
				group.SubscriptionUSD = &value
			}
		}
	}
	out := make([]SubscriptionComparison, 0, len(groups))
	for _, value := range groups {
		out = append(out, *value)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind < out[j].Kind })
	return out, nil
}
