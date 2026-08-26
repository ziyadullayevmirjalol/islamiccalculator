package xlsx

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestParseObject_PreservesKeyOrder(t *testing.T) {
	obj, err := ParseObject(json.RawMessage(`{"n":1,"dueDate":"2026-10-01","amount":"11000000.00","balance":"0.00"}`))
	require.NoError(t, err)
	assert.Equal(t, []string{"n", "dueDate", "amount", "balance"}, obj.Keys)
}

func TestParseObject_NestedAndArrays(t *testing.T) {
	obj, err := ParseObject(json.RawMessage(`{
		"markup": {"mode": "rate", "value": "0.2"},
		"schedule": [{"n": 1, "amount": "800000.00"}, {"n": 2, "amount": "800000.00"}]
	}`))
	require.NoError(t, err)

	markup, ok := obj.Values["markup"].(*Object)
	require.True(t, ok)
	assert.Equal(t, "rate", markup.Values["mode"])

	schedule, ok := obj.Values["schedule"].([]any)
	require.True(t, ok)
	require.Len(t, schedule, 2)
	row0, ok := schedule[0].(*Object)
	require.True(t, ok)
	assert.Equal(t, []string{"n", "amount"}, row0.Keys)
}

// Build must produce a workbook that a spreadsheet application can open,
// with the schedule as its own sheet in the API's own column order and
// money values as real numbers (so a user can SUM them), not text.
func TestBuild_MurabahaGoldenCase(t *testing.T) {
	inputs, err := ParseObject(json.RawMessage(`{
		"cost": "8000000", "markup": {"mode": "rate", "value": "0.2"}, "termMonths": 12
	}`))
	require.NoError(t, err)

	result, err := ParseObject(json.RawMessage(`{
		"cost": "8000000.00", "markupTotal": "1600000.00", "salePrice": "9600000.00",
		"downPayment": "0.00", "financed": "9600000.00", "monthlyInstallment": "800000.00",
		"termMonths": 12,
		"schedule": [
			{"n": 1, "dueDate": "2026-10-01", "amount": "800000.00", "principal": "666666.66", "markup": "133333.34", "balance": "8800000.00"},
			{"n": 2, "dueDate": "2026-11-01", "amount": "800000.00", "principal": "666666.67", "markup": "133333.33", "balance": "8000000.00"}
		]
	}`))
	require.NoError(t, err)

	f, err := Build("Murabaha", "2026-08-26 12:00 UTC", inputs, result, "en")
	require.NoError(t, err)

	sheets := f.GetSheetList()
	assert.Contains(t, sheets, "Result")
	assert.Contains(t, sheets, "Schedule")

	// Round-trip through the actual xlsx binary format, exactly as a
	// real spreadsheet app would read the downloaded file.
	buf, err := f.WriteToBuffer()
	require.NoError(t, err)
	reopened, err := excelize.OpenReader(buf)
	require.NoError(t, err)
	defer reopened.Close()

	rows, err := reopened.GetRows("Schedule")
	require.NoError(t, err)
	require.Len(t, rows, 3) // header + 2 schedule rows
	assert.Equal(t, []string{"N", "Due Date", "Amount", "Principal", "Markup", "Balance"}, rows[0])
	// "800,000" (with the thousands separator) is the displayed text of a
	// real numeric cell under the "#,##0" format — proof it's a number,
	// not a text cell, which is what makes it SUM-able in a spreadsheet.
	assert.Equal(t, "800,000", rows[1][2], "amount must be a real, formatted number cell")

	resultRows, err := reopened.GetRows("Result")
	require.NoError(t, err)
	found := false
	for _, r := range resultRows {
		if len(r) >= 2 && r[0] == "Monthly Installment" {
			assert.Equal(t, "800,000", r[1])
			found = true
		}
	}
	assert.True(t, found, "Monthly Installment row must be present")
}

func TestBuild_FractionFieldsFormatAsPercent(t *testing.T) {
	result, err := ParseObject(json.RawMessage(`{
		"effectiveAnnualRate": "0.1080", "expectedProfit": "1080000.00", "guaranteed": false
	}`))
	require.NoError(t, err)

	f, err := Build("Mudaraba", "2026-08-26", nil, result, "en")
	require.NoError(t, err)

	buf, err := f.WriteToBuffer()
	require.NoError(t, err)
	reopened, err := excelize.OpenReader(buf)
	require.NoError(t, err)
	defer reopened.Close()

	rows, err := reopened.GetRows("Result")
	require.NoError(t, err)
	var rateVal, guaranteedVal string
	for _, r := range rows {
		if len(r) >= 2 && r[0] == "Effective Annual Rate" {
			rateVal = r[1]
		}
		if len(r) >= 2 && r[0] == "Guaranteed" {
			guaranteedVal = r[1]
		}
	}
	// Excel's native percentage semantics: the cell stores the fraction
	// 0.108 and the "0.00%" format displays it as "10.80%" — exactly
	// like every percentage cell in every spreadsheet.
	assert.Equal(t, "10.80%", rateVal, "0.1080 stored as a fraction, displayed as a percent")
	assert.Equal(t, "No", guaranteedVal, "booleans render as localized Yes/No, not raw JSON true/false")
}

// Regression test: sukuk's "positions" array appears in BOTH inputs and
// results. Without disambiguation the second writeListSheet call lands
// on the same sheet name and silently overwrites the first.
func TestBuild_InputAndResultListsWithTheSameKeyDoNotCollide(t *testing.T) {
	inputs, err := ParseObject(json.RawMessage(`{
		"positions": [{"name": "A", "faceValue": "100"}]
	}`))
	require.NoError(t, err)
	result, err := ParseObject(json.RawMessage(`{
		"positions": [{"name": "A", "faceValue": "100", "currentYield": "0.09"}],
		"totalInvested": "100"
	}`))
	require.NoError(t, err)

	f, err := Build("Sukuk", "2026-08-26", inputs, result, "en")
	require.NoError(t, err)

	sheets := f.GetSheetList()
	assert.Contains(t, sheets, "Positions (input)")
	assert.Contains(t, sheets, "Positions")

	inputRows, err := f.GetRows("Positions (input)")
	require.NoError(t, err)
	resultRows, err := f.GetRows("Positions")
	require.NoError(t, err)

	assert.Equal(t, []string{"Name", "Face Value"}, inputRows[0],
		"the input sheet keeps only what was entered")
	assert.Equal(t, []string{"Name", "Face Value", "Current Yield"}, resultRows[0],
		"the result sheet is untouched by the input write")
}

func TestSanitizeSheetName_HandlesLongAndInvalidNames(t *testing.T) {
	assert.LessOrEqual(t, len(sanitizeSheetName("A Very Long Field Name That Exceeds Excel's Thirty One Character Limit")), 31)
	assert.NotContains(t, sanitizeSheetName("weird/name:with*bad?chars"), "/")
}

func TestHumanize(t *testing.T) {
	cases := map[string]string{
		"monthlyInstallment":           "Monthly Installment",
		"n":                            "N",
		"ownershipPercent":             "Ownership Percent",
		"computed_by_combination_rule": "Computed By Combination Rule",
	}
	for in, want := range cases {
		assert.Equal(t, want, humanize(in), "humanize(%q)", in)
	}
}
