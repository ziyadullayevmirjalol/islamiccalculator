package handler

import (
	"context"
	"net/http"

	"github.com/diyorbek/islamiccalculator/internal/domain/zakat"
	"github.com/diyorbek/islamiccalculator/internal/pkg/httpx"
)

type ReferenceService interface {
	LivestockRules(ctx context.Context) ([]zakat.LivestockRule, error)
}

type Reference struct {
	svc ReferenceService
}

func NewReference(svc ReferenceService) *Reference {
	return &Reference{svc: svc}
}

type livestockRuleDTO struct {
	Species       string         `json:"species"`
	MinCount      int            `json:"minCount"`
	MaxCount      *int           `json:"maxCount,omitempty"` // absent = open-ended
	Due           []animalDueDTO `json:"due"`
	PerExtraEvery *int           `json:"perExtraEvery,omitempty"`
	Note          string         `json:"note,omitempty"`
}

func (h *Reference) LivestockRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.svc.LivestockRules(r.Context())
	if err != nil {
		httpx.Err(w, err)
		return
	}

	dtos := make([]livestockRuleDTO, len(rules))
	for i, rule := range rules {
		dto := livestockRuleDTO{
			Species:       rule.Species,
			MinCount:      rule.MinCount,
			MaxCount:      rule.MaxCount,
			Due:           make([]animalDueDTO, len(rule.Due)),
			PerExtraEvery: rule.PerExtraEvery,
			Note:          rule.Note,
		}
		for j, due := range rule.Due {
			dto.Due[j] = animalDueDTO{Animal: due.Animal, Count: due.Count}
		}
		dtos[i] = dto
	}
	httpx.Data(w, http.StatusOK, map[string]any{"rules": dtos})
}
