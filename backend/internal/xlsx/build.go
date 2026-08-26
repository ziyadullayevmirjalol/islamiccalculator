package xlsx

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// fractionFields are the exact wire field names (input or output) that
// carry a fraction (0..1) rather than a human percent or plain amount —
// these get Excel's "0.00%" number format. This is an exact allowlist,
// not a substring guess, because the API is NOT consistent about it:
// e.g. istisna's "percent" and musharaka's "profitSharePercent" are
// already human 0..100 and must NOT be treated as fractions, while
// "rate", "capitalShare", "ownershipPercent" etc. are true fractions.
// Keep this in sync with the wire contract in openapi.yaml.
var fractionFields = map[string]bool{
	// outputs
	"rate": true, "effectiveAnnualRate": true, "marginRate": true,
	"profitRate": true, "currentYield": true, "portfolioCurrentYield": true,
	"purificationRatio": true, "ownershipPercent": true,
	"initialOwnershipPercent": true, "capitalShare": true, "appliedShare": true,
	"ratio": true, "threshold": true,
	// inputs (already converted client-side to fractions before the API call)
	"poolRateAnnual": true, "shareRatio": true, "wakalaFeeRate": true,
	"annualRentalRate": true, "annualRate": true, "distributionRateAnnual": true,
	"impureRatio": true,
}

// chrome holds the small set of structural strings the workbook itself
// needs, localized. Calculator field names stay in English — they are
// data-table headers, not user-interface copy.
var chrome = map[string]map[string]string{
	"uz": {
		"sheet_result": "Natija", "inputs": "Kiritilgan ma'lumotlar",
		"result": "Natija", "generated": "Yaratilgan", "yes": "Ha", "no": "Yo'q",
		"input_suffix": " (kiritilgan)",
	},
	"ru": {
		"sheet_result": "Результат", "inputs": "Введённые данные",
		"result": "Результат", "generated": "Создано", "yes": "Да", "no": "Нет",
		"input_suffix": " (ввод)",
	},
	"en": {
		"sheet_result": "Result", "inputs": "Inputs",
		"result": "Result", "generated": "Generated", "yes": "Yes", "no": "No",
		"input_suffix": " (input)",
	},
}

func chromeStr(lang, key string) string {
	m, ok := chrome[lang]
	if !ok {
		m = chrome["en"]
	}
	if v, ok := m[key]; ok {
		return v
	}
	return chrome["en"][key]
}

type pendingSheet struct {
	title string // humanized, pre-sanitization
	items []any  // []*Object
}

// Build produces the workbook for one calculation: a Result sheet with
// the inputs and the scalar/nested-object outputs as label/value rows,
// plus one additional sheet per array-of-objects field (schedules,
// shares, positions, checks, due animals, ...). No per-calculator code:
// it walks whatever shape the API returned.
func Build(title, generatedAt string, inputs, result *Object, lang string) (*excelize.File, error) {
	f := excelize.NewFile()
	const mainSheet = "Sheet1"
	if err := f.SetSheetName(mainSheet, chromeStr(lang, "sheet_result")); err != nil {
		return nil, err
	}
	sheet := chromeStr(lang, "sheet_result")

	titleStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}})
	captionStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Italic: true, Size: 9, Color: "666666"}})
	sectionStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 11}, Fill: excelize.Fill{Type: "pattern", Color: []string{"E9F2ED"}, Pattern: 1}})
	labelStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Color: "444444"}})
	headerStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Color: "FFFFFF"}, Fill: excelize.Fill{Type: "pattern", Color: []string{"2F7D5B"}, Pattern: 1}})

	// Number-format styles are cached: a 360-row schedule must not create
	// 360 near-identical style objects.
	numFmtStyles := map[string]int{}
	styleFor := func(numFmt string) int {
		if numFmt == "" {
			return 0
		}
		if id, ok := numFmtStyles[numFmt]; ok {
			return id
		}
		format := numFmt
		id, _ := f.NewStyle(&excelize.Style{CustomNumFmt: &format})
		numFmtStyles[numFmt] = id
		return id
	}

	row := 1
	setCell(f, sheet, 1, row, title, titleStyle)
	row++
	setCell(f, sheet, 1, row, fmt.Sprintf("%s · %s", chromeStr(lang, "generated"), generatedAt), captionStyle)
	row += 2

	var pending []pendingSheet

	if inputs != nil && len(inputs.Keys) > 0 {
		setCell(f, sheet, 1, row, strings.ToUpper(chromeStr(lang, "inputs")), sectionStyle)
		f.MergeCell(sheet, cellRef(1, row), cellRef(2, row))
		row++
		row = writeObject(f, sheet, row, inputs, "", labelStyle, lang, &pending, chromeStr(lang, "input_suffix"), styleFor)
		row++
	}

	setCell(f, sheet, 1, row, strings.ToUpper(chromeStr(lang, "result")), sectionStyle)
	f.MergeCell(sheet, cellRef(1, row), cellRef(2, row))
	row++
	if result != nil {
		row = writeObject(f, sheet, row, result, "", labelStyle, lang, &pending, "", styleFor)
	}

	_ = f.SetColWidth(sheet, "A", "A", 34)
	_ = f.SetColWidth(sheet, "B", "B", 22)

	// Defensive de-duplication: a sheet name collision would otherwise
	// have the second write silently land on and corrupt the first
	// sheet (excelize.NewSheet returns the existing index rather than
	// erroring on a repeated name). Every list sheet already gets an
	// input/output-disambiguating suffix from writeObject, so this only
	// guards against the unforeseen — it must never fire in practice.
	usedNames := map[string]bool{sheet: true}
	for i, ps := range pending {
		name := sanitizeSheetName(ps.title)
		for n, suffix := 2, name; usedNames[name]; n++ {
			name = sanitizeSheetName(fmt.Sprintf("%s %d", suffix, n))
		}
		usedNames[name] = true
		pending[i].title = name // writeListSheet re-sanitizes; already-unique name passes through untouched
		if err := writeListSheet(f, pending[i], headerStyle, styleFor); err != nil {
			return nil, err
		}
	}

	f.SetActiveSheet(0)
	return f, nil
}

// writeObject writes scalar fields as label/value rows on `sheet`
// starting at `row`, recurses into nested objects (dotted label
// prefix), and queues arrays-of-objects as separate sheets. Returns the
// next free row.
func writeObject(f *excelize.File, sheet string, row int, obj *Object, prefix string,
	labelStyle int, lang string, pending *[]pendingSheet, suffix string, styleFor func(string) int) int {

	for _, key := range obj.Keys {
		val := obj.Values[key]
		label := prefix + humanize(key)

		switch v := val.(type) {
		case *Object:
			// murabaha.markup / ijara.profit: the sibling "mode" field
			// decides whether "value" is a fraction or an amount — the
			// one genuinely context-dependent field in the whole API.
			if key == "markup" || key == "profit" {
				mode, _ := v.Values["mode"].(string)
				raw := v.Values["value"]
				fractionOverride := mode == "rate"
				setCell(f, sheet, 1, row, label+" "+humanize("mode")+suffix, labelStyle)
				setCell(f, sheet, 2, row, mode, 0)
				row++
				cellVal, numFmt := formatValue(raw, "", fractionOverride)
				writeValueRow(f, sheet, &row, label+" "+humanize("value")+suffix, cellVal, numFmt, labelStyle, styleFor)
				continue
			}
			row = writeObject(f, sheet, row, v, label+" ", labelStyle, lang, pending, suffix, styleFor)
		case []any:
			if len(v) == 0 {
				continue
			}
			if _, ok := v[0].(*Object); ok {
				// The input suffix (e.g. " (input)") disambiguates from a
				// same-named result array — sukuk's "positions" appears
				// in both inputs and results, and without this the two
				// list sheets would collide under one name and the
				// second write would silently overwrite the first.
				*pending = append(*pending, pendingSheet{title: label + suffix, items: v})
				continue
			}
			parts := make([]string, 0, len(v))
			for _, s := range v {
				parts = append(parts, fmt.Sprint(s))
			}
			setCell(f, sheet, 1, row, label+suffix, labelStyle)
			setCell(f, sheet, 2, row, strings.Join(parts, ", "), 0)
			row++
		default:
			cellVal, numFmt := formatValue(val, key, fractionFields[key])
			if b, ok := val.(bool); ok {
				cellVal = boolLabel(b, lang)
			}
			writeValueRow(f, sheet, &row, label+suffix, cellVal, numFmt, labelStyle, styleFor)
		}
	}
	return row
}

func writeValueRow(f *excelize.File, sheet string, row *int, label string, val any, numFmt string, labelStyle int, styleFor func(string) int) {
	setCell(f, sheet, 1, *row, label, labelStyle)
	ref := cellRef(2, *row)
	_ = f.SetCellValue(sheet, ref, val)
	if style := styleFor(numFmt); style != 0 {
		_ = f.SetCellStyle(sheet, ref, ref, style)
	}
	*row++
}

func writeListSheet(f *excelize.File, ps pendingSheet, headerStyle int, styleFor func(string) int) error {
	name := sanitizeSheetName(ps.title)
	if _, err := f.NewSheet(name); err != nil {
		return err
	}
	first, ok := ps.items[0].(*Object)
	if !ok {
		return nil
	}
	for i, key := range first.Keys {
		ref := cellRef(i+1, 1)
		_ = f.SetCellValue(name, ref, humanize(key))
		_ = f.SetCellStyle(name, ref, ref, headerStyle)
	}
	for r, item := range ps.items {
		obj, ok := item.(*Object)
		if !ok {
			continue
		}
		for i, key := range first.Keys {
			cellVal, numFmt := formatValue(obj.Values[key], key, fractionFields[key])
			ref := cellRef(i+1, r+2)
			_ = f.SetCellValue(name, ref, cellVal)
			if style := styleFor(numFmt); style != 0 {
				_ = f.SetCellStyle(name, ref, ref, style)
			}
		}
	}
	lastCol, _ := excelize.ColumnNumberToName(len(first.Keys))
	_ = f.SetColWidth(name, "A", lastCol, 16)
	return nil
}

var decimalPattern = regexp.MustCompile(`^-?\d+(\.\d+)?$`)

// formatValue converts a parsed JSON value into an Excel cell value plus
// the number format to apply, if any.
func formatValue(val any, key string, isFraction bool) (any, string) {
	switch v := val.(type) {
	case nil:
		return "", ""
	case bool:
		return v, "" // caller overrides with a localized label
	case string:
		if decimalPattern.MatchString(v) {
			f, err := strconv.ParseFloat(v, 64)
			if err == nil {
				if isFraction {
					return f, "0.00%"
				}
				if f == float64(int64(f)) {
					return f, "#,##0"
				}
				return f, "#,##0.00"
			}
		}
		return strings.ReplaceAll(v, "_", " "), ""
	default:
		// json.Number for plain integers (termMonths, daysLate, count, n...)
		s := fmt.Sprint(v)
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			if isFraction {
				return f, "0.00%"
			}
			return f, "#,##0"
		}
		return s, ""
	}
}

func boolLabel(b bool, lang string) string {
	if b {
		return chromeStr(lang, "yes")
	}
	return chromeStr(lang, "no")
}

var camelBoundary = regexp.MustCompile(`([a-z0-9])([A-Z])`)

// humanize turns an API field name into a readable column/row label:
// "monthlyInstallment" -> "Monthly Installment".
func humanize(key string) string {
	if key == "" {
		return key
	}
	spaced := camelBoundary.ReplaceAllString(key, "$1 $2")
	spaced = strings.ReplaceAll(spaced, "_", " ")
	words := strings.Fields(spaced)
	for i, w := range words {
		if len(w) == 0 {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

var invalidSheetChars = regexp.MustCompile(`[\[\]\*\?/\\:]`)

func sanitizeSheetName(name string) string {
	name = invalidSheetChars.ReplaceAllString(name, " ")
	name = strings.TrimSpace(name)
	if len(name) > 31 {
		name = name[:31]
	}
	if name == "" {
		name = "Sheet"
	}
	return name
}

func cellRef(col, row int) string {
	ref, _ := excelize.CoordinatesToCellName(col, row)
	return ref
}

func setCell(f *excelize.File, sheet string, col, row int, val any, style int) {
	ref := cellRef(col, row)
	_ = f.SetCellValue(sheet, ref, val)
	if style != 0 {
		_ = f.SetCellStyle(sheet, ref, ref, style)
	}
}
