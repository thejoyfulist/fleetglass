package api

import (
	"net/http"

	"github.com/thejoyfulist/fleetglass/internal/money"
)

func (a *API) targetCurrency(r *http.Request) string {
	code := r.URL.Query().Get("display_currency")
	if money.ValidCurrency(code) {
		return code
	}
	if a.DisplayCurrency != "" {
		return a.DisplayCurrency
	}
	return "USD"
}

func (a *API) convert(r *http.Request, amount float64, currency string) money.Amount {
	service := a.Money
	if service == nil {
		service = &money.Service{}
	}
	return service.Convert(r.Context(), amount, currency, a.targetCurrency(r))
}

func (a *API) decorateCost(r *http.Request, row map[string]any) {
	amount, _ := row["amount"].(float64)
	currency, _ := row["currency"].(string)
	if currency == "" {
		currency = "USD"
		amount, _ = row["usd"].(float64)
	}
	row["money"] = a.convert(r, amount, currency)
}
