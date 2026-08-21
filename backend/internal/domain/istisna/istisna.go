// Package istisna implements the manufacturing/construction contract:
// the price is fixed up front and paid in stages tied to milestones
// (or split equally). Milestone percentages must cover the contract
// exactly — a schedule that over- or under-collects is rejected.
package istisna

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	// ModeMilestones pays per named milestone percentages.
	ModeMilestones = "milestones"
	// ModeEqual splits the price into equal stages.
	ModeEqual = "equal"

	MaxStages = 120
)

type Milestone struct {
	Name    string
	Percent decimal.Decimal // of contract price, (0..100]
	DueDate time.Time       // optional
}

type Input struct {
	Mode          string
	ContractPrice decimal.Decimal
	Milestones    []Milestone // mode == milestones
	Stages        int         // mode == equal
}

type StagePayment struct {
	N       int
	Name    string
	Percent decimal.Decimal
	DueDate time.Time
	Amount  decimal.Decimal
}

type Result struct {
	ContractPrice decimal.Decimal
	Stages        int
	Schedule      []StagePayment
}

func Calculate(in Input) (Result, error) {
	if err := validate(in); err != nil {
		return Result{}, err
	}

	price := money.Round2(in.ContractPrice)

	if in.Mode == ModeEqual {
		amounts, err := money.Split(price, in.Stages)
		if err != nil {
			return Result{}, apperr.Validation(err.Error(), nil)
		}
		pct := decimal.NewFromInt(100).Div(decimal.NewFromInt(int64(in.Stages))).Round(4)
		schedule := make([]StagePayment, in.Stages)
		for i, amount := range amounts {
			schedule[i] = StagePayment{N: i + 1, Percent: pct, Amount: amount}
		}
		return Result{ContractPrice: price, Stages: in.Stages, Schedule: schedule}, nil
	}

	hundred := decimal.NewFromInt(100)
	schedule := make([]StagePayment, len(in.Milestones))
	paid := decimal.Zero
	for i, m := range in.Milestones {
		var amount decimal.Decimal
		if i == len(in.Milestones)-1 {
			amount = price.Sub(paid) // absorb the rounding residual
		} else {
			amount = money.Round2(price.Mul(m.Percent).Div(hundred))
		}
		paid = paid.Add(amount)
		schedule[i] = StagePayment{
			N:       i + 1,
			Name:    m.Name,
			Percent: m.Percent,
			DueDate: m.DueDate,
			Amount:  amount,
		}
	}

	return Result{ContractPrice: price, Stages: len(in.Milestones), Schedule: schedule}, nil
}

func validate(in Input) error {
	fields := map[string]string{}
	if !in.ContractPrice.IsPositive() {
		fields["contractPrice"] = "must_be_positive"
	}
	switch in.Mode {
	case ModeEqual:
		if in.Stages < 1 || in.Stages > MaxStages {
			fields["stages"] = "out_of_range"
		}
	case ModeMilestones:
		if len(in.Milestones) < 1 || len(in.Milestones) > MaxStages {
			fields["milestones"] = "out_of_range"
		} else {
			sum := decimal.Zero
			for _, m := range in.Milestones {
				if !m.Percent.IsPositive() {
					fields["milestones"] = "percent_must_be_positive"
					break
				}
				sum = sum.Add(m.Percent)
			}
			if _, bad := fields["milestones"]; !bad && !sum.Equal(decimal.NewFromInt(100)) {
				// A schedule that doesn't cover the price exactly is not a
				// valid istisna payment structure.
				fields["milestones"] = "percents_must_sum_to_100"
			}
		}
	default:
		fields["mode"] = "must_be_milestones_or_equal"
	}
	if len(fields) > 0 {
		return apperr.Validation("invalid istisna input", fields)
	}
	return nil
}
