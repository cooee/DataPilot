package catalog

// AggFunc 指标聚合方式（GROUP BY 时使用）。
type AggFunc string

const (
	AggSum  AggFunc = "sum"
	AggAvg  AggFunc = "avg"
	AggMax  AggFunc = "max"
	AggMin  AggFunc = "min"
	AggNone AggFunc = "none" // 已预聚合的 ADS 视图字段，直接 SELECT
)

// FieldKind 字段类型，用于响应列元数据。
type FieldKind string

const (
	KindDate    FieldKind = "date"
	KindString  FieldKind = "string"
	KindNumber  FieldKind = "number"
	KindUnknown FieldKind = "unknown"
)

type DimensionDef struct {
	Kind      FieldKind
	MetricKey string // 可选，关联 meta_metric_dict
}

type MetricDef struct {
	AggFunc   AggFunc
	Kind      FieldKind
	MetricKey string // 关联 meta_metric_dict，供 AI 理解语义
}

type DatasetDef struct {
	Name        string
	Label       string
	Description string
	Table       string // ClickHouse 表/视图名（白名单）
	DateColumn  string // 日期过滤列，空表示无日期维度
	Dimensions  map[string]DimensionDef
	Metrics     map[string]MetricDef
}

// Datasets 可查询数据集白名单（语义层入口，禁止客户端拼 SQL）。
var Datasets = map[string]DatasetDef{
	"dws_product_daily": {
		Name:        "dws_product_daily",
		Label:       "产品日汇总",
		Description: "DWS 层产品粒度日指标，支持按日期/产品/小组/类型聚合",
		Table:       "dws_product_daily",
		DateColumn:  "date",
		Dimensions: map[string]DimensionDef{
			"date":          {Kind: KindDate},
			"product_type":  {Kind: KindString},
			"product_code":  {Kind: KindString},
			"product_name":  {Kind: KindString},
			"team":          {Kind: KindString},
			"business_unit": {Kind: KindString},
		},
		Metrics: productDailyMetrics(),
	},
	"dws_team_daily": {
		Name:        "dws_team_daily",
		Label:       "小组日汇总",
		Description: "DWS 层小组粒度日指标",
		Table:       "dws_team_daily",
		DateColumn:  "date",
		Dimensions: map[string]DimensionDef{
			"date":          {Kind: KindDate},
			"team":          {Kind: KindString},
			"business_unit": {Kind: KindString},
		},
		Metrics: map[string]MetricDef{
			"active_product_cnt":     {AggFunc: AggSum, Kind: KindNumber},
			"dau_sum":                {AggFunc: AggSum, Kind: KindNumber, MetricKey: "dau"},
			"recharge_total_amt_sum": {AggFunc: AggSum, Kind: KindNumber, MetricKey: "recharge_total_amt"},
			"new_paying_user_cnt_sum": {AggFunc: AggSum, Kind: KindNumber, MetricKey: "new_paying_user_cnt"},
			"recharge_order_cnt_sum": {AggFunc: AggSum, Kind: KindNumber, MetricKey: "recharge_order_cnt"},
			"old_user_dau_sum":       {AggFunc: AggSum, Kind: KindNumber, MetricKey: "old_user_dau"},
		},
	},
	"ads_product_health_overview": {
		Name:        "ads_product_health_overview",
		Label:       "产品健康度概览",
		Description: "ADS 视图：近 30 日产品健康分与核心指标均值",
		Table:       "ads_product_health_overview",
		DateColumn:  "",
		Dimensions: map[string]DimensionDef{
			"product_code":  {Kind: KindString},
			"product_name":  {Kind: KindString},
			"team":          {Kind: KindString},
			"business_unit": {Kind: KindString},
		},
		Metrics: map[string]MetricDef{
			"avg_dau_30d":            {AggFunc: AggNone, Kind: KindNumber, MetricKey: "dau"},
			"recharge_total_30d":     {AggFunc: AggNone, Kind: KindNumber, MetricKey: "recharge_total_amt"},
			"avg_retention_d7_30d":   {AggFunc: AggNone, Kind: KindNumber, MetricKey: "retention_d7_ratio"},
			"avg_payment_success_30d": {AggFunc: AggNone, Kind: KindNumber, MetricKey: "payment_success_ratio"},
			"avg_arppu_30d":          {AggFunc: AggNone, Kind: KindNumber, MetricKey: "arppu"},
			"health_score":           {AggFunc: AggNone, Kind: KindNumber},
		},
	},
	"ads_team_performance_daily": {
		Name:        "ads_team_performance_daily",
		Label:       "小组日表现",
		Description: "ADS 视图：小组日维度核心指标",
		Table:       "ads_team_performance_daily",
		DateColumn:  "date",
		Dimensions: map[string]DimensionDef{
			"date":          {Kind: KindDate},
			"team":          {Kind: KindString},
			"business_unit": {Kind: KindString},
		},
		Metrics: map[string]MetricDef{
			"active_product_cnt":        {AggFunc: AggNone, Kind: KindNumber},
			"dau_sum":                   {AggFunc: AggNone, Kind: KindNumber, MetricKey: "dau"},
			"recharge_total_amt_sum":    {AggFunc: AggNone, Kind: KindNumber, MetricKey: "recharge_total_amt"},
			"new_paying_user_cnt_sum":   {AggFunc: AggNone, Kind: KindNumber, MetricKey: "new_paying_user_cnt"},
			"recharge_order_cnt_sum":    {AggFunc: AggNone, Kind: KindNumber, MetricKey: "recharge_order_cnt"},
			"retention_d7_ratio_wavg":   {AggFunc: AggNone, Kind: KindNumber, MetricKey: "retention_d7_ratio"},
		},
	},
}

func GetDataset(name string) (DatasetDef, bool) {
	ds, ok := Datasets[name]
	return ds, ok
}

func productDailyMetrics() map[string]MetricDef {
	keys := []struct {
		key   string
		agg   AggFunc
		metric string
	}{
		{"dau", AggSum, "dau"},
		{"dau_chain_ratio", AggAvg, "dau_chain_ratio"},
		{"old_user_dau", AggSum, "old_user_dau"},
		{"recharge_total_amt", AggSum, "recharge_total_amt"},
		{"recharge_total_chain_ratio", AggAvg, "recharge_total_chain_ratio"},
		{"recharge_organic_amt", AggSum, "recharge_organic_amt"},
		{"recharge_channel_amt", AggSum, "recharge_channel_amt"},
		{"recharge_internal_amt", AggSum, "recharge_internal_amt"},
		{"new_user_total_cnt", AggSum, "new_user_total_cnt"},
		{"new_paying_user_cnt", AggSum, "new_paying_user_cnt"},
		{"old_paying_user_cnt", AggSum, "old_paying_user_cnt"},
		{"recharge_order_cnt", AggSum, "recharge_order_cnt"},
		{"payment_success_ratio", AggAvg, "payment_success_ratio"},
		{"retention_d1_ratio", AggAvg, "retention_d1_ratio"},
		{"retention_d3_ratio", AggAvg, "retention_d3_ratio"},
		{"retention_d7_ratio", AggAvg, "retention_d7_ratio"},
		{"landing_page_visit_cnt", AggSum, "landing_page_visit_cnt"},
		{"landing_page_click_cnt", AggSum, "landing_page_click_cnt"},
		{"landing_page_download_ratio", AggAvg, "landing_page_download_ratio"},
		{"arppu", AggAvg, "arppu"},
	}
	m := make(map[string]MetricDef, len(keys))
	for _, k := range keys {
		m[k.key] = MetricDef{AggFunc: k.agg, Kind: KindNumber, MetricKey: k.metric}
	}
	return m
}
