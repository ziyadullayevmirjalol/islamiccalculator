package zakat

import (
	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/money"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

const (
	// KindFidya: one feeding per missed fast (for those unable to fast).
	KindFidya = "fidya"
	// KindKaffarahFast: sixty feedings per deliberately broken fast.
	KindKaffarahFast = "kaffarah_fast"
	// KindKaffarahOath: ten feedings per broken oath.
	KindKaffarahOath = "kaffarah_oath"
)

type FidyaParams struct {
	DailyRate    decimal.Decimal // cost of feeding one poor person for a day
	Currency     string
	FastFeedings int  // typically 60
	OathFeedings int  // typically 10
	NeedsReview  bool // seed value not yet scholar-approved
}

type FidyaInput struct {
	Kind  string
	Count int // missed days / broken fasts / broken oaths
}

type FidyaResult struct {
	Kind        string
	Count       int
	Feedings    int // feedings per unit
	DailyRate   decimal.Decimal
	TotalDue    decimal.Decimal
	Currency    string
	NeedsReview bool
}

func CalculateFidya(in FidyaInput, p FidyaParams) (FidyaResult, error) {
	fields := map[string]string{}
	var feedings int
	switch in.Kind {
	case KindFidya:
		feedings = 1
	case KindKaffarahFast:
		feedings = p.FastFeedings
	case KindKaffarahOath:
		feedings = p.OathFeedings
	default:
		fields["kind"] = "must_be_fidya_or_kaffarah"
	}
	if in.Count < 1 {
		fields["count"] = "must_be_positive"
	}
	if len(fields) > 0 {
		return FidyaResult{}, apperr.Validation("invalid fidya input", fields)
	}
	if !p.DailyRate.IsPositive() || feedings < 1 {
		return FidyaResult{}, apperr.Internal("invalid fidya parameters", nil)
	}

	total := money.Round2(p.DailyRate.
		Mul(decimal.NewFromInt(int64(feedings))).
		Mul(decimal.NewFromInt(int64(in.Count))))

	return FidyaResult{
		Kind:        in.Kind,
		Count:       in.Count,
		Feedings:    feedings,
		DailyRate:   p.DailyRate,
		TotalDue:    total,
		Currency:    p.Currency,
		NeedsReview: p.NeedsReview,
	}, nil
}
