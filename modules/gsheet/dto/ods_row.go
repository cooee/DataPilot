package dto

import "time"

// ODSRow 与 ods_product_daily_report 表 1:1 对应
type ODSRow struct {
	Date                  time.Time
	ProductName           string
	ProductCode           string
	Team                  string
	BusinessUnit          string
	DAU                   uint64
	DAUChain              float64
	RechargeTotal         float64
	RechargeTotalChain    float64
	RechargeOrganic       float64
	RechargeOrganicChain  float64
	RechargeChannel       float64
	RechargeChannelChain  float64
	RechargeInternal      float64
	RechargeInternalChain float64
	NewUserTotal          uint64
	NewUserTotalChain     float64
	NewUserOrganic        uint64
	NewUserOrganicChain   float64
	NewUserChannel        uint64
	NewUserChannelChain   float64
	NewUserInternal       uint64
	NewUserInternalChain  float64
	NewPayingUserCnt      uint64
	OldPayingUserCnt      uint64
	NewPayingUserAmt      float64
	OldPayingUserAmt      float64
	RechargeOrderCnt      uint64
	PaymentSuccessRatio   float64
	RetentionD1           float64
	RetentionD3           float64
	RetentionD7           float64
	LandingPageVisit      uint64
	LandingPageClick      uint64
	LandingPageDownload   float64
	ConversionTotal       float64
	ConversionNewPaying   float64
	ConversionOldPaying   float64
	OldUserDAU            uint64
	ARPPU                 float64

	// 元数据
	ProductType string // 'paid' | 'free'
	Source      string
	SrcRowNo    uint32
	IngestedAt  time.Time
}
