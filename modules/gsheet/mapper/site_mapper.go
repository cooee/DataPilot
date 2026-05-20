package mapper

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/dto"
)

// ExpectedSiteHeaders 站点产品 Sheet 表头（gid=553168897，11 列）。
var ExpectedSiteHeaders = []string{
	"日期", "产品名称", "产品编号", "小组", "部门",
	"日活跃数", "日活环比", "日导量新增", "日导量新增环比", "日导量充值", "日导量充值环比",
}

// MapSiteRow 把 CSV 一行映射为 SiteODSRow。
func MapSiteRow(row []string, rowNo int, source string) (dto.SiteODSRow, []dto.ParseWarning, error) {
	if len(row) < len(ExpectedSiteHeaders) {
		return dto.SiteODSRow{}, nil, fmt.Errorf("row %d: got %d cols, need %d", rowNo, len(row), len(ExpectedSiteHeaders))
	}

	var warns []dto.ParseWarning
	warn := func(field, raw, msg string) {
		warns = append(warns, dto.ParseWarning{RowNo: rowNo, Field: field, RawVal: raw, Message: msg})
	}

	parseF := func(field, s string) float64 {
		s = strings.TrimSpace(s)
		if s == "" || s == "-" {
			warn(field, s, "empty, defaulting to 0")
			return 0
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			warn(field, s, "not a number, defaulting to 0")
			return 0
		}
		return v
	}
	parseU := func(field, s string) uint64 {
		f := parseF(field, s)
		if f < 0 {
			warn(field, s, "negative uint, clamped to 0")
			return 0
		}
		return uint64(f)
	}

	date, err := time.Parse("2006-01-02", strings.TrimSpace(row[0]))
	if err != nil {
		return dto.SiteODSRow{}, nil, fmt.Errorf("row %d: invalid date %q: %w", rowNo, row[0], err)
	}

	r := dto.SiteODSRow{
		Date:              date,
		ProductName:       strings.TrimSpace(row[1]),
		ProductCode:       strings.TrimSpace(row[2]),
		Team:              strings.TrimSpace(row[3]),
		BusinessUnit:      strings.TrimSpace(row[4]),
		DAU:               parseU("日活跃数", row[5]),
		DAUChain:          parseF("日活环比", row[6]),
		LeadNewCnt:        parseU("日导量新增", row[7]),
		LeadNewChain:      parseF("日导量新增环比", row[8]),
		LeadRechargeAmt:   parseF("日导量充值", row[9]),
		LeadRechargeChain: parseF("日导量充值环比", row[10]),
		ProductType:       "site",
		Source:            source,
		SrcRowNo:          uint32(rowNo),
		IngestedAt:        time.Now(),
	}
	return r, warns, nil
}
