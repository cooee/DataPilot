package dto

import "time"

// SiteODSRow 与 ods_site_product_daily 表 1:1 对应。
type SiteODSRow struct {
	Date                  time.Time
	ProductName           string
	ProductCode           string
	Team                  string
	BusinessUnit          string
	DAU                   uint64
	DAUChain              float64
	LeadNewCnt            uint64
	LeadNewChain          float64
	LeadRechargeAmt       float64
	LeadRechargeChain     float64
	ProductType           string // 固定 'site'
	Source                string
	SrcRowNo              uint32
	IngestedAt            time.Time
}
