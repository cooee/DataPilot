package mapper_test

import (
	"testing"

	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/mapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeRow() []string {
	return []string{
		"2026-05-01", "海角乱伦", "JHA-031", "运营1组", "一部",
		"44702", "0.26753055", "116150", "2.78338762",
		"45450", "2.28158845", "112800", "2.96485062",
		"200", "0", "15578", "1.46370394",
		"5835", "2.34001145", "9667", "1.13682582",
		"3", "-0.625", "106", "538",
		"18950", "97200", "644", "0.67013528",
		"0.0863", "0.0451", "0.0226",
		"54744", "27775", "0.50736154",
		"7.456", "1.2165", "3.3375", "29124", "180.3571",
	}
}

func TestMapRow_HappyPath(t *testing.T) {
	r, warns, err := mapper.MapRow(makeRow(), 1, "paid", "gsheet:test")
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Equal(t, "JHA-031", r.ProductCode)
	assert.Equal(t, uint64(44702), r.DAU)
	assert.InDelta(t, 0.26753055, r.DAUChain, 1e-8)
	assert.InDelta(t, 180.3571, r.ARPPU, 1e-4)
	assert.Equal(t, "gsheet:test", r.Source)
	assert.Equal(t, uint32(1), r.SrcRowNo)
}

func TestMapRow_EmptyNumericField(t *testing.T) {
	row := makeRow()
	row[5] = "" // 日活 为空
	r, warns, err := mapper.MapRow(row, 2, "paid", "gsheet:test")
	require.NoError(t, err)
	assert.Equal(t, uint64(0), r.DAU)
	assert.NotEmpty(t, warns)
	assert.Contains(t, warns[0].Field, "日活")
}

func TestMapRow_InvalidDate(t *testing.T) {
	row := makeRow()
	row[0] = "not-a-date"
	_, _, err := mapper.MapRow(row, 3, "paid", "gsheet:test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date")
}

func TestMapRow_TooFewCols(t *testing.T) {
	row := []string{"2026-05-01", "产品", "JHA-001"}
	_, _, err := mapper.MapRow(row, 4, "paid", "gsheet:test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cols")
}

func TestMapRow_NegativeUint(t *testing.T) {
	row := makeRow()
	row[5] = "-100" // 日活 为负数
	r, warns, err := mapper.MapRow(row, 5, "paid", "gsheet:test")
	require.NoError(t, err)
	assert.Equal(t, uint64(0), r.DAU) // clamped to 0
	assert.NotEmpty(t, warns)
}
