package dto

// QueryRequest 语义层查询请求（行业主流：metrics + dimensions + filters，非原始 SQL）。
type QueryRequest struct {
	Dataset    string      `json:"dataset" binding:"required"`
	Metrics    []string    `json:"metrics" binding:"required,min=1"`
	Dimensions []string    `json:"dimensions"`
	Filters    []Filter    `json:"filters"`
	DateRange  *DateRange  `json:"date_range"`
	OrderBy    []OrderBy   `json:"order_by"`
	Limit      int         `json:"limit"`
}

type Filter struct {
	Field    string `json:"field" binding:"required"`
	Operator string `json:"operator" binding:"required"` // eq, neq, in, gt, gte, lt, lte, between
	Value    any    `json:"value" binding:"required"`
}

type DateRange struct {
	Field string `json:"field"` // 默认使用数据集 date 列
	From  string `json:"from" binding:"required"` // YYYY-MM-DD
	To    string `json:"to" binding:"required"`
}

type OrderBy struct {
	Field string `json:"field" binding:"required"`
	Desc  bool   `json:"desc"`
}

// ColumnMeta 列元数据，供 AI 理解字段含义。
type ColumnMeta struct {
	Name        string  `json:"name"`
	Kind        string  `json:"kind"` // date | string | number
	Role        string  `json:"role"` // dimension | metric
	MetricKey   string  `json:"metric_key,omitempty"`
	MetricNameZH string `json:"metric_name_zh,omitempty"`
	Unit        string  `json:"unit,omitempty"`
	Direction   string  `json:"direction,omitempty"`
	Description string  `json:"description,omitempty"`
}

// QueryResult 统一 JSON 表格响应，便于 LLM 消费。
type QueryResult struct {
	Columns []ColumnMeta     `json:"columns"`
	Rows    []map[string]any `json:"rows"`
}

type QueryMeta struct {
	Dataset   string `json:"dataset"`
	RowCount  int    `json:"row_count"`
	Truncated bool   `json:"truncated"`
	SQL       string `json:"sql,omitempty"` // 仅 debug 模式返回
}

// SchemaResponse 数据集语义描述，供 AI tool-calling / 探索。
type SchemaResponse struct {
	Datasets []DatasetSchema `json:"datasets"`
}

type DatasetSchema struct {
	Name        string              `json:"name"`
	Label       string              `json:"label"`
	Description string              `json:"description"`
	DateColumn  string              `json:"date_column,omitempty"`
	Dimensions  []FieldSchema       `json:"dimensions"`
	Metrics     []MetricFieldSchema `json:"metrics"`
}

type FieldSchema struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type MetricFieldSchema struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	AggFunc     string `json:"agg_func"`
	MetricKey   string `json:"metric_key,omitempty"`
	MetricNameZH string `json:"metric_name_zh,omitempty"`
	Unit        string `json:"unit,omitempty"`
	Direction   string `json:"direction,omitempty"`
	Description string `json:"description,omitempty"`
}
