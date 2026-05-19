package service

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/catalog"
	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
)

const (
	defaultQueryLimit = 1000
	maxQueryLimit     = 10000
)

var identRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type builtQuery struct {
	SQL  string
	Args []any
}

func buildQuery(req *dto.QueryRequest) (builtQuery, []dto.ColumnMeta, error) {
	ds, ok := catalog.GetDataset(req.Dataset)
	if !ok {
		return builtQuery{}, nil, fmt.Errorf("未知数据集: %s", req.Dataset)
	}

	if err := validateFields(req.Metrics, ds.Metrics); err != nil {
		return builtQuery{}, nil, fmt.Errorf("metrics: %w", err)
	}
	if err := validateFields(req.Dimensions, ds.Dimensions); err != nil {
		return builtQuery{}, nil, fmt.Errorf("dimensions: %w", err)
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultQueryLimit
	}
	if limit > maxQueryLimit {
		limit = maxQueryLimit
	}

	selectCols, colMeta, err := buildSelectCols(req, ds)
	if err != nil {
		return builtQuery{}, nil, err
	}

	var (
		args       []any
		whereParts []string
	)

	if req.DateRange != nil {
		if ds.DateColumn == "" {
			return builtQuery{}, nil, fmt.Errorf("数据集 %s 不支持 date_range", ds.Name)
		}
		col := ds.DateColumn
		if req.DateRange.Field != "" {
			if req.DateRange.Field != ds.DateColumn {
				return builtQuery{}, nil, fmt.Errorf("date_range.field 仅支持 %s", ds.DateColumn)
			}
			col = req.DateRange.Field
		}
		if _, err := time.Parse("2006-01-02", req.DateRange.From); err != nil {
			return builtQuery{}, nil, fmt.Errorf("date_range.from 格式须为 YYYY-MM-DD")
		}
		if _, err := time.Parse("2006-01-02", req.DateRange.To); err != nil {
			return builtQuery{}, nil, fmt.Errorf("date_range.to 格式须为 YYYY-MM-DD")
		}
		whereParts = append(whereParts, fmt.Sprintf("%s >= ? AND %s <= ?", col, col))
		args = append(args, req.DateRange.From, req.DateRange.To)
	}

	for _, f := range req.Filters {
		part, filterArgs, err := buildFilter(f, ds)
		if err != nil {
			return builtQuery{}, nil, err
		}
		whereParts = append(whereParts, part)
		args = append(args, filterArgs...)
	}

	grouped := needsGroupBy(req, ds)

	sql := "SELECT " + strings.Join(selectCols, ", ") + " FROM " + ds.Table
	if len(whereParts) > 0 {
		sql += " WHERE " + strings.Join(whereParts, " AND ")
	}
	if grouped {
		sql += " GROUP BY " + strings.Join(req.Dimensions, ", ")
	}
	if orderSQL := buildOrderBy(req.OrderBy, ds, grouped); orderSQL != "" {
		sql += " ORDER BY " + orderSQL
	}
	sql += fmt.Sprintf(" LIMIT %d", limit)

	return builtQuery{SQL: sql, Args: args}, colMeta, nil
}

func buildSelectCols(req *dto.QueryRequest, ds catalog.DatasetDef) ([]string, []dto.ColumnMeta, error) {
	var cols []string
	var meta []dto.ColumnMeta

	for _, dim := range req.Dimensions {
		if !identRe.MatchString(dim) {
			return nil, nil, fmt.Errorf("非法字段名: %s", dim)
		}
		def := ds.Dimensions[dim]
		cols = append(cols, dim)
		meta = append(meta, dto.ColumnMeta{
			Name:      dim,
			Kind:      string(def.Kind),
			Role:      "dimension",
			MetricKey: def.MetricKey,
		})
	}

	grouped := needsGroupBy(req, ds)
	for _, m := range req.Metrics {
		if !identRe.MatchString(m) {
			return nil, nil, fmt.Errorf("非法字段名: %s", m)
		}
		def := ds.Metrics[m]
		expr := m
		if grouped {
			switch def.AggFunc {
			case catalog.AggNone:
				expr = fmt.Sprintf("any(%s) AS %s", m, m)
			default:
				expr = fmt.Sprintf("%s(%s) AS %s", def.AggFunc, m, m)
			}
		}
		cols = append(cols, expr)
		meta = append(meta, dto.ColumnMeta{
			Name:      m,
			Kind:      string(def.Kind),
			Role:      "metric",
			MetricKey: def.MetricKey,
		})
	}
	return cols, meta, nil
}

func buildFilter(f dto.Filter, ds catalog.DatasetDef) (string, []any, error) {
	if !identRe.MatchString(f.Field) {
		return "", nil, fmt.Errorf("非法字段名: %s", f.Field)
	}
	if _, ok := ds.Dimensions[f.Field]; !ok {
		if _, ok := ds.Metrics[f.Field]; !ok {
			return "", nil, fmt.Errorf("字段 %s 不在数据集 %s 中", f.Field, ds.Name)
		}
	}

	op := strings.ToLower(f.Operator)
	switch op {
	case "eq":
		return fmt.Sprintf("%s = ?", f.Field), []any{f.Value}, nil
	case "neq":
		return fmt.Sprintf("%s != ?", f.Field), []any{f.Value}, nil
	case "gt":
		return fmt.Sprintf("%s > ?", f.Field), []any{f.Value}, nil
	case "gte":
		return fmt.Sprintf("%s >= ?", f.Field), []any{f.Value}, nil
	case "lt":
		return fmt.Sprintf("%s < ?", f.Field), []any{f.Value}, nil
	case "lte":
		return fmt.Sprintf("%s <= ?", f.Field), []any{f.Value}, nil
	case "in":
		vals, err := asSlice(f.Value)
		if err != nil {
			return "", nil, err
		}
		if len(vals) == 0 {
			return "", nil, fmt.Errorf("in 操作符 value 不能为空")
		}
		placeholders := strings.TrimRight(strings.Repeat("?,", len(vals)), ",")
		return fmt.Sprintf("%s IN (%s)", f.Field, placeholders), vals, nil
	case "between":
		vals, err := asSlice(f.Value)
		if err != nil {
			return "", nil, err
		}
		if len(vals) != 2 {
			return "", nil, fmt.Errorf("between 操作符需要 2 个值")
		}
		return fmt.Sprintf("%s BETWEEN ? AND ?", f.Field), vals, nil
	default:
		return "", nil, fmt.Errorf("不支持的操作符: %s", f.Operator)
	}
}

func buildOrderBy(orders []dto.OrderBy, ds catalog.DatasetDef, grouped bool) string {
	if len(orders) == 0 {
		return ""
	}
	var parts []string
	for _, o := range orders {
		if !identRe.MatchString(o.Field) {
			continue
		}
		if _, ok := ds.Dimensions[o.Field]; !ok {
			if _, ok := ds.Metrics[o.Field]; !ok {
				continue
			}
		}
		dir := "ASC"
		if o.Desc {
			dir = "DESC"
		}
		field := o.Field
		if grouped {
			if def, ok := ds.Metrics[o.Field]; ok {
				switch def.AggFunc {
				case catalog.AggNone:
					field = fmt.Sprintf("any(%s)", o.Field)
				default:
					field = fmt.Sprintf("%s(%s)", def.AggFunc, o.Field)
				}
			}
		}
		parts = append(parts, field+" "+dir)
	}
	return strings.Join(parts, ", ")
}

// needsGroupBy 仅当存在维度且至少一个指标需要聚合时为 true。
// ADS 预聚合视图（指标均为 AggNone）按行查询，不加 GROUP BY。
func needsGroupBy(req *dto.QueryRequest, ds catalog.DatasetDef) bool {
	if len(req.Dimensions) == 0 {
		return false
	}
	for _, m := range req.Metrics {
		if def, ok := ds.Metrics[m]; ok && def.AggFunc != catalog.AggNone {
			return true
		}
	}
	return false
}

func validateFields[T any](fields []string, allowed map[string]T) error {
	for _, f := range fields {
		if !identRe.MatchString(f) {
			return fmt.Errorf("非法字段名: %s", f)
		}
		if _, ok := allowed[f]; !ok {
			return fmt.Errorf("字段 %s 不在白名单", f)
		}
	}
	return nil
}

func asSlice(v any) ([]any, error) {
	switch val := v.(type) {
	case []any:
		return val, nil
	case []string:
		out := make([]any, len(val))
		for i, s := range val {
			out[i] = s
		}
		return out, nil
	default:
		return nil, fmt.Errorf("value 须为数组")
	}
}
