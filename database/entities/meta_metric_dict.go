package entities

import "gorm.io/gorm"

// MetaMetricDict 指标语义层，供 AI Agent 解读 ClickHouse 指标含义。
type MetaMetricDict struct {
	gorm.Model
	MetricKey    string `gorm:"column:metric_key;uniqueIndex;not null"`   // DWD 英文列名（snake_case）
	MetricNameZH string `gorm:"column:metric_name_zh;not null"`           // 中文显示名
	Category     string `gorm:"column:category;not null;index"`           // 指标分类
	Unit         string `gorm:"column:unit;not null"`                     // 单位类型: count/amount/ratio/multiple
	Direction    string `gorm:"column:direction;not null"`                // higher_better / lower_better / neutral
	Description  string `gorm:"column:description;type:text"`             // 供 AI 理解的自然语言描述
}

func (MetaMetricDict) TableName() string { return "meta_metric_dict" }
