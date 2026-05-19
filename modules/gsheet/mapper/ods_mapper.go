package mapper

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/dto"
)

// ExpectedHeaders 与 Sheet 表头严格对应（索引即列序）
var ExpectedHeaders = []string{
	"日期", "产品名称", "产品编号", "小组", "部门",
	"日活", "日活环比", "总充值", "总充值环比",
	"自然充值", "自然充值环比", "渠道充值", "渠道充值环比",
	"内导充值", "内导充值环比", "总新增", "总新增环比",
	"自然新增", "自然新增环比", "渠道新增", "渠道新增环比",
	"内导新增", "内导新增环比", "新充人数", "老充人数",
	"新用户充值", "老用户充值", "充值单数", "付款成功率",
	"次留率", "3留率", "7留率",
	"下载页访问数", "下载页点击数", "落地页下载率",
	"总转化", "新充转化", "老充转化", "老用户日活", "ARPPU",
}

// MapRow 把 CSV 一行（[]string）映射成 ODSRow。
// rowNo 是 1-based 数据行编号（不含表头）。
// productType: 'paid' | 'free'。
func MapRow(row []string, rowNo int, productType, source string) (dto.ODSRow, []dto.ParseWarning, error) {
	if len(row) < len(ExpectedHeaders) {
		return dto.ODSRow{}, nil, fmt.Errorf("row %d: got %d cols, need %d", rowNo, len(row), len(ExpectedHeaders))
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
		return dto.ODSRow{}, nil, fmt.Errorf("row %d: invalid date %q: %w", rowNo, row[0], err)
	}

	r := dto.ODSRow{
		Date:                  date,
		ProductName:           strings.TrimSpace(row[1]),
		ProductCode:           strings.TrimSpace(row[2]),
		Team:                  strings.TrimSpace(row[3]),
		BusinessUnit:          strings.TrimSpace(row[4]),
		DAU:                   parseU("日活", row[5]),
		DAUChain:              parseF("日活环比", row[6]),
		RechargeTotal:         parseF("总充值", row[7]),
		RechargeTotalChain:    parseF("总充值环比", row[8]),
		RechargeOrganic:       parseF("自然充值", row[9]),
		RechargeOrganicChain:  parseF("自然充值环比", row[10]),
		RechargeChannel:       parseF("渠道充值", row[11]),
		RechargeChannelChain:  parseF("渠道充值环比", row[12]),
		RechargeInternal:      parseF("内导充值", row[13]),
		RechargeInternalChain: parseF("内导充值环比", row[14]),
		NewUserTotal:          parseU("总新增", row[15]),
		NewUserTotalChain:     parseF("总新增环比", row[16]),
		NewUserOrganic:        parseU("自然新增", row[17]),
		NewUserOrganicChain:   parseF("自然新增环比", row[18]),
		NewUserChannel:        parseU("渠道新增", row[19]),
		NewUserChannelChain:   parseF("渠道新增环比", row[20]),
		NewUserInternal:       parseU("内导新增", row[21]),
		NewUserInternalChain:  parseF("内导新增环比", row[22]),
		NewPayingUserCnt:      parseU("新充人数", row[23]),
		OldPayingUserCnt:      parseU("老充人数", row[24]),
		NewPayingUserAmt:      parseF("新用户充值", row[25]),
		OldPayingUserAmt:      parseF("老用户充值", row[26]),
		RechargeOrderCnt:      parseU("充值单数", row[27]),
		PaymentSuccessRatio:   parseF("付款成功率", row[28]),
		RetentionD1:           parseF("次留率", row[29]),
		RetentionD3:           parseF("3留率", row[30]),
		RetentionD7:           parseF("7留率", row[31]),
		LandingPageVisit:      parseU("下载页访问数", row[32]),
		LandingPageClick:      parseU("下载页点击数", row[33]),
		LandingPageDownload:   parseF("落地页下载率", row[34]),
		ConversionTotal:       parseF("总转化", row[35]),
		ConversionNewPaying:   parseF("新充转化", row[36]),
		ConversionOldPaying:   parseF("老充转化", row[37]),
		OldUserDAU:            parseU("老用户日活", row[38]),
		ARPPU:                 parseF("ARPPU", row[39]),
		ProductType:           productType,
		Source:                source,
		SrcRowNo:              uint32(rowNo),
		IngestedAt:            time.Now(),
	}
	return r, warns, nil
}
