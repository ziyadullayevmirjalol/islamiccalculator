// Package qardhasan implements the benevolent loan: the borrower repays
// exactly the principal plus, at most, a fixed administrative fee. The
// fee is an absolute amount by design — a percentage of the principal
// would be riba, so the API gives no way to express one.
package qardhasan

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const MaxTermMonths = 360

type Input struct {
	Principal    decimal.Decimal
	ServiceFee   decimal.Decimal // fixed amount, paid once upfront; zero is the norm
	TermMonths   int
	FirstDueDate time.Time // optional
}

type Installment struct {
	N       int
	DueDate time.Time
	Amount  decimal.Decimal
	Balance decimal.Decimal
}

type Result struct {
	Principal          decimal.Decimal
	ServiceFee         decimal.Decimal
	TotalRepayment     decimal.Decimal // principal + fee
	MonthlyInstallment decimal.Decimal
	TermMonths         int
	Schedule           []Installment // principal only; the fee is due upfront
}

func Calculate(in Input) (Result, error) {
	if err := validate(in); err != nil {
		return Result{}, err
	}

	principal := money.Round2(in.Principal)
	fee := money.Round2(in.ServiceFee)

	amounts, err := money.Split(principal, in.TermMonths)
	if err != nil {
		return Result{}, apperr.Validation(err.Error(), nil)
	}

	schedule := make([]Installment, in.TermMonths)
	balance := principal
	for i, amount := range amounts {
		balance = balance.Sub(amount)
		inst := Installment{N: i + 1, Amount: amount, Balance: balance}
		if !in.FirstDueDate.IsZero() {
			inst.DueDate = in.FirstDueDate.AddDate(0, i, 0)
		}
		schedule[i] = inst
	}

	return Result{
		Principal:          principal,
		ServiceFee:         fee,
		TotalRepayment:     principal.Add(fee),
		MonthlyInstallment: amounts[0],
		TermMonths:         in.TermMonths,
		Schedule:           schedule,
	}, nil
}

func validate(in Input) error {
	fields := map[string]string{}
	if !in.Principal.IsPositive() {
		fields["principal"] = "must_be_positive"
	}
	if in.ServiceFee.IsNegative() {
		fields["serviceFee"] = "must_not_be_negative"
	}
	// A "service fee" rivaling the principal is disguised riba.
	if in.Principal.IsPositive() && in.ServiceFee.GreaterThanOrEqual(in.Principal) {
		fields["serviceFee"] = "must_be_less_than_principal"
	}
	if in.TermMonths < 1 || in.TermMonths > MaxTermMonths {
		fields["termMonths"] = "out_of_range"
	}
	if len(fields) > 0 {
		return apperr.Validation("invalid qard al-hasan input", fields)
	}
	return nil
}
