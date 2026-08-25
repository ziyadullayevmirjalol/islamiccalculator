// Package dimmusharaka implements Diminishing Musharakah (Musharakah
// Mutanaqisah) — the standard Islamic home-financing structure: client
// and bank co-own the property; the client pays rent on the bank's
// outstanding share and buys a slice of it every month until the bank's
// share reaches zero. Rent is charged for actual use of the bank's
// share — it declines as ownership transfers, and nothing compounds.
package dimmusharaka

import (
	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const MaxTermMonths = 360

type Input struct {
	PropertyValue    decimal.Decimal
	DownPayment      decimal.Decimal // client's initial share
	AnnualRentalRate decimal.Decimal // charged on the bank's outstanding share, e.g. 0.05
	TermMonths       int
}

type Month struct {
	N                int
	BankShareBefore  decimal.Decimal // bank's share value at month start
	Rent             decimal.Decimal
	Acquisition      decimal.Decimal
	Payment          decimal.Decimal // rent + acquisition
	OwnershipPercent decimal.Decimal // client's ownership after this month, 4dp fraction
}

type Result struct {
	PropertyValue           decimal.Decimal
	DownPayment             decimal.Decimal
	BankFinancing           decimal.Decimal
	InitialOwnershipPercent decimal.Decimal // client's starting share, 4dp fraction
	MonthlyAcquisition      decimal.Decimal // typical monthly share purchase
	FirstMonthPayment       decimal.Decimal
	TotalRent               decimal.Decimal
	TotalAcquisition        decimal.Decimal
	TotalPaid               decimal.Decimal // down payment + rent + acquisitions
	TermMonths              int
	Schedule                []Month
}

func Calculate(in Input) (Result, error) {
	if err := validate(in); err != nil {
		return Result{}, err
	}

	value := money.Round2(in.PropertyValue)
	down := money.Round2(in.DownPayment)
	financing := value.Sub(down)

	acquisitions, err := money.Split(financing, in.TermMonths)
	if err != nil {
		return Result{}, apperr.Validation(err.Error(), nil)
	}

	twelve := decimal.NewFromInt(12)
	schedule := make([]Month, in.TermMonths)
	bankShare := financing
	totalRent := decimal.Zero

	for i, acquisition := range acquisitions {
		rent := money.Round2(bankShare.Mul(in.AnnualRentalRate).Div(twelve))
		totalRent = totalRent.Add(rent)
		after := bankShare.Sub(acquisition)
		schedule[i] = Month{
			N:                i + 1,
			BankShareBefore:  bankShare,
			Rent:             rent,
			Acquisition:      acquisition,
			Payment:          rent.Add(acquisition),
			OwnershipPercent: value.Sub(after).Div(value).Round(4),
		}
		bankShare = after
	}

	return Result{
		PropertyValue:           value,
		DownPayment:             down,
		BankFinancing:           financing,
		InitialOwnershipPercent: down.Div(value).Round(4),
		MonthlyAcquisition:      acquisitions[0],
		FirstMonthPayment:       schedule[0].Payment,
		TotalRent:               totalRent,
		TotalAcquisition:        financing,
		TotalPaid:               down.Add(financing).Add(totalRent),
		TermMonths:              in.TermMonths,
		Schedule:                schedule,
	}, nil
}

func validate(in Input) error {
	fields := map[string]string{}
	if !in.PropertyValue.IsPositive() {
		fields["propertyValue"] = "must_be_positive"
	}
	if in.DownPayment.IsNegative() {
		fields["downPayment"] = "must_not_be_negative"
	} else if in.PropertyValue.IsPositive() && in.DownPayment.GreaterThanOrEqual(in.PropertyValue) {
		fields["downPayment"] = "must_be_less_than_property_value"
	}
	one := decimal.NewFromInt(1)
	if in.AnnualRentalRate.IsNegative() || in.AnnualRentalRate.GreaterThanOrEqual(one) {
		fields["annualRentalRate"] = "out_of_range"
	}
	if in.TermMonths < 1 || in.TermMonths > MaxTermMonths {
		fields["termMonths"] = "out_of_range"
	}
	if len(fields) > 0 {
		return apperr.Validation("invalid diminishing musharakah input", fields)
	}
	return nil
}
