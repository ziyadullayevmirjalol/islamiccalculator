package service

import (
	"context"
	"log/slog"

	"github.com/diyorbek/islamiccalculator/internal/domain/dimmusharaka"
	"github.com/diyorbek/islamiccalculator/internal/domain/ijara"
	"github.com/diyorbek/islamiccalculator/internal/domain/istisna"
	"github.com/diyorbek/islamiccalculator/internal/domain/latepayment"
	"github.com/diyorbek/islamiccalculator/internal/domain/mudaraba"
	"github.com/diyorbek/islamiccalculator/internal/domain/murabaha"
	"github.com/diyorbek/islamiccalculator/internal/domain/musharaka"
	"github.com/diyorbek/islamiccalculator/internal/domain/qardhasan"
	"github.com/diyorbek/islamiccalculator/internal/domain/salam"
)

// Finance runs the retail-finance calculator workflows. History
// persistence is best-effort: a storage failure must never take down a
// calculation the user already has.
type Finance struct {
	history CalculationRepo
}

func NewFinance(history CalculationRepo) *Finance {
	return &Finance{history: history}
}

func (s *Finance) Murabaha(ctx context.Context, in murabaha.Input) (murabaha.Result, error) {
	res, err := murabaha.Calculate(in)
	if err != nil {
		return murabaha.Result{}, err
	}
	s.record(ctx, "finance.murabaha", in, res)
	return res, nil
}

func (s *Finance) Ijara(ctx context.Context, in ijara.Input) (ijara.Result, error) {
	res, err := ijara.Calculate(in)
	if err != nil {
		return ijara.Result{}, err
	}
	s.record(ctx, "finance.ijara", in, res)
	return res, nil
}

func (s *Finance) QardHasan(ctx context.Context, in qardhasan.Input) (qardhasan.Result, error) {
	res, err := qardhasan.Calculate(in)
	if err != nil {
		return qardhasan.Result{}, err
	}
	s.record(ctx, "finance.qard_hasan", in, res)
	return res, nil
}

func (s *Finance) Mudaraba(ctx context.Context, in mudaraba.Input) (mudaraba.Result, error) {
	res, err := mudaraba.Calculate(in)
	if err != nil {
		return mudaraba.Result{}, err
	}
	s.record(ctx, "finance.mudaraba", in, res)
	return res, nil
}

func (s *Finance) Salam(ctx context.Context, in salam.Input) (salam.Result, error) {
	res, err := salam.Calculate(in)
	if err != nil {
		return salam.Result{}, err
	}
	s.record(ctx, "finance.salam", in, res)
	return res, nil
}

func (s *Finance) Istisna(ctx context.Context, in istisna.Input) (istisna.Result, error) {
	res, err := istisna.Calculate(in)
	if err != nil {
		return istisna.Result{}, err
	}
	s.record(ctx, "finance.istisna", in, res)
	return res, nil
}

func (s *Finance) Musharaka(ctx context.Context, in musharaka.Input) (musharaka.Result, error) {
	res, err := musharaka.Calculate(in)
	if err != nil {
		return musharaka.Result{}, err
	}
	s.record(ctx, "finance.musharaka", in, res)
	return res, nil
}

func (s *Finance) DimMusharaka(ctx context.Context, in dimmusharaka.Input) (dimmusharaka.Result, error) {
	res, err := dimmusharaka.Calculate(in)
	if err != nil {
		return dimmusharaka.Result{}, err
	}
	s.record(ctx, "finance.diminishing_musharaka", in, res)
	return res, nil
}

func (s *Finance) LatePayment(ctx context.Context, in latepayment.Input) (latepayment.Result, error) {
	res, err := latepayment.Calculate(in)
	if err != nil {
		return latepayment.Result{}, err
	}
	s.record(ctx, "finance.late_payment", in, res)
	return res, nil
}

func (s *Finance) record(ctx context.Context, calcType string, inputs, result any) {
	if err := s.history.Save(ctx, newRecord(ctx, calcType, inputs, result)); err != nil {
		slog.Warn("save calculation history", "calc_type", calcType, "err", err)
	}
}
