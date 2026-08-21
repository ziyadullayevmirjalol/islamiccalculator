// Package murabaha implements the cost-plus installment sale. The sale
// price (cost + markup) is fixed at contract time and never recalculated —
// late payments are handled by the separate late-payment-charity
// calculator, never by growing this debt.
package murabaha

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	// MarkupModeRate expresses the markup as a fraction of cost (0.10 = 10%).
	MarkupModeRate = "rate"
	// MarkupModeAmount expresses the markup as an absolute amount.
	MarkupModeAmount = "amount"

	MaxTermMonths = 360
)

type Markup struct {
	Mode  string
	Value decimal.Decimal
}

type Input struct {
	Cost         decimal.Decimal
	Markup       Markup
	DownPayment  decimal.Decimal
	TermMonths   int
	FirstDueDate time.Time // optional; zero means the schedule carries no dates
}

type Installment struct {
	N         int
	DueDate   time.Time // zero when Input.FirstDueDate was zero
	Amount    decimal.Decimal
	Principal decimal.Decimal
	Markup    decimal.Decimal
	Balance   decimal.Decimal // outstanding after this installment
}

type Result struct {
	Cost               decimal.Decimal
	MarkupTotal        decimal.Decimal
	SalePrice          decimal.Decimal
	DownPayment        decimal.Decimal
	Financed           decimal.Decimal
	MonthlyInstallment decimal.Decimal // first-installment amount; the final one may differ by the residual
	TermMonths         int
	Schedule           []Installment
}

// Calculate validates the contract and produces the fixed installment
// schedule. All amounts are treated at 2 decimal places.
func Calculate(in Input) (Result, error) {
	if err := validate(in); err != nil {
		return Result{}, err
	}

	cost := money.Round2(in.Cost)
	down := money.Round2(in.DownPayment)

	var markupTotal decimal.Decimal
	if in.Markup.Mode == MarkupModeRate {
		markupTotal = money.Round2(cost.Mul(in.Markup.Value))
	} else {
		markupTotal = money.Round2(in.Markup.Value)
	}

	salePrice := cost.Add(markupTotal)
	if down.GreaterThanOrEqual(salePrice) {
		return Result{}, apperr.Validation("downPayment must be less than the sale price",
			map[string]string{"downPayment": "too_large"})
	}
	financed := salePrice.Sub(down)

	amounts, err := money.Split(financed, in.TermMonths)
	if err != nil {
		return Result{}, apperr.Validation(err.Error(), nil)
	}

	// The down payment retires cost first, so the financed balance is
	// cost-remainder plus the whole markup.
	costFinanced := cost.Sub(down)
	if costFinanced.IsNegative() {
		costFinanced = decimal.Zero
	}
	principals := money.SplitProportional(amounts, costFinanced.Div(financed), costFinanced)

	schedule := make([]Installment, in.TermMonths)
	balance := financed
	for i, amount := range amounts {
		balance = balance.Sub(amount)
		inst := Installment{
			N:         i + 1,
			Amount:    amount,
			Principal: principals[i],
			Markup:    amount.Sub(principals[i]),
			Balance:   balance,
		}
		if !in.FirstDueDate.IsZero() {
			inst.DueDate = in.FirstDueDate.AddDate(0, i, 0)
		}
		schedule[i] = inst
	}

	return Result{
		Cost:               cost,
		MarkupTotal:        markupTotal,
		SalePrice:          salePrice,
		DownPayment:        down,
		Financed:           financed,
		MonthlyInstallment: amounts[0],
		TermMonths:         in.TermMonths,
		Schedule:           schedule,
	}, nil
}

func validate(in Input) error {
	fields := map[string]string{}
	if !in.Cost.IsPositive() {
		fields["cost"] = "must_be_positive"
	}
	if in.TermMonths < 1 || in.TermMonths > MaxTermMonths {
		fields["termMonths"] = "out_of_range"
	}
	if in.Markup.Mode != MarkupModeRate && in.Markup.Mode != MarkupModeAmount {
		fields["markup.mode"] = "must_be_rate_or_amount"
	}
	if in.Markup.Value.IsNegative() {
		fields["markup.value"] = "must_not_be_negative"
	}
	if in.DownPayment.IsNegative() {
		fields["downPayment"] = "must_not_be_negative"
	}
	if len(fields) > 0 {
		return apperr.Validation("invalid murabaha input", fields)
	}
	return nil
}
