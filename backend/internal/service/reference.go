package service

import (
	"context"

	"github.com/diyorbek/islamiccalculator/internal/domain/zakat"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

// Reference serves read-only reference data for client display.
type Reference struct {
	rules LivestockRuleRepo
}

func NewReference(rules LivestockRuleRepo) *Reference {
	return &Reference{rules: rules}
}

func (s *Reference) LivestockRules(ctx context.Context) ([]zakat.LivestockRule, error) {
	rules, err := s.rules.All(ctx)
	if err != nil {
		return nil, apperr.Internal("livestock rules unavailable", err)
	}
	return rules, nil
}
