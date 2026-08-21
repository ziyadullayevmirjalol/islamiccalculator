package handler

import (
	"context"
	"net/http"

	"github.com/diyorbek/islamiccalculator/internal/pkg/httpx"
	"github.com/diyorbek/islamiccalculator/internal/service"
)

type RatesService interface {
	Metals(ctx context.Context) (service.MetalsOutput, error)
}

type Rates struct {
	svc RatesService
}

func NewRates(svc RatesService) *Rates {
	return &Rates{svc: svc}
}

type metalsResponse struct {
	Gold     metalPriceDTO `json:"gold"`
	Silver   metalPriceDTO `json:"silver"`
	Nisab    nisabDTO      `json:"nisab"`
	Currency string        `json:"currency"`
}

func (h *Rates) Metals(w http.ResponseWriter, r *http.Request) {
	out, err := h.svc.Metals(r.Context())
	if err != nil {
		httpx.Err(w, err)
		return
	}
	httpx.Data(w, http.StatusOK, metalsResponse{
		Gold:   toMetalPriceDTO(out.Gold),
		Silver: toMetalPriceDTO(out.Silver),
		Nisab: nisabDTO{
			GoldValue:   out.Nisab.GoldValue.StringFixed(2),
			SilverValue: out.Nisab.SilverValue.StringFixed(2),
			Applied:     out.Nisab.Applied.StringFixed(2),
			Basis:       out.Nisab.Basis,
		},
		Currency: out.Currency,
	})
}
