package money_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thejoyfulist/fleetglass/internal/money"
	"github.com/thejoyfulist/fleetglass/internal/store"
)

const fixture = `<?xml version="1.0" encoding="UTF-8"?><gesmes:Envelope xmlns:gesmes="http://www.gesmes.org/xml/2002-08-01" xmlns="http://www.ecb.int/vocabulary/2002-08-01/eurofxref"><Cube><Cube time="2026-10-08"><Cube currency="USD" rate="1.20"/><Cube currency="GBP" rate="0.80"/></Cube></Cube></gesmes:Envelope>`

func TestConversionAndMissingRate(t *testing.T) {
	s := &money.Service{Static: &money.Snapshot{Date: "2026-10-08", Source: "static", Rates: map[string]float64{"USD": 1.20, "GBP": 0.80}}}
	got := s.Convert(context.Background(), 120, "USD", "EUR")
	if got.Converted == nil || *got.Converted != 100 || got.RateSource != "static" || got.RateDate != "2026-10-08" {
		t.Fatalf("unexpected conversion: %+v", got)
	}
	missing := s.Convert(context.Background(), 120, "USD", "JPY")
	if missing.Converted != nil || missing.Original != 120 || missing.Currency != "USD" {
		t.Fatalf("missing rate must retain original: %+v", missing)
	}
}

func TestECBCacheFallback(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.Write([]byte(fixture)) }))
	s, err := store.Open(t.TempDir() + "/rates.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.DB.Close()
	firstDay := func() time.Time { return time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) }
	fx := &money.Service{DB: s.DB, Source: "ecb", URL: server.URL, Now: firstDay}
	if err := fx.Init(); err != nil {
		t.Fatal(err)
	}
	first := fx.Convert(context.Background(), 120, "USD", "EUR")
	if first.Converted == nil || *first.Converted != 100 || first.RateSource != "ecb" {
		t.Fatalf("unexpected ECB conversion: %+v", first)
	}
	fx.Convert(context.Background(), 120, "USD", "EUR")
	if requests.Load() != 1 {
		t.Fatalf("daily feed fetched %d times", requests.Load())
	}
	fx.Static = &money.Snapshot{Date: "2026-10-07", Source: "static", Rates: map[string]float64{"JPY": 150}}
	fallbackCurrency := fx.Convert(context.Background(), 150, "JPY", "EUR")
	if fallbackCurrency.Converted == nil || *fallbackCurrency.Converted != 1 || fallbackCurrency.RateSource != "static" {
		t.Fatalf("static fallback failed: %+v", fallbackCurrency)
	}
	server.Close()
	followingDay := func() time.Time { return time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC) }
	fx = &money.Service{DB: s.DB, Source: "ecb", URL: server.URL, Now: followingDay}
	last := fx.Convert(context.Background(), 120, "USD", "EUR")
	if last.Converted == nil || *last.Converted != 100 || last.RateDate != "2026-10-08" {
		t.Fatalf("cache fallback failed: %+v", last)
	}
}
