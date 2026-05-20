package report

// DailyReportHTMLTitle 页面主标题（<title> 与 <h1> 均使用，禁止 LLM 自造变体）。
const DailyReportHTMLTitle = "DataPilot 每日运营分析报告"

// DailyReportPayload 每日运营 HTML 报表数据。
type DailyReportPayload struct {
	HTMLTitle    string // 固定 DailyReportHTMLTitle，供 LLM 读取
	ReportDate   string
	SyncFrom     string
	SyncTo       string
	GeneratedAt  string
	DQOK         bool
	DQSummary    string
	Compare      []TypeCompareRow
	TeamDaily    []TeamRow
	HealthTop    []HealthRow
	ProductDaily []ProductDayRow
	Anomalies    []AnomalyRow
	TrendHistory []TrendPoint
	Forecast7d   []ForecastPoint // 兼容旧字段
	Insights     []string
	Segments          []SegmentReport // paid, free, site
	SiteSummary       SiteSummaryRow
	SiteProductDaily  []SiteProductDayRow
	SiteTrendHistory  []SiteTrendPoint
}

// SiteSummaryRow 站点产品报告日汇总。
type SiteSummaryRow struct {
	DAU             float64
	LeadNewCnt      float64
	LeadRechargeAmt float64
	LeadNewWoW      float64
}

// SiteProductDayRow 站点产品日明细。
type SiteProductDayRow struct {
	ProductCode     string
	ProductName     string
	Team            string
	DAU             float64
	LeadNewCnt      float64
	LeadRechargeAmt float64
}

// SiteTrendPoint 站点产品按日聚合趋势。
type SiteTrendPoint struct {
	Date            string
	DAU             float64
	LeadNewCnt      float64
	LeadRechargeAmt float64
}

type TypeCompareRow struct {
	ProductType string
	DAU         float64
	Recharge    float64
	RetentionD7 float64
	ARPPU       float64
	NewUsers    float64
	NewPaying   float64
}

type TeamRow struct {
	Date        string
	Team        string
	DAU         float64
	Recharge    float64
	RetentionD7 float64
}

type HealthRow struct {
	ProductCode string
	ProductName string
	Team        string
	HealthScore float64
	AvgDAU      float64
	Recharge30d float64
}

type ProductDayRow struct {
	ProductCode string
	ProductName string
	Team        string
	ProductType string
	DAU         float64
	Recharge    float64
	RetentionD7 float64
	ARPPU       float64
	NewUsers    float64
	NewPaying   float64
}

type AnomalyRow struct {
	ProductCode string
	ProductName string
	Team        string
	ProductType string
	Metric      string
	Value       float64
	ZScore      float64
	Direction   string
}

type TrendPoint struct {
	Date            string
	ProductType     string
	DAU             float64
	Recharge        float64
	RetentionD7     float64
	ARPPU           float64
	NewUsers        float64
	NewPaying       float64
	LeadNewCnt      float64 // site 产品线
	LeadRechargeAmt float64
}

type ForecastPoint struct {
	Date        string
	ProductType string
	DAU         float64
	Recharge    float64
	Method      string
}

// SegmentReport 单条产品线（付费/免费）分析视图。
type SegmentReport struct {
	Key          string // paid | free
	Label        string
	Focus        string // 分析侧重点说明
	Snapshot     SegmentSnapshot
	TrendRecent  []TrendPoint
	Forecasts    []MetricForecast
	Anomalies    []AnomalyRow
	TopProducts     []ProductDayRow
	SiteTrendRecent []SiteTrendPoint // 仅 site 分栏
	SiteTopProducts []SiteProductDayRow
	AnalystNotes    []string
	RiskLevel       string // low | medium | high
}

type SegmentSnapshot struct {
	DAU             float64
	Recharge        float64
	RetentionD7     float64
	ARPPU           float64
	NewUsers        float64
	NewPaying       float64
	LeadNewCnt      float64 // 站点：日导量新增
	LeadRechargeAmt float64 // 站点：日导量充值
	DAUWoW          float64 // 环比%，基于 trend
	RechargeWoW     float64
	RetainWoW       float64
	LeadNewWoW      float64 // 站点：日导量新增环比
}

// MetricForecast 单指标 7 日预测序列中的一行。
type MetricForecast struct {
	Date         string
	MetricKey    string
	MetricLabel  string
	Value        float64
	Method       string // linear_regression_7d | linear_regression | carry_forward | no_data
	SampleDays   int    // 参与回归的历史样本天数
	MethodDetail string // 中文说明，供 HTML/LLM 展示
}
