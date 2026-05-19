package seeds

import (
	"github.com/Caknoooo/go-gin-clean-starter/database/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SeedMetaMetricDict upserts all metric definitions into meta_metric_dict.
func SeedMetaMetricDict(db *gorm.DB) error {
	records := []entities.MetaMetricDict{
		// ---------- DAU ----------
		{MetricKey: "dau", MetricNameZH: "日活", Category: "dau", Unit: "count", Direction: "higher_better",
			Description: "当日活跃用户数（Daily Active Users），反映产品当日的用户规模"},
		{MetricKey: "dau_chain_ratio", MetricNameZH: "日活环比", Category: "dau", Unit: "ratio", Direction: "higher_better",
			Description: "日活相较前一日的变化比例，正值表示增长，负值表示下降"},
		{MetricKey: "old_user_dau", MetricNameZH: "老用户日活", Category: "dau", Unit: "count", Direction: "higher_better",
			Description: "注册超过一定天数的老用户当日活跃数，反映留存质量"},

		// ---------- 充值 ----------
		{MetricKey: "recharge_total_amt", MetricNameZH: "总充值", Category: "recharge", Unit: "amount", Direction: "higher_better",
			Description: "当日全渠道充值金额总和（元），是产品的核心变现指标"},
		{MetricKey: "recharge_total_chain_ratio", MetricNameZH: "总充值环比", Category: "recharge", Unit: "ratio", Direction: "higher_better",
			Description: "总充值相较前一日的变化比例"},
		{MetricKey: "recharge_organic_amt", MetricNameZH: "自然充值", Category: "recharge", Unit: "amount", Direction: "higher_better",
			Description: "通过自然流量（非付费渠道）带来的充值金额"},
		{MetricKey: "recharge_organic_chain_ratio", MetricNameZH: "自然充值环比", Category: "recharge", Unit: "ratio", Direction: "higher_better",
			Description: "自然充值相较前一日的变化比例"},
		{MetricKey: "recharge_channel_amt", MetricNameZH: "渠道充值", Category: "recharge", Unit: "amount", Direction: "higher_better",
			Description: "通过付费买量渠道带来的充值金额"},
		{MetricKey: "recharge_channel_chain_ratio", MetricNameZH: "渠道充值环比", Category: "recharge", Unit: "ratio", Direction: "higher_better",
			Description: "渠道充值相较前一日的变化比例"},
		{MetricKey: "recharge_internal_amt", MetricNameZH: "内导充值", Category: "recharge", Unit: "amount", Direction: "higher_better",
			Description: "通过内部流量导流（如站内推荐）带来的充值金额"},
		{MetricKey: "recharge_internal_chain_ratio", MetricNameZH: "内导充值环比", Category: "recharge", Unit: "ratio", Direction: "higher_better",
			Description: "内导充值相较前一日的变化比例"},

		// ---------- 新增用户 ----------
		{MetricKey: "new_user_total_cnt", MetricNameZH: "总新增", Category: "new_user", Unit: "count", Direction: "higher_better",
			Description: "当日全渠道新增注册用户数"},
		{MetricKey: "new_user_total_chain_ratio", MetricNameZH: "总新增环比", Category: "new_user", Unit: "ratio", Direction: "higher_better",
			Description: "总新增相较前一日的变化比例"},
		{MetricKey: "new_user_organic_cnt", MetricNameZH: "自然新增", Category: "new_user", Unit: "count", Direction: "higher_better",
			Description: "通过自然流量注册的新用户数"},
		{MetricKey: "new_user_organic_chain_ratio", MetricNameZH: "自然新增环比", Category: "new_user", Unit: "ratio", Direction: "higher_better",
			Description: "自然新增相较前一日的变化比例"},
		{MetricKey: "new_user_channel_cnt", MetricNameZH: "渠道新增", Category: "new_user", Unit: "count", Direction: "higher_better",
			Description: "通过付费渠道获取的新增用户数"},
		{MetricKey: "new_user_channel_chain_ratio", MetricNameZH: "渠道新增环比", Category: "new_user", Unit: "ratio", Direction: "higher_better",
			Description: "渠道新增相较前一日的变化比例"},
		{MetricKey: "new_user_internal_cnt", MetricNameZH: "内导新增", Category: "new_user", Unit: "count", Direction: "higher_better",
			Description: "通过内部导流获取的新增用户数"},
		{MetricKey: "new_user_internal_chain_ratio", MetricNameZH: "内导新增环比", Category: "new_user", Unit: "ratio", Direction: "higher_better",
			Description: "内导新增相较前一日的变化比例"},

		// ---------- 付费用户 ----------
		{MetricKey: "new_paying_user_cnt", MetricNameZH: "新充人数", Category: "paying", Unit: "count", Direction: "higher_better",
			Description: "当日首次充值的用户数，衡量新用户变现能力"},
		{MetricKey: "old_paying_user_cnt", MetricNameZH: "老充人数", Category: "paying", Unit: "count", Direction: "higher_better",
			Description: "当日再次充值的老用户数，反映老用户留存与复购"},
		{MetricKey: "new_paying_user_amt", MetricNameZH: "新用户充值", Category: "paying", Unit: "amount", Direction: "higher_better",
			Description: "当日新付费用户贡献的充值金额"},
		{MetricKey: "old_paying_user_amt", MetricNameZH: "老用户充值", Category: "paying", Unit: "amount", Direction: "higher_better",
			Description: "当日老付费用户贡献的充值金额"},
		{MetricKey: "recharge_order_cnt", MetricNameZH: "充值单数", Category: "paying", Unit: "count", Direction: "higher_better",
			Description: "当日充值订单总数"},
		{MetricKey: "payment_success_ratio", MetricNameZH: "付款成功率", Category: "paying", Unit: "ratio", Direction: "higher_better",
			Description: "充值订单中成功付款的比例，低于正常水平可能表明支付通道异常"},

		// ---------- 留存 ----------
		{MetricKey: "retention_d1_ratio", MetricNameZH: "次留率", Category: "retention", Unit: "ratio", Direction: "higher_better",
			Description: "新用户次日留存率（D1 Retention），衡量产品的初始吸引力"},
		{MetricKey: "retention_d3_ratio", MetricNameZH: "3留率", Category: "retention", Unit: "ratio", Direction: "higher_better",
			Description: "新用户3日留存率（D3 Retention）"},
		{MetricKey: "retention_d7_ratio", MetricNameZH: "7留率", Category: "retention", Unit: "ratio", Direction: "higher_better",
			Description: "新用户7日留存率（D7 Retention），是衡量产品长期粘性的关键指标"},

		// ---------- 落地页 ----------
		{MetricKey: "landing_page_visit_cnt", MetricNameZH: "下载页访问数", Category: "landing", Unit: "count", Direction: "higher_better",
			Description: "当日访问下载落地页的 UV 数"},
		{MetricKey: "landing_page_click_cnt", MetricNameZH: "下载页点击数", Category: "landing", Unit: "count", Direction: "higher_better",
			Description: "当日点击下载按钮的次数"},
		{MetricKey: "landing_page_download_ratio", MetricNameZH: "落地页下载率", Category: "landing", Unit: "ratio", Direction: "higher_better",
			Description: "下载页点击数/访问数，衡量落地页的转化效率"},

		// ---------- 转化 ----------
		{MetricKey: "conversion_total_multi", MetricNameZH: "总转化", Category: "conversion", Unit: "multiple", Direction: "higher_better",
			Description: "总充值相对新增用户的倍数，反映整体变现效率"},
		{MetricKey: "conversion_new_paying_multi", MetricNameZH: "新充转化", Category: "conversion", Unit: "multiple", Direction: "higher_better",
			Description: "新充人数/总新增，反映新用户中付费转化的比例（倍数形式）"},
		{MetricKey: "conversion_old_paying_multi", MetricNameZH: "老充转化", Category: "conversion", Unit: "multiple", Direction: "higher_better",
			Description: "老充人数/老用户日活，反映老用户的复购转化率（倍数形式）"},

		// ---------- ARPPU ----------
		{MetricKey: "arppu", MetricNameZH: "ARPPU", Category: "arppu", Unit: "amount", Direction: "higher_better",
			Description: "每付费用户平均收入（Average Revenue Per Paying User），衡量付费用户价值"},
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "metric_key"}},
		DoUpdates: clause.AssignmentColumns([]string{"metric_name_zh", "category", "unit", "direction", "description", "updated_at"}),
	}).Create(&records).Error
}
