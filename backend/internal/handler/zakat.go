package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/zakat"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
	"github.com/diyorbek/islamiccalculator/internal/pkg/httpx"
	"github.com/diyorbek/islamiccalculator/internal/service"
)

type ZakatService interface {
	Wealth(ctx context.Context, in zakat.WealthInput) (service.WealthOutput, error)
	Business(ctx context.Context, in zakat.BusinessInput) (zakat.BusinessResult, error)
	Ushr(ctx context.Context, in zakat.UshrInput) (zakat.UshrResult, error)
	Livestock(ctx context.Context, in zakat.LivestockInput) (zakat.LivestockResult, error)
	Fidya(ctx context.Context, in zakat.FidyaInput) (zakat.FidyaResult, error)
	Tazkiya(ctx context.Context, in zakat.TazkiyaInput) (zakat.TazkiyaResult, error)
}

type Zakat struct {
	svc ZakatService
}

func NewZakat(svc ZakatService) *Zakat {
	return &Zakat{svc: svc}
}

type wealthRequest struct {
	GoldGrams    string `json:"goldGrams"`
	SilverGrams  string `json:"silverGrams"`
	Cash         string `json:"cash"`
	OtherAssets  string `json:"otherAssets"`
	HawlComplete bool   `json:"hawlComplete"`
}

type nisabDTO struct {
	GoldValue   string `json:"goldValue"`
	SilverValue string `json:"silverValue"`
	Applied     string `json:"applied"`
	Basis       string `json:"basis"`
}

type metalPriceDTO struct {
	PricePerGram string `json:"pricePerGram"`
	Currency     string `json:"currency"`
	Source       string `json:"source"`
	FetchedAt    string `json:"fetchedAt"`
	Stale        bool   `json:"stale"`
}

type wealthResponse struct {
	GoldValue    string        `json:"goldValue"`
	SilverValue  string        `json:"silverValue"`
	TotalWealth  string        `json:"totalWealth"`
	Nisab        nisabDTO      `json:"nisab"`
	AboveNisab   bool          `json:"aboveNisab"`
	HawlComplete bool          `json:"hawlComplete"`
	ZakatDue     string        `json:"zakatDue"`
	Currency     string        `json:"currency"`
	Prices       struct {
		Gold   metalPriceDTO `json:"gold"`
		Silver metalPriceDTO `json:"silver"`
	} `json:"prices"`
}

func (h *Zakat) Wealth(w http.ResponseWriter, r *http.Request) {
	var req wealthRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := zakat.WealthInput{HawlComplete: req.HawlComplete}
	in.GoldGrams = parseOptionalAmount(req.GoldGrams, "goldGrams", fields)
	in.SilverGrams = parseOptionalAmount(req.SilverGrams, "silverGrams", fields)
	in.Cash = parseOptionalAmount(req.Cash, "cash", fields)
	in.OtherAssets = parseOptionalAmount(req.OtherAssets, "otherAssets", fields)
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid zakat request", fields))
		return
	}

	out, err := h.svc.Wealth(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	res := out.Result
	resp := wealthResponse{
		GoldValue:   res.GoldValue.StringFixed(2),
		SilverValue: res.SilverValue.StringFixed(2),
		TotalWealth: res.TotalWealth.StringFixed(2),
		Nisab: nisabDTO{
			GoldValue:   res.Nisab.GoldValue.StringFixed(2),
			SilverValue: res.Nisab.SilverValue.StringFixed(2),
			Applied:     res.Nisab.Applied.StringFixed(2),
			Basis:       res.Nisab.Basis,
		},
		AboveNisab:   res.AboveNisab,
		HawlComplete: res.HawlComplete,
		ZakatDue:     res.ZakatDue.StringFixed(2),
		Currency:     res.Currency,
	}
	resp.Prices.Gold = toMetalPriceDTO(out.Gold)
	resp.Prices.Silver = toMetalPriceDTO(out.Silver)
	httpx.Data(w, http.StatusOK, resp)
}

func toMetalPriceDTO(p service.MetalPrice) metalPriceDTO {
	return metalPriceDTO{
		PricePerGram: p.PricePerGram.StringFixed(2),
		Currency:     p.Currency,
		Source:       p.Source,
		FetchedAt:    p.FetchedAt.UTC().Format(time.RFC3339),
		Stale:        p.Stale,
	}
}

// --- business zakat --------------------------------------------------------

type businessRequest struct {
	Cash                 string `json:"cash"`
	Receivables          string `json:"receivables"`
	Inventory            string `json:"inventory"`
	ShortTermLiabilities string `json:"shortTermLiabilities"`
	HawlComplete         bool   `json:"hawlComplete"`
}

type businessResponse struct {
	ZakatBase    string   `json:"zakatBase"`
	Nisab        nisabDTO `json:"nisab"`
	AboveNisab   bool     `json:"aboveNisab"`
	HawlComplete bool     `json:"hawlComplete"`
	ZakatDue     string   `json:"zakatDue"`
	Currency     string   `json:"currency"`
}

func (h *Zakat) Business(w http.ResponseWriter, r *http.Request) {
	var req businessRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := zakat.BusinessInput{HawlComplete: req.HawlComplete}
	in.Cash = parseOptionalAmount(req.Cash, "cash", fields)
	in.Receivables = parseOptionalAmount(req.Receivables, "receivables", fields)
	in.Inventory = parseOptionalAmount(req.Inventory, "inventory", fields)
	in.ShortTermLiabilities = parseOptionalAmount(req.ShortTermLiabilities, "shortTermLiabilities", fields)
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid business zakat request", fields))
		return
	}

	res, err := h.svc.Business(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	httpx.Data(w, http.StatusOK, businessResponse{
		ZakatBase: res.ZakatBase.StringFixed(2),
		Nisab: nisabDTO{
			GoldValue:   res.Nisab.GoldValue.StringFixed(2),
			SilverValue: res.Nisab.SilverValue.StringFixed(2),
			Applied:     res.Nisab.Applied.StringFixed(2),
			Basis:       res.Nisab.Basis,
		},
		AboveNisab:   res.AboveNisab,
		HawlComplete: res.HawlComplete,
		ZakatDue:     res.ZakatDue.StringFixed(2),
		Currency:     res.Currency,
	})
}

// --- ushr ------------------------------------------------------------------

type ushrRequest struct {
	IrrigationType string `json:"irrigationType"`
	HarvestValue   string `json:"harvestValue"`
}

type ushrResponse struct {
	IrrigationType string `json:"irrigationType"`
	HarvestValue   string `json:"harvestValue"`
	Rate           string `json:"rate"`
	UshrDue        string `json:"ushrDue"`
}

func (h *Zakat) Ushr(w http.ResponseWriter, r *http.Request) {
	var req ushrRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := zakat.UshrInput{IrrigationType: req.IrrigationType}
	in.HarvestValue = parseAmount(req.HarvestValue, "harvestValue", fields)
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid ushr request", fields))
		return
	}

	res, err := h.svc.Ushr(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	httpx.Data(w, http.StatusOK, ushrResponse{
		IrrigationType: res.IrrigationType,
		HarvestValue:   res.HarvestValue.StringFixed(2),
		Rate:           res.Rate.StringFixed(2),
		UshrDue:        res.UshrDue.StringFixed(2),
	})
}

// --- livestock -------------------------------------------------------------

type livestockRequest struct {
	Species     string `json:"species"`
	Count       int    `json:"count"`
	MarketValue string `json:"marketValue"` // silk_cocoons only
}

type animalDueDTO struct {
	Animal string `json:"animal"`
	Count  int    `json:"count"`
}

type livestockResponse struct {
	Species    string         `json:"species"`
	Count      int            `json:"count,omitempty"`
	Due        []animalDueDTO `json:"due"`
	BelowNisab bool           `json:"belowNisab"`
	CashDue    string         `json:"cashDue,omitempty"`
	Rate       string         `json:"rate,omitempty"`
	Note       string         `json:"note,omitempty"`
}

func (h *Zakat) Livestock(w http.ResponseWriter, r *http.Request) {
	var req livestockRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := zakat.LivestockInput{Species: req.Species, Count: req.Count}
	if req.MarketValue != "" {
		in.MarketValue = parseAmount(req.MarketValue, "marketValue", fields)
	}
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid livestock request", fields))
		return
	}

	res, err := h.svc.Livestock(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	resp := livestockResponse{
		Species:    res.Species,
		Count:      res.Count,
		Due:        make([]animalDueDTO, len(res.Due)),
		BelowNisab: res.BelowNisab,
		Note:       res.Note,
	}
	for i, due := range res.Due {
		resp.Due[i] = animalDueDTO{Animal: due.Animal, Count: due.Count}
	}
	if !res.CashDue.IsZero() {
		resp.CashDue = res.CashDue.StringFixed(2)
		resp.Rate = res.Rate.StringFixed(4)
	}
	httpx.Data(w, http.StatusOK, resp)
}

// --- fidya / kaffarah ------------------------------------------------------

type fidyaRequest struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

type fidyaResponse struct {
	Kind        string `json:"kind"`
	Count       int    `json:"count"`
	Feedings    int    `json:"feedingsPerUnit"`
	DailyRate   string `json:"dailyRate"`
	TotalDue    string `json:"totalDue"`
	Currency    string `json:"currency"`
	NeedsReview bool   `json:"needsReview"`
}

func (h *Zakat) Fidya(w http.ResponseWriter, r *http.Request) {
	var req fidyaRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	res, err := h.svc.Fidya(r.Context(), zakat.FidyaInput{Kind: req.Kind, Count: req.Count})
	if err != nil {
		httpx.Err(w, err)
		return
	}

	httpx.Data(w, http.StatusOK, fidyaResponse{
		Kind:        res.Kind,
		Count:       res.Count,
		Feedings:    res.Feedings,
		DailyRate:   res.DailyRate.StringFixed(2),
		TotalDue:    res.TotalDue.StringFixed(2),
		Currency:    res.Currency,
		NeedsReview: res.NeedsReview,
	})
}

// --- tazkiya ---------------------------------------------------------------

type tazkiyaRequest struct {
	Mode           string `json:"mode"`
	TotalIncome    string `json:"totalIncome"`
	ImpureAmount   string `json:"impureAmount"`
	DividendAmount string `json:"dividendAmount"`
	ImpureRatio    string `json:"impureRatio"`
}

type tazkiyaResponse struct {
	Mode        string `json:"mode"`
	PurgeAmount string `json:"purgeAmount"`
	CleanAmount string `json:"cleanAmount"`
	Disposition string `json:"disposition"`
}

func (h *Zakat) Tazkiya(w http.ResponseWriter, r *http.Request) {
	var req tazkiyaRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := zakat.TazkiyaInput{Mode: req.Mode}
	in.TotalIncome = parseOptionalAmount(req.TotalIncome, "totalIncome", fields)
	in.ImpureAmount = parseOptionalAmount(req.ImpureAmount, "impureAmount", fields)
	in.DividendAmount = parseOptionalAmount(req.DividendAmount, "dividendAmount", fields)
	in.ImpureRatio = parseOptionalAmount(req.ImpureRatio, "impureRatio", fields)
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid tazkiya request", fields))
		return
	}

	res, err := h.svc.Tazkiya(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	httpx.Data(w, http.StatusOK, tazkiyaResponse{
		Mode:        res.Mode,
		PurgeAmount: res.PurgeAmount.StringFixed(2),
		CleanAmount: res.CleanAmount.StringFixed(2),
		Disposition: "charity",
	})
}

// parseOptionalAmount treats "" as zero — zakat inputs are a form where
// most people fill only some fields.
func parseOptionalAmount(s, field string, fields map[string]string) decimal.Decimal {
	if s == "" {
		return decimal.Zero
	}
	return parseAmount(s, field, fields)
}
