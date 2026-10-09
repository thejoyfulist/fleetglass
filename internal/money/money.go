package money

import (
	"context"
	"database/sql"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"
)

const ECBURL = "https://www.ecb.europa.eu/stats/eurofxref/eurofxref-daily.xml"

// Snapshot uses units of currency per EUR, as published by the ECB.
type Snapshot struct {
	Date   string             `json:"date"`
	Source string             `json:"source"`
	Rates  map[string]float64 `json:"rates"`
}

type Amount struct {
	Original        float64  `json:"original"`
	Currency        string   `json:"currency"`
	Converted       *float64 `json:"converted"`
	DisplayCurrency string   `json:"display_currency"`
	Rate            *float64 `json:"rate,omitempty"`
	RateSource      string   `json:"rate_source,omitempty"`
	RateDate        string   `json:"rate_date,omitempty"`
}

type Service struct {
	DB      *sql.DB
	Static  *Snapshot
	Source  string
	URL     string // Tests may use a local fixture server.
	Client  *http.Client
	Now     func() time.Time // Tests control the UTC check day.
	mu      sync.Mutex
	checked string
}

func (s *Service) Init() error {
	_, err := s.DB.Exec(`CREATE TABLE IF NOT EXISTS exchange_rates (source TEXT PRIMARY KEY, date TEXT NOT NULL, data BLOB NOT NULL)`)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`CREATE TABLE IF NOT EXISTS exchange_rate_checks (source TEXT PRIMARY KEY, day TEXT NOT NULL)`)
	return err
}

func (s *Service) snapshots() []*Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	today := now().UTC().Format("2006-01-02")
	if s.Source == "ecb" && s.checked != today {
		_ = s.DB.QueryRow(`SELECT day FROM exchange_rate_checks WHERE source='ecb'`).Scan(&s.checked)
	}
	if s.Source == "ecb" && s.checked != today {
		s.checked = today
		_, _ = s.DB.Exec(`INSERT OR REPLACE INTO exchange_rate_checks(source,day) VALUES('ecb',?)`, today)
		client := s.Client
		if client == nil {
			client = &http.Client{Timeout: 4 * time.Second}
		}
		url := s.URL
		if url == "" {
			url = ECBURL
		}
		fetchContext, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		request, err := http.NewRequestWithContext(fetchContext, http.MethodGet, url, nil)
		if err == nil {
			response, e := client.Do(request)
			if e == nil {
				if response.StatusCode == http.StatusOK {
					body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
					if readErr == nil {
						if snap, parseErr := ParseECB(body); parseErr == nil {
							var lastDate string
							_ = s.DB.QueryRow(`SELECT date FROM exchange_rates WHERE source='ecb'`).Scan(&lastDate)
							if snap.Date >= lastDate {
								_, _ = s.DB.Exec(`INSERT OR REPLACE INTO exchange_rates(source,date,data) VALUES(?,?,?)`, "ecb", snap.Date, body)
							}
						}
					}
				}
				response.Body.Close()
			}
		}
	}
	var cached *Snapshot
	if s.Source == "ecb" {
		var body []byte
		if s.DB.QueryRow(`SELECT data FROM exchange_rates WHERE source='ecb'`).Scan(&body) == nil {
			cached, _ = ParseECB(body)
		}
	}
	if cached == nil && s.Static == nil {
		return nil
	}
	if cached == nil {
		return []*Snapshot{s.Static}
	}
	if s.Static == nil {
		return []*Snapshot{cached}
	}
	if s.Static.Date > cached.Date {
		return []*Snapshot{s.Static, cached}
	}
	return []*Snapshot{cached, s.Static}
}

func (s *Service) Convert(ctx context.Context, value float64, from, to string) Amount {
	// A canceled browser request must not prevent today's cache refresh.
	_ = ctx
	out := Amount{Original: value, Currency: from, DisplayCurrency: to}
	if from == to {
		out.Converted = &value
		return out
	}
	for _, snap := range s.snapshots() {
		a, b := snap.Rates[from], snap.Rates[to]
		if from == "EUR" {
			a = 1
		}
		if to == "EUR" {
			b = 1
		}
		if a <= 0 || b <= 0 {
			continue
		}
		rate := b / a
		converted := value * rate
		out.Converted, out.Rate = &converted, &rate
		out.RateDate, out.RateSource = snap.Date, snap.Source
		return out
	}
	return out
}

func ParseECB(data []byte) (*Snapshot, error) {
	var feed struct {
		Cubes []struct {
			Time  string `xml:"time,attr"`
			Rates []struct {
				Currency string  `xml:"currency,attr"`
				Rate     float64 `xml:"rate,attr"`
			} `xml:"Cube"`
		} `xml:"Cube>Cube"`
	}
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}
	for _, day := range feed.Cubes {
		if _, err := time.Parse("2006-01-02", day.Time); err != nil {
			continue
		}
		rates := map[string]float64{"EUR": 1}
		for _, row := range day.Rates {
			if !ValidCurrency(row.Currency) || row.Rate <= 0 || math.IsInf(row.Rate, 0) || math.IsNaN(row.Rate) {
				return nil, errors.New("invalid ECB rate")
			}
			rates[row.Currency] = row.Rate
		}
		if len(rates) > 1 {
			return &Snapshot{Date: day.Time, Source: "ecb", Rates: rates}, nil
		}
	}
	return nil, fmt.Errorf("ECB response has no dated rates")
}
