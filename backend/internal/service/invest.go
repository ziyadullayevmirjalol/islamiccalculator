package service

import (
	"context"
	"log/slog"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/screener"
	"github.com/diyorbek/islamiccalculator/internal/domain/sukuk"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

type ScreenerRule struct {
	Key         string
	Threshold   decimal.Decimal
	Description string
}

type ScreenerRuleRepo interface {
	All(ctx context.Context) ([]ScreenerRule, error)
}

// Invest runs the investment calculators: AAOIFI screening and sukuk math.
type Invest struct {
	rules   ScreenerRuleRepo
	history CalculationRepo
}

func NewInvest(rules ScreenerRuleRepo, history CalculationRepo) *Invest {
	return &Invest{rules: rules, history: history}
}

func (s *Invest) Screen(ctx context.Context, in screener.Input) (screener.Result, error) {
	params, err := s.loadScreenerParams(ctx)
	if err != nil {
		return screener.Result{}, err
	}
	res, err := screener.Screen(in, params)
	if err != nil {
		return screener.Result{}, err
	}
	s.record(ctx, "invest.screener", in, res)
	return res, nil
}

func (s *Invest) Sukuk(ctx context.Context, positions []sukuk.Position) (sukuk.PortfolioResult, error) {
	res, err := sukuk.Calculate(positions)
	if err != nil {
		return sukuk.PortfolioResult{}, err
	}
	s.record(ctx, "invest.sukuk", positions, res)
	return res, nil
}

func (s *Invest) loadScreenerParams(ctx context.Context) (screener.Params, error) {
	rules, err := s.rules.All(ctx)
	if err != nil {
		return screener.Params{}, apperr.Internal("screener rules unavailable", err)
	}

	var params screener.Params
	seen := map[string]bool{}
	for _, r := range rules {
		seen[r.Key] = true
		switch r.Key {
		case screener.CheckDebt:
			params.DebtToMarketCapMax = r.Threshold
		case screener.CheckInvest:
			params.InvestToMarketCapMax = r.Threshold
		case screener.CheckImpure:
			params.ImpureIncomeMax = r.Threshold
		}
	}
	for _, key := range []string{screener.CheckDebt, screener.CheckInvest, screener.CheckImpure} {
		if !seen[key] {
			return screener.Params{}, apperr.Internal("screener rule missing: "+key, nil)
		}
	}
	return params, nil
}

func (s *Invest) record(ctx context.Context, calcType string, inputs, result any) {
	if err := s.history.Save(ctx, newRecord(ctx, calcType, inputs, result)); err != nil {
		slog.Warn("save calculation history", "calc_type", calcType, "err", err)
	}
}
