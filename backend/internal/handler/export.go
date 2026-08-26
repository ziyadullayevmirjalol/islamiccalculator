package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
	"github.com/diyorbek/islamiccalculator/internal/pkg/httpx"
	"github.com/diyorbek/islamiccalculator/internal/xlsx"
)

// calcTypeToPath maps every calculator's history calc_type (also the
// identifier clients already send) to its own endpoint path. Adding a
// 19th calculator means adding one line here — export itself needs no
// other change, since it re-runs the real endpoint and walks whatever
// JSON it returns.
var calcTypeToPath = map[string]string{
	"finance.murabaha":              "/api/v1/finance/murabaha",
	"finance.ijara":                 "/api/v1/finance/ijara",
	"finance.qard_hasan":            "/api/v1/finance/qard-hasan",
	"finance.mudaraba":              "/api/v1/finance/mudaraba",
	"finance.diminishing_musharaka": "/api/v1/finance/diminishing-musharaka",
	"finance.salam":                 "/api/v1/finance/salam",
	"finance.istisna":               "/api/v1/finance/istisna",
	"finance.musharaka":             "/api/v1/finance/musharaka",
	"finance.late_payment":          "/api/v1/finance/late-payment",
	"zakat.wealth":                  "/api/v1/zakat/wealth",
	"zakat.business":                "/api/v1/zakat/business",
	"zakat.ushr":                    "/api/v1/zakat/ushr",
	"zakat.livestock":               "/api/v1/zakat/livestock",
	"zakat.fidya":                   "/api/v1/zakat/fidya",
	"zakat.fitrah":                  "/api/v1/zakat/fitrah",
	"zakat.tazkiya":                 "/api/v1/zakat/tazkiya",
	"invest.screener":               "/api/v1/invest/screener",
	"invest.sukuk":                  "/api/v1/invest/sukuk",
}

type calcTitle struct{ uz, ru, en string }

var calcTitles = map[string]calcTitle{
	"finance.murabaha":              {"Murobaha", "Мурабаха", "Murabaha"},
	"finance.ijara":                 {"Ijara Muntahiya Bittamlik", "Иджара Мунтахия Биттамлик", "Ijara Muntahia Bittamleek"},
	"finance.qard_hasan":            {"Qarzi Hasan", "Кард аль-Хасан", "Qard al-Hasan"},
	"finance.mudaraba":              {"Mudoraba / Vakola omonati", "Мудараба / Вакала депозит", "Mudaraba / Wakala deposit"},
	"finance.diminishing_musharaka": {"Kamayuvchi Mushoraka", "Убывающая Мушарака", "Diminishing Musharakah"},
	"finance.salam":                 {"Salam", "Салям", "Salam"},
	"finance.istisna":               {"Istisna", "Истисна", "Istisna"},
	"finance.musharaka":             {"Mushoraka", "Мушарака", "Musharaka"},
	"finance.late_payment":          {"Kechikish xayriyasi", "Благотворительный штраф", "Late-payment charity"},
	"zakat.wealth":                  {"Oltin, kumush va naqd pul zakoti", "Закят с золота, серебра и денег", "Gold, silver & cash zakat"},
	"zakat.business":                {"Biznes zakoti", "Закят с бизнеса", "Business zakat"},
	"zakat.ushr":                    {"Ushr (hosil zakoti)", "Ушр (закят с урожая)", "Ushr (harvest zakat)"},
	"zakat.livestock":               {"Chorva va pilla zakoti", "Закят со скота и коконов", "Livestock & silk-cocoon zakat"},
	"zakat.fidya":                   {"Fidya va Kaffora", "Фидия и Каффара", "Fidya & Kaffarah"},
	"zakat.fitrah":                  {"Fitr zakoti", "Закят аль-Фитр", "Zakat al-Fitr"},
	"zakat.tazkiya":                 {"Tazkiya (daromadni poklash)", "Тазкия (очищение дохода)", "Tazkiya (income purification)"},
	"invest.screener":               {"Halol aksiyalar skrineri", "Скрининг халяльных акций", "Halal stock screener"},
	"invest.sukuk":                  {"Sukuk portfeli", "Портфель сукук", "Sukuk portfolio"},
}

func (t calcTitle) forLang(lang string) string {
	switch lang {
	case "ru":
		return t.ru
	case "en":
		return t.en
	default:
		return t.uz
	}
}

type exportRequest struct {
	CalcType string          `json:"calcType"`
	Inputs   json.RawMessage `json:"inputs"`
	Lang     string          `json:"lang"`
}

// Export re-runs a calculation through the real calculator endpoint —
// in-process, via the same router — and turns its response into an
// .xlsx workbook. Re-running (rather than trusting a client-supplied
// result) guarantees the exported numbers are always genuine calculator
// output: nobody can hand a fabricated result and get an
// official-looking spreadsheet out of it.
type Export struct {
	router http.Handler
}

func NewExport(router http.Handler) *Export {
	return &Export{router: router}
}

func (h *Export) Handle(w http.ResponseWriter, r *http.Request) {
	var req exportRequest
	if err := httpx.Decode(r, &req); err != nil {
		httpx.Err(w, err)
		return
	}

	path, ok := calcTypeToPath[req.CalcType]
	if !ok {
		httpx.Err(w, apperr.Validation("invalid export request",
			map[string]string{"calcType": "unknown_calc_type"}))
		return
	}
	lang := req.Lang
	if lang != "ru" && lang != "en" {
		lang = "uz"
	}
	if len(req.Inputs) == 0 {
		req.Inputs = json.RawMessage("{}")
	}

	// Deliberately anonymous: only Content-Type is forwarded, never the
	// caller's Authorization header. Exporting a result must never write
	// a second, surprising entry into the user's calculation history.
	subReq := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(req.Inputs))
	subReq.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, subReq)

	if rec.Code != http.StatusOK {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(rec.Code)
		_, _ = w.Write(rec.Body.Bytes())
		return
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		httpx.Err(w, apperr.Internal("parse calculation result", err))
		return
	}

	resultObj, err := xlsx.ParseObject(envelope.Data)
	if err != nil {
		httpx.Err(w, apperr.Internal("parse calculation result", err))
		return
	}
	inputsObj, err := xlsx.ParseObject(req.Inputs)
	if err != nil {
		inputsObj = nil // malformed inputs already surfaced as the calc's own error above
	}

	title := calcTitles[req.CalcType].forLang(lang)
	if title == "" {
		title = req.CalcType
	}
	generatedAt := time.Now().UTC().Format("2006-01-02 15:04") + " UTC"

	file, err := xlsx.Build(title, generatedAt, inputsObj, resultObj, lang)
	if err != nil {
		httpx.Err(w, apperr.Internal("build workbook", err))
		return
	}

	var buf bytes.Buffer
	if _, err := file.WriteTo(&buf); err != nil {
		httpx.Err(w, apperr.Internal("write workbook", err))
		return
	}

	slug := strings.ReplaceAll(strings.SplitN(req.CalcType, ".", 2)[1], "_", "-")
	filename := fmt.Sprintf("%s-%s.xlsx", slug, time.Now().UTC().Format("2006-01-02"))

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprint(buf.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(buf.Bytes())
}
