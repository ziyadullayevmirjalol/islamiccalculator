package zakat

import (
	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

type BusinessInput struct {
	Cash                 decimal.Decimal
	Receivables          decimal.Decimal
	Inventory            decimal.Decimal // finished goods and stock at market value
	ShortTermLiabilities decimal.Decimal
	HawlComplete         bool
}

type BusinessResult struct {
	ZakatBase    decimal.Decimal // net zakatable working capital, floored at zero
	Nisab        Nisab
	AboveNisab   bool
	HawlComplete bool
	ZakatDue     decimal.Decimal
	Currency     string
}

// CalculateBusiness computes zakat on a company's working capital:
// cash + receivables + inventory − short-term liabilities, against the
// same nisab gate as personal wealth.
func CalculateBusiness(in BusinessInput, p Params) (BusinessResult, error) {
	if err := validateBusiness(in, p); err != nil {
		return BusinessResult{}, err
	}

	base := money.Round2(in.Cash).
		Add(money.Round2(in.Receivables)).
		Add(money.Round2(in.Inventory)).
		Sub(money.Round2(in.ShortTermLiabilities))
	if base.IsNegative() {
		base = decimal.Zero
	}

	nisab := NisabValues(p)
	above := base.GreaterThanOrEqual(nisab.Applied)

	due := decimal.Zero
	if above && in.HawlComplete {
		due = money.Round2(base.Mul(p.Rate))
	}

	return BusinessResult{
		ZakatBase:    base,
		Nisab:        nisab,
		AboveNisab:   above,
		HawlComplete: in.HawlComplete,
		ZakatDue:     due,
		Currency:     p.Currency,
	}, nil
}

func validateBusiness(in BusinessInput, p Params) error {
	fields := map[string]string{}
	if in.Cash.IsNegative() {
		fields["cash"] = "must_not_be_negative"
	}
	if in.Receivables.IsNegative() {
		fields["receivables"] = "must_not_be_negative"
	}
	if in.Inventory.IsNegative() {
		fields["inventory"] = "must_not_be_negative"
	}
	if in.ShortTermLiabilities.IsNegative() {
		fields["shortTermLiabilities"] = "must_not_be_negative"
	}
	if len(fields) > 0 {
		return apperr.Validation("invalid business zakat input", fields)
	}
	if !p.Rate.IsPositive() || p.Rate.GreaterThanOrEqual(decimal.NewFromInt(1)) ||
		!p.GoldPricePerGram.IsPositive() || !p.SilverPricePerGram.IsPositive() {
		return apperr.Internal("invalid zakat parameters", nil)
	}
	return nil
}
