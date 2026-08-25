package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/shopspring/decimal"

	"github.com/diyorbek/islamiccalculator/internal/domain/dimmusharaka"
	"github.com/diyorbek/islamiccalculator/internal/domain/ijara"
	"github.com/diyorbek/islamiccalculator/internal/domain/istisna"
	"github.com/diyorbek/islamiccalculator/internal/domain/latepayment"
	"github.com/diyorbek/islamiccalculator/internal/domain/mudaraba"
	"github.com/diyorbek/islamiccalculator/internal/domain/murabaha"
	"github.com/diyorbek/islamiccalculator/internal/domain/musharaka"
	"github.com/diyorbek/islamiccalculator/internal/domain/qardhasan"
	"github.com/diyorbek/islamiccalculator/internal/domain/salam"
	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
	"github.com/diyorbek/islamiccalculator/internal/pkg/httpx"
)

type FinanceService interface {
	Murabaha(ctx context.Context, in murabaha.Input) (murabaha.Result, error)
	Ijara(ctx context.Context, in ijara.Input) (ijara.Result, error)
	QardHasan(ctx context.Context, in qardhasan.Input) (qardhasan.Result, error)
	Mudaraba(ctx context.Context, in mudaraba.Input) (mudaraba.Result, error)
	DimMusharaka(ctx context.Context, in dimmusharaka.Input) (dimmusharaka.Result, error)
	Salam(ctx context.Context, in salam.Input) (salam.Result, error)
	Istisna(ctx context.Context, in istisna.Input) (istisna.Result, error)
	Musharaka(ctx context.Context, in musharaka.Input) (musharaka.Result, error)
	LatePayment(ctx context.Context, in latepayment.Input) (latepayment.Result, error)
}

type Finance struct {
	svc FinanceService
}

func NewFinance(svc FinanceService) *Finance {
	return &Finance{svc: svc}
}

type murabahaRequest struct {
	Cost         string `json:"cost"`
	Markup       struct {
		Mode  string `json:"mode"`
		Value string `json:"value"`
	} `json:"markup"`
	DownPayment  string `json:"downPayment"`
	TermMonths   int    `json:"termMonths"`
	FirstDueDate string `json:"firstDueDate"` // optional, YYYY-MM-DD
}

type installmentDTO struct {
	N         int    `json:"n"`
	DueDate   string `json:"dueDate,omitempty"`
	Amount    string `json:"amount"`
	Principal string `json:"principal"`
	Markup    string `json:"markup"`
	Balance   string `json:"balance"`
}

type murabahaResponse struct {
	Cost               string           `json:"cost"`
	MarkupTotal        string           `json:"markupTotal"`
	SalePrice          string           `json:"salePrice"`
	DownPayment        string           `json:"downPayment"`
	Financed           string           `json:"financed"`
	MonthlyInstallment string           `json:"monthlyInstallment"`
	TermMonths         int              `json:"termMonths"`
	Schedule           []installmentDTO `json:"schedule"`
}

func (h *Finance) Murabaha(w http.ResponseWriter, r *http.Request) {
	var req murabahaRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := murabaha.Input{TermMonths: req.TermMonths}
	in.Cost = parseAmount(req.Cost, "cost", fields)
	in.Markup.Mode = req.Markup.Mode
	in.Markup.Value = parseAmount(req.Markup.Value, "markup.value", fields)
	if req.DownPayment != "" {
		in.DownPayment = parseAmount(req.DownPayment, "downPayment", fields)
	}
	if req.FirstDueDate != "" {
		t, err := time.Parse("2006-01-02", req.FirstDueDate)
		if err != nil {
			fields["firstDueDate"] = "must_be_yyyy_mm_dd"
		}
		in.FirstDueDate = t
	}
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid murabaha request", fields))
		return
	}

	res, err := h.svc.Murabaha(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	resp := murabahaResponse{
		Cost:               res.Cost.StringFixed(2),
		MarkupTotal:        res.MarkupTotal.StringFixed(2),
		SalePrice:          res.SalePrice.StringFixed(2),
		DownPayment:        res.DownPayment.StringFixed(2),
		Financed:           res.Financed.StringFixed(2),
		MonthlyInstallment: res.MonthlyInstallment.StringFixed(2),
		TermMonths:         res.TermMonths,
		Schedule:           make([]installmentDTO, len(res.Schedule)),
	}
	for i, inst := range res.Schedule {
		dto := installmentDTO{
			N:         inst.N,
			Amount:    inst.Amount.StringFixed(2),
			Principal: inst.Principal.StringFixed(2),
			Markup:    inst.Markup.StringFixed(2),
			Balance:   inst.Balance.StringFixed(2),
		}
		if !inst.DueDate.IsZero() {
			dto.DueDate = inst.DueDate.Format("2006-01-02")
		}
		resp.Schedule[i] = dto
	}
	httpx.Data(w, http.StatusOK, resp)
}

// --- ijara -----------------------------------------------------------------

type ijaraRequest struct {
	Mode      string `json:"mode"`
	AssetCost string `json:"assetCost"`
	Profit    struct {
		Mode  string `json:"mode"`
		Value string `json:"value"`
	} `json:"profit"`
	MonthlyRent    string `json:"monthlyRent"`
	TransferPrice  string `json:"transferPrice"`
	AdvancePayment string `json:"advancePayment"`
	TermMonths     int    `json:"termMonths"`
	FirstDueDate   string `json:"firstDueDate"`
}

type rentalDTO struct {
	N       int    `json:"n"`
	DueDate string `json:"dueDate,omitempty"`
	Amount  string `json:"amount"`
	Balance string `json:"balance"`
}

type ijaraResponse struct {
	AssetCost      string      `json:"assetCost"`
	TransferPrice  string      `json:"transferPrice"`
	AdvancePayment string      `json:"advancePayment"`
	TotalRentals   string      `json:"totalRentals"`
	TotalReceipts  string      `json:"totalReceipts"`
	ProfitTotal    string      `json:"profitTotal"`
	ProfitRate     string      `json:"profitRate"`
	MonthlyRent    string      `json:"monthlyRent"`
	TermMonths     int         `json:"termMonths"`
	Schedule       []rentalDTO `json:"schedule"`
}

func (h *Finance) Ijara(w http.ResponseWriter, r *http.Request) {
	var req ijaraRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := ijara.Input{Mode: req.Mode, TermMonths: req.TermMonths}
	in.AssetCost = parseAmount(req.AssetCost, "assetCost", fields)
	in.Profit.Mode = req.Profit.Mode
	if req.Profit.Value != "" {
		in.Profit.Value = parseAmount(req.Profit.Value, "profit.value", fields)
	}
	if req.MonthlyRent != "" {
		in.MonthlyRent = parseAmount(req.MonthlyRent, "monthlyRent", fields)
	}
	if req.TransferPrice != "" {
		in.TransferPrice = parseAmount(req.TransferPrice, "transferPrice", fields)
	}
	if req.AdvancePayment != "" {
		in.AdvancePayment = parseAmount(req.AdvancePayment, "advancePayment", fields)
	}
	if req.FirstDueDate != "" {
		t, err := time.Parse("2006-01-02", req.FirstDueDate)
		if err != nil {
			fields["firstDueDate"] = "must_be_yyyy_mm_dd"
		}
		in.FirstDueDate = t
	}
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid ijara request", fields))
		return
	}

	res, err := h.svc.Ijara(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	resp := ijaraResponse{
		AssetCost:      res.AssetCost.StringFixed(2),
		TransferPrice:  res.TransferPrice.StringFixed(2),
		AdvancePayment: res.AdvancePayment.StringFixed(2),
		TotalRentals:   res.TotalRentals.StringFixed(2),
		TotalReceipts:  res.TotalReceipts.StringFixed(2),
		ProfitTotal:    res.ProfitTotal.StringFixed(2),
		ProfitRate:     res.ProfitRate.StringFixed(4),
		MonthlyRent:    res.MonthlyRent.StringFixed(2),
		TermMonths:     res.TermMonths,
		Schedule:       make([]rentalDTO, len(res.Schedule)),
	}
	for i, rental := range res.Schedule {
		dto := rentalDTO{
			N:       rental.N,
			Amount:  rental.Amount.StringFixed(2),
			Balance: rental.Balance.StringFixed(2),
		}
		if !rental.DueDate.IsZero() {
			dto.DueDate = rental.DueDate.Format("2006-01-02")
		}
		resp.Schedule[i] = dto
	}
	httpx.Data(w, http.StatusOK, resp)
}

// --- qard al-hasan ---------------------------------------------------------

type qardHasanRequest struct {
	Principal    string `json:"principal"`
	ServiceFee   string `json:"serviceFee"`
	TermMonths   int    `json:"termMonths"`
	FirstDueDate string `json:"firstDueDate"`
}

type qardHasanResponse struct {
	Principal          string      `json:"principal"`
	ServiceFee         string      `json:"serviceFee"`
	TotalRepayment     string      `json:"totalRepayment"`
	MonthlyInstallment string      `json:"monthlyInstallment"`
	TermMonths         int         `json:"termMonths"`
	Schedule           []rentalDTO `json:"schedule"`
}

func (h *Finance) QardHasan(w http.ResponseWriter, r *http.Request) {
	var req qardHasanRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := qardhasan.Input{TermMonths: req.TermMonths}
	in.Principal = parseAmount(req.Principal, "principal", fields)
	if req.ServiceFee != "" {
		in.ServiceFee = parseAmount(req.ServiceFee, "serviceFee", fields)
	}
	if req.FirstDueDate != "" {
		t, err := time.Parse("2006-01-02", req.FirstDueDate)
		if err != nil {
			fields["firstDueDate"] = "must_be_yyyy_mm_dd"
		}
		in.FirstDueDate = t
	}
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid qard al-hasan request", fields))
		return
	}

	res, err := h.svc.QardHasan(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	resp := qardHasanResponse{
		Principal:          res.Principal.StringFixed(2),
		ServiceFee:         res.ServiceFee.StringFixed(2),
		TotalRepayment:     res.TotalRepayment.StringFixed(2),
		MonthlyInstallment: res.MonthlyInstallment.StringFixed(2),
		TermMonths:         res.TermMonths,
		Schedule:           make([]rentalDTO, len(res.Schedule)),
	}
	for i, inst := range res.Schedule {
		dto := rentalDTO{
			N:       inst.N,
			Amount:  inst.Amount.StringFixed(2),
			Balance: inst.Balance.StringFixed(2),
		}
		if !inst.DueDate.IsZero() {
			dto.DueDate = inst.DueDate.Format("2006-01-02")
		}
		resp.Schedule[i] = dto
	}
	httpx.Data(w, http.StatusOK, resp)
}

// --- mudaraba / wakala deposit ---------------------------------------------

type mudarabaRequest struct {
	Mode           string `json:"mode"`
	Amount         string `json:"amount"`
	PoolRateAnnual string `json:"poolRateAnnual"`
	ShareRatio     string `json:"shareRatio"`
	WakalaFeeRate  string `json:"wakalaFeeRate"`
	TermMonths     int    `json:"termMonths"`
}

type mudarabaResponse struct {
	Mode                  string `json:"mode"`
	Amount                string `json:"amount"`
	EffectiveAnnualRate   string `json:"effectiveAnnualRate"`
	ExpectedProfit        string `json:"expectedProfit"`
	ExpectedMonthlyProfit string `json:"expectedMonthlyProfit"`
	ExpectedTotal         string `json:"expectedTotal"`
	TermMonths            int    `json:"termMonths"`
	Guaranteed            bool   `json:"guaranteed"`
}

func (h *Finance) Mudaraba(w http.ResponseWriter, r *http.Request) {
	var req mudarabaRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := mudaraba.Input{Mode: req.Mode, TermMonths: req.TermMonths}
	in.Amount = parseAmount(req.Amount, "amount", fields)
	in.PoolRateAnnual = parseAmount(req.PoolRateAnnual, "poolRateAnnual", fields)
	if req.ShareRatio != "" {
		in.ShareRatio = parseAmount(req.ShareRatio, "shareRatio", fields)
	}
	if req.WakalaFeeRate != "" {
		in.WakalaFeeRate = parseAmount(req.WakalaFeeRate, "wakalaFeeRate", fields)
	}
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid deposit request", fields))
		return
	}

	res, err := h.svc.Mudaraba(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	httpx.Data(w, http.StatusOK, mudarabaResponse{
		Mode:                  res.Mode,
		Amount:                res.Amount.StringFixed(2),
		EffectiveAnnualRate:   res.EffectiveAnnualRate.StringFixed(4),
		ExpectedProfit:        res.ExpectedProfit.StringFixed(2),
		ExpectedMonthlyProfit: res.ExpectedMonthlyProfit.StringFixed(2),
		ExpectedTotal:         res.ExpectedTotal.StringFixed(2),
		TermMonths:            res.TermMonths,
		Guaranteed:            res.Guaranteed,
	})
}

// --- salam -----------------------------------------------------------------

type salamRequest struct {
	Quantity          string `json:"quantity"`
	UnitPrice         string `json:"unitPrice"`
	ExpectedUnitPrice string `json:"expectedUnitPrice"`
	DeliveryDate      string `json:"deliveryDate"` // optional, YYYY-MM-DD
}

type salamResponse struct {
	Quantity            string `json:"quantity"`
	UnitPrice           string `json:"unitPrice"`
	AdvanceTotal        string `json:"advanceTotal"`
	ExpectedUnitPrice   string `json:"expectedUnitPrice"`
	ExpectedMarketValue string `json:"expectedMarketValue"`
	ExpectedMargin      string `json:"expectedMargin"`
	MarginRate          string `json:"marginRate"`
	DeliveryDate        string `json:"deliveryDate,omitempty"`
}

func (h *Finance) Salam(w http.ResponseWriter, r *http.Request) {
	var req salamRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	var in salam.Input
	in.Quantity = parseAmount(req.Quantity, "quantity", fields)
	in.UnitPrice = parseAmount(req.UnitPrice, "unitPrice", fields)
	in.ExpectedUnitPrice = parseAmount(req.ExpectedUnitPrice, "expectedUnitPrice", fields)
	if req.DeliveryDate != "" {
		t, err := time.Parse("2006-01-02", req.DeliveryDate)
		if err != nil {
			fields["deliveryDate"] = "must_be_yyyy_mm_dd"
		}
		in.DeliveryDate = t
	}
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid salam request", fields))
		return
	}

	res, err := h.svc.Salam(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	resp := salamResponse{
		Quantity:            res.Quantity.String(),
		UnitPrice:           res.UnitPrice.StringFixed(2),
		AdvanceTotal:        res.AdvanceTotal.StringFixed(2),
		ExpectedUnitPrice:   res.ExpectedUnitPrice.StringFixed(2),
		ExpectedMarketValue: res.ExpectedMarketValue.StringFixed(2),
		ExpectedMargin:      res.ExpectedMargin.StringFixed(2),
		MarginRate:          res.MarginRate.StringFixed(4),
	}
	if !res.DeliveryDate.IsZero() {
		resp.DeliveryDate = res.DeliveryDate.Format("2006-01-02")
	}
	httpx.Data(w, http.StatusOK, resp)
}

// --- istisna ---------------------------------------------------------------

type istisnaRequest struct {
	Mode          string `json:"mode"`
	ContractPrice string `json:"contractPrice"`
	Milestones    []struct {
		Name    string `json:"name"`
		Percent string `json:"percent"`
		DueDate string `json:"dueDate"`
	} `json:"milestones"`
	Stages int `json:"stages"`
}

type stagePaymentDTO struct {
	N       int    `json:"n"`
	Name    string `json:"name,omitempty"`
	Percent string `json:"percent"`
	DueDate string `json:"dueDate,omitempty"`
	Amount  string `json:"amount"`
}

type istisnaResponse struct {
	ContractPrice string            `json:"contractPrice"`
	Stages        int               `json:"stages"`
	Schedule      []stagePaymentDTO `json:"schedule"`
}

func (h *Finance) Istisna(w http.ResponseWriter, r *http.Request) {
	var req istisnaRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := istisna.Input{Mode: req.Mode, Stages: req.Stages}
	in.ContractPrice = parseAmount(req.ContractPrice, "contractPrice", fields)
	for i, m := range req.Milestones {
		ms := istisna.Milestone{Name: m.Name}
		ms.Percent = parseAmount(m.Percent, fmt.Sprintf("milestones[%d].percent", i), fields)
		if m.DueDate != "" {
			t, err := time.Parse("2006-01-02", m.DueDate)
			if err != nil {
				fields[fmt.Sprintf("milestones[%d].dueDate", i)] = "must_be_yyyy_mm_dd"
			}
			ms.DueDate = t
		}
		in.Milestones = append(in.Milestones, ms)
	}
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid istisna request", fields))
		return
	}

	res, err := h.svc.Istisna(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	resp := istisnaResponse{
		ContractPrice: res.ContractPrice.StringFixed(2),
		Stages:        res.Stages,
		Schedule:      make([]stagePaymentDTO, len(res.Schedule)),
	}
	for i, s := range res.Schedule {
		dto := stagePaymentDTO{
			N:       s.N,
			Name:    s.Name,
			Percent: s.Percent.StringFixed(2),
			Amount:  s.Amount.StringFixed(2),
		}
		if !s.DueDate.IsZero() {
			dto.DueDate = s.DueDate.Format("2006-01-02")
		}
		resp.Schedule[i] = dto
	}
	httpx.Data(w, http.StatusOK, resp)
}

// --- musharaka -------------------------------------------------------------

type musharakaRequest struct {
	Partners []struct {
		Name               string `json:"name"`
		Capital            string `json:"capital"`
		ProfitSharePercent string `json:"profitSharePercent"`
	} `json:"partners"`
	ResultType string `json:"resultType"`
	Amount     string `json:"amount"`
}

type partnerShareDTO struct {
	Name               string `json:"name,omitempty"`
	Capital            string `json:"capital"`
	CapitalShare       string `json:"capitalShare"`
	ProfitSharePercent string `json:"profitSharePercent"`
	AppliedShare       string `json:"appliedShare"`
	Amount             string `json:"amount"`
}

type musharakaResponse struct {
	TotalCapital string            `json:"totalCapital"`
	ResultType   string            `json:"resultType"`
	Amount       string            `json:"amount"`
	Basis        string            `json:"basis"`
	Shares       []partnerShareDTO `json:"shares"`
}

func (h *Finance) Musharaka(w http.ResponseWriter, r *http.Request) {
	var req musharakaRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := musharaka.Input{ResultType: req.ResultType}
	in.Amount = parseAmount(req.Amount, "amount", fields)
	for i, p := range req.Partners {
		partner := musharaka.Partner{Name: p.Name}
		partner.Capital = parseAmount(p.Capital, fmt.Sprintf("partners[%d].capital", i), fields)
		partner.ProfitSharePercent = parseAmount(p.ProfitSharePercent, fmt.Sprintf("partners[%d].profitSharePercent", i), fields)
		in.Partners = append(in.Partners, partner)
	}
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid musharaka request", fields))
		return
	}

	res, err := h.svc.Musharaka(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	resp := musharakaResponse{
		TotalCapital: res.TotalCapital.StringFixed(2),
		ResultType:   res.ResultType,
		Amount:       res.Amount.StringFixed(2),
		Basis:        res.Basis,
		Shares:       make([]partnerShareDTO, len(res.Shares)),
	}
	for i, s := range res.Shares {
		resp.Shares[i] = partnerShareDTO{
			Name:               s.Name,
			Capital:            s.Capital.StringFixed(2),
			CapitalShare:       s.CapitalShare.StringFixed(4),
			ProfitSharePercent: s.ProfitSharePercent.StringFixed(2),
			AppliedShare:       s.AppliedShare.StringFixed(4),
			Amount:             s.Amount.StringFixed(2),
		}
	}
	httpx.Data(w, http.StatusOK, resp)
}

// --- diminishing musharakah ------------------------------------------------

type dimMusharakaRequest struct {
	PropertyValue    string `json:"propertyValue"`
	DownPayment      string `json:"downPayment"`
	AnnualRentalRate string `json:"annualRentalRate"`
	TermMonths       int    `json:"termMonths"`
}

type dimMonthDTO struct {
	N                int    `json:"n"`
	BankShareBefore  string `json:"bankShareBefore"`
	Rent             string `json:"rent"`
	Acquisition      string `json:"acquisition"`
	Payment          string `json:"payment"`
	OwnershipPercent string `json:"ownershipPercent"`
}

type dimMusharakaResponse struct {
	PropertyValue           string        `json:"propertyValue"`
	DownPayment             string        `json:"downPayment"`
	BankFinancing           string        `json:"bankFinancing"`
	InitialOwnershipPercent string        `json:"initialOwnershipPercent"`
	MonthlyAcquisition      string        `json:"monthlyAcquisition"`
	FirstMonthPayment       string        `json:"firstMonthPayment"`
	TotalRent               string        `json:"totalRent"`
	TotalAcquisition        string        `json:"totalAcquisition"`
	TotalPaid               string        `json:"totalPaid"`
	TermMonths              int           `json:"termMonths"`
	Schedule                []dimMonthDTO `json:"schedule"`
}

func (h *Finance) DimMusharaka(w http.ResponseWriter, r *http.Request) {
	var req dimMusharakaRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := dimmusharaka.Input{TermMonths: req.TermMonths}
	in.PropertyValue = parseAmount(req.PropertyValue, "propertyValue", fields)
	if req.DownPayment != "" {
		in.DownPayment = parseAmount(req.DownPayment, "downPayment", fields)
	}
	in.AnnualRentalRate = parseAmount(req.AnnualRentalRate, "annualRentalRate", fields)
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid diminishing musharakah request", fields))
		return
	}

	res, err := h.svc.DimMusharaka(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	resp := dimMusharakaResponse{
		PropertyValue:           res.PropertyValue.StringFixed(2),
		DownPayment:             res.DownPayment.StringFixed(2),
		BankFinancing:           res.BankFinancing.StringFixed(2),
		InitialOwnershipPercent: res.InitialOwnershipPercent.StringFixed(4),
		MonthlyAcquisition:      res.MonthlyAcquisition.StringFixed(2),
		FirstMonthPayment:       res.FirstMonthPayment.StringFixed(2),
		TotalRent:               res.TotalRent.StringFixed(2),
		TotalAcquisition:        res.TotalAcquisition.StringFixed(2),
		TotalPaid:               res.TotalPaid.StringFixed(2),
		TermMonths:              res.TermMonths,
		Schedule:                make([]dimMonthDTO, len(res.Schedule)),
	}
	for i, m := range res.Schedule {
		resp.Schedule[i] = dimMonthDTO{
			N:                m.N,
			BankShareBefore:  m.BankShareBefore.StringFixed(2),
			Rent:             m.Rent.StringFixed(2),
			Acquisition:      m.Acquisition.StringFixed(2),
			Payment:          m.Payment.StringFixed(2),
			OwnershipPercent: m.OwnershipPercent.StringFixed(4),
		}
	}
	httpx.Data(w, http.StatusOK, resp)
}

// --- late payment charity --------------------------------------------------

type latePaymentRequest struct {
	Mode          string `json:"mode"`
	OverdueAmount string `json:"overdueAmount"`
	DaysLate      int    `json:"daysLate"`
	AnnualRate    string `json:"annualRate"`
	FlatFee       string `json:"flatFee"`
}

type latePaymentResponse struct {
	Mode          string `json:"mode"`
	OverdueAmount string `json:"overdueAmount"`
	DaysLate      int    `json:"daysLate"`
	CharityDue    string `json:"charityDue"`
	Disposition   string `json:"disposition"`
}

func (h *Finance) LatePayment(w http.ResponseWriter, r *http.Request) {
	var req latePaymentRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	fields := map[string]string{}
	in := latepayment.Input{Mode: req.Mode, DaysLate: req.DaysLate}
	in.OverdueAmount = parseAmount(req.OverdueAmount, "overdueAmount", fields)
	if req.AnnualRate != "" {
		in.AnnualRate = parseAmount(req.AnnualRate, "annualRate", fields)
	}
	if req.FlatFee != "" {
		in.FlatFee = parseAmount(req.FlatFee, "flatFee", fields)
	}
	if len(fields) > 0 {
		httpx.Err(w, apperr.Validation("invalid late payment request", fields))
		return
	}

	res, err := h.svc.LatePayment(r.Context(), in)
	if err != nil {
		httpx.Err(w, err)
		return
	}

	httpx.Data(w, http.StatusOK, latePaymentResponse{
		Mode:          res.Mode,
		OverdueAmount: res.OverdueAmount.StringFixed(2),
		DaysLate:      res.DaysLate,
		CharityDue:    res.CharityDue.StringFixed(2),
		Disposition:   res.Disposition,
	})
}

// parseAmount parses a decimal string, recording a field error instead of
// failing fast so the client sees every bad field at once.
func parseAmount(s, field string, fields map[string]string) decimal.Decimal {
	if s == "" {
		fields[field] = "required"
		return decimal.Zero
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		fields[field] = "must_be_decimal_string"
		return decimal.Zero
	}
	return d
}
