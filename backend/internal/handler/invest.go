package handler

import (
	"context"
	"fmt"
	"net/http"

	"github.com/diyorbek/islamiccalculator/internal/domain/screener"
	"github.com/diyorbek/islamiccalculator/internal/domain/sukuk"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
	"github.com/diyorbek/islamiccalculator/internal/pkg/httpx"
)

type InvestService interface {
	Screen(ctx context.Context, in screener.Input) (screener.Result, error)
	Sukuk(ctx context.Context, positions []sukuk.Position) (sukuk.PortfolioResult, error)
}

type Invest struct {
	svc InvestService
}

func NewInvest(svc InvestService) *Invest {
	return &Invest{svc: svc}
}

// --- screener --------------------------------------------------------------

type screenerRequest struct {
	ProhibitedActivities       []string `json:"prohibitedActivities"`
	InterestBearingDebt        string   `json:"interestBearingDebt"`
	InterestBearingInvestments string   `json:"interestBearingInvestments"`
	MarketCap                  string   `json:"marketCap"`
	ImpureIncome               string   `json:"impureIncome"`
	TotalRevenue               string   `json:"totalRevenue"`
}

type ruleCheckDTO struct {
	Key       string `json:"key"`
	Ratio     string `json:"ratio"`
	Threshold string `json:"threshold"`
	Passed    bool   `json:"passed"`
}

type screenerResponse struct {
	Verdict           string         `json:"verdict"`
	ActivityPassed    bool           `json:"activityPassed"`
	FailedActivities  []string       `json:"failedActivities,omitempty"`
	Checks            []ruleCheckDTO `json:"checks"`
	PurificationRatio string         `json:"purificationRatio"`
}

func (h *Invest) Screener(w http.ResponseWriter, r *http.Request) {
	var req screenerRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := screener.Input{ProhibitedActivities: req.ProhibitedActivities}
	in.InterestBearingDebt = parseOptionalAmount(req.InterestBearingDebt, "interestBearingDebt", fields)
	in.InterestBearingInvestments = parseOptionalAmount(req.InterestBearingInvestments, "interestBearingInvestments", fields)
	in.MarketCap = parseAmount(req.MarketCap, "marketCap", fields)
	in.ImpureIncome = parseOptionalAmount(req.ImpureIncome, "impureIncome", fields)
	in.TotalRevenue = parseAmount(req.TotalRevenue, "totalRevenue", fields)
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid screener request", fields))
		return
	}

	res, err := h.svc.Screen(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	resp := screenerResponse{
		Verdict:           res.Verdict,
		ActivityPassed:    res.ActivityPassed,
		FailedActivities:  res.FailedActivities,
		Checks:            make([]ruleCheckDTO, len(res.Checks)),
		PurificationRatio: res.PurificationRatio.String(),
	}
	for i, c := range res.Checks {
		resp.Checks[i] = ruleCheckDTO{
			Key:       c.Key,
			Ratio:     c.Ratio.StringFixed(4),
			Threshold: c.Threshold.StringFixed(4),
			Passed:    c.Passed,
		}
	}
	httpx.Data(w, http.StatusOK, resp)
}

// --- sukuk -----------------------------------------------------------------

type sukukRequest struct {
	Positions []struct {
		Name                   string `json:"name"`
		FaceValue              string `json:"faceValue"`
		PurchasePrice          string `json:"purchasePrice"`
		DistributionRateAnnual string `json:"distributionRateAnnual"`
		Frequency              int    `json:"frequency"`
		TermMonths             int    `json:"termMonths"`
	} `json:"positions"`
}

type sukukPositionDTO struct {
	Name                       string `json:"name,omitempty"`
	FaceValue                  string `json:"faceValue"`
	PurchasePrice              string `json:"purchasePrice"`
	DistributionRateAnnual     string `json:"distributionRateAnnual"`
	Frequency                  int    `json:"frequency"`
	TermMonths                 int    `json:"termMonths"`
	Payments                   int    `json:"payments"`
	PeriodicDistribution       string `json:"periodicDistribution"`
	TotalExpectedDistributions string `json:"totalExpectedDistributions"`
	RedemptionAtFace           string `json:"redemptionAtFace"`
	ExpectedGain               string `json:"expectedGain"`
	CurrentYield               string `json:"currentYield"`
}

type sukukResponse struct {
	Positions             []sukukPositionDTO `json:"positions"`
	TotalInvested         string             `json:"totalInvested"`
	TotalFace             string             `json:"totalFace"`
	TotalAnnualIncome     string             `json:"totalAnnualIncome"`
	PortfolioCurrentYield string             `json:"portfolioCurrentYield"`
	TotalExpectedGain     string             `json:"totalExpectedGain"`
	Guaranteed            bool               `json:"guaranteed"`
}

func (h *Invest) Sukuk(w http.ResponseWriter, r *http.Request) {
	var req sukukRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	positions := make([]sukuk.Position, len(req.Positions))
	for i, p := range req.Positions {
		pos := sukuk.Position{Name: p.Name, Frequency: p.Frequency, TermMonths: p.TermMonths}
		pos.FaceValue = parseAmount(p.FaceValue, fmt.Sprintf("positions[%d].faceValue", i), fields)
		pos.PurchasePrice = parseAmount(p.PurchasePrice, fmt.Sprintf("positions[%d].purchasePrice", i), fields)
		pos.DistributionRateAnnual = parseAmount(p.DistributionRateAnnual, fmt.Sprintf("positions[%d].distributionRateAnnual", i), fields)
		positions[i] = pos
	}
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid sukuk request", fields))
		return
	}

	res, err := h.svc.Sukuk(r.Context(), positions)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	resp := sukukResponse{
		Positions:             make([]sukukPositionDTO, len(res.Positions)),
		TotalInvested:         res.TotalInvested.StringFixed(2),
		TotalFace:             res.TotalFace.StringFixed(2),
		TotalAnnualIncome:     res.TotalAnnualIncome.StringFixed(2),
		PortfolioCurrentYield: res.PortfolioCurrentYield.StringFixed(4),
		TotalExpectedGain:     res.TotalExpectedGain.StringFixed(2),
		Guaranteed:            res.Guaranteed,
	}
	for i, p := range res.Positions {
		resp.Positions[i] = sukukPositionDTO{
			Name:                       p.Name,
			FaceValue:                  p.FaceValue.StringFixed(2),
			PurchasePrice:              p.PurchasePrice.StringFixed(2),
			DistributionRateAnnual:     p.DistributionRateAnnual.String(),
			Frequency:                  p.Frequency,
			TermMonths:                 p.TermMonths,
			Payments:                   p.Payments,
			PeriodicDistribution:       p.PeriodicDistribution.StringFixed(2),
			TotalExpectedDistributions: p.TotalExpectedDistributions.StringFixed(2),
			RedemptionAtFace:           p.RedemptionAtFace.StringFixed(2),
			ExpectedGain:               p.ExpectedGain.StringFixed(2),
			CurrentYield:               p.CurrentYield.StringFixed(4),
		}
	}
	httpx.Data(w, http.StatusOK, resp)
}
