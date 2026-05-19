package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	analyticsCLI "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/cli"
	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
	analyticsSvc "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/service"
	"github.com/samber/do"
)

type analyticsArgs struct {
	preset   string
	dataset  string
	metrics  []string
	dims     []string
	from     string
	to       string
	filters  []dto.Filter
	orderBy  []dto.OrderBy
	limit    int
	jsonRaw  string
	jsonFile string
	format   string
}

func runAnalyticsSchema(injector *do.Injector) {
	args := parseAnalyticsFlags("--analytics:schema")
	svc := do.MustInvoke[analyticsSvc.AnalyticsService](injector)
	schema, err := svc.GetSchema(context.Background())
	if err != nil {
		log.Fatalf("[analytics:schema] %v", err)
	}
	if err := analyticsCLI.PrintSchema(args.format, schema); err != nil {
		log.Fatalf("[analytics:schema] %v", err)
	}
}

func runAnalyticsQuery(injector *do.Injector) {
	args := parseAnalyticsFlags("--analytics:query")
	svc := do.MustInvoke[analyticsSvc.AnalyticsService](injector)

	req, err := args.buildRequest()
	if err != nil {
		log.Fatalf("[analytics:query] %v", err)
	}

	result, meta, err := svc.Query(context.Background(), req)
	if err != nil {
		log.Fatalf("[analytics:query] %v", err)
	}

	if err := analyticsCLI.PrintResult(args.format, result, meta); err != nil {
		log.Fatalf("[analytics:query] %v", err)
	}
}

func parseAnalyticsFlags(cmd string) analyticsArgs {
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	preset := fs.String("preset", "", "内置场景: product-type-compare | product-detail | product-trend | team-performance | product-health")
	dataset := fs.String("dataset", "", "数据集名称")
	metrics := fs.String("metrics", "", "指标，逗号分隔")
	dims := fs.String("dimensions", "", "维度，逗号分隔")
	from := fs.String("from", "", "起始日期 YYYY-MM-DD")
	to := fs.String("to", "", "结束日期 YYYY-MM-DD")
	filterStrs := fs.String("filter", "", "过滤条件，可重复。格式 field:op:value，如 product_type:in:paid,free")
	orderStr := fs.String("order", "", "排序，格式 field:asc|desc，多个逗号分隔")
	limit := fs.Int("limit", 0, "最大行数")
	jsonRaw := fs.String("json", "", "完整 JSON 查询体（覆盖其他参数）")
	jsonFile := fs.String("json-file", "", "JSON 查询文件路径")
	format := fs.String("format", "table", "输出格式: table | json")

	rest := argsAfterCommand(cmd)
	_ = fs.Parse(rest)

	filters, err := parseCLIFilters(*filterStrs)
	if err != nil {
		log.Fatalf("[analytics] %v", err)
	}
	orderBy, err := parseCLIOrder(*orderStr)
	if err != nil {
		log.Fatalf("[analytics] %v", err)
	}

	return analyticsArgs{
		preset:   *preset,
		dataset:  *dataset,
		metrics:  splitCSV(*metrics),
		dims:     splitCSV(*dims),
		from:     *from,
		to:       *to,
		filters:  filters,
		orderBy:  orderBy,
		limit:    *limit,
		jsonRaw:  *jsonRaw,
		jsonFile: *jsonFile,
		format:   *format,
	}
}

func (a analyticsArgs) buildRequest() (*dto.QueryRequest, error) {
	if a.jsonFile != "" {
		raw, err := os.ReadFile(a.jsonFile)
		if err != nil {
			return nil, fmt.Errorf("读取 json-file: %w", err)
		}
		return parseJSONRequest(raw)
	}
	if a.jsonRaw != "" {
		return parseJSONRequest([]byte(a.jsonRaw))
	}
	if a.preset != "" {
		fn, ok := analyticsCLI.Presets[a.preset]
		if !ok {
			return nil, fmt.Errorf("未知 preset %q，可选: %s", a.preset, strings.Join(presetNames(), ", "))
		}
		req, err := fn(a.from, a.to)
		if err != nil {
			return nil, err
		}
		if len(a.filters) > 0 {
			req.Filters = append(req.Filters, a.filters...)
		}
		if a.limit > 0 {
			req.Limit = a.limit
		}
		return req, nil
	}

	if a.dataset == "" {
		return nil, fmt.Errorf("需要 -dataset 或 -preset 或 -json/-json-file")
	}
	if len(a.metrics) == 0 {
		return nil, fmt.Errorf("需要 -metrics")
	}

	req := &dto.QueryRequest{
		Dataset:    a.dataset,
		Metrics:    a.metrics,
		Dimensions: a.dims,
		Filters:    a.filters,
		OrderBy:    a.orderBy,
		Limit:      a.limit,
	}
	if a.from != "" || a.to != "" {
		from, to := a.from, a.to
		if from == "" {
			from = to
		}
		if to == "" {
			to = from
		}
		req.DateRange = &dto.DateRange{From: from, To: to}
	}
	return req, nil
}

func parseJSONRequest(raw []byte) (*dto.QueryRequest, error) {
	var req dto.QueryRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("解析 JSON: %w", err)
	}
	return &req, nil
}

func argsAfterCommand(cmd string) []string {
	for i, a := range os.Args[1:] {
		if a == cmd {
			return os.Args[i+2:]
		}
	}
	return nil
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseCLIFilters 支持多次 -filter 或逗号分隔多个条件（用 | 分隔多条）。
// 格式: field:op:value  例 product_type:eq:paid  /  product_type:in:paid,free
func parseCLIFilters(combined string) ([]dto.Filter, error) {
	if combined == "" {
		return nil, nil
	}
	var filters []dto.Filter
	for _, part := range strings.Split(combined, "|") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		f, err := parseOneFilter(part)
		if err != nil {
			return nil, err
		}
		filters = append(filters, f)
	}
	return filters, nil
}

func parseOneFilter(s string) (dto.Filter, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return dto.Filter{}, fmt.Errorf("filter 格式须为 field:op:value， got %q", s)
	}
	field, op, valStr := parts[0], parts[1], parts[2]
	var value any = valStr
	if op == "in" {
		value = splitCSV(valStr)
	}
	if op == "between" {
		betweenParts := strings.SplitN(valStr, ",", 2)
		if len(betweenParts) != 2 {
			return dto.Filter{}, fmt.Errorf("between 需要两个值，用逗号分隔")
		}
		value = []any{strings.TrimSpace(betweenParts[0]), strings.TrimSpace(betweenParts[1])}
	}
	return dto.Filter{Field: field, Operator: op, Value: value}, nil
}

func parseCLIOrder(s string) ([]dto.OrderBy, error) {
	if s == "" {
		return nil, nil
	}
	var orders []dto.OrderBy
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		sub := strings.SplitN(part, ":", 2)
		if len(sub) != 2 {
			return nil, fmt.Errorf("order 格式须为 field:asc|desc， got %q", part)
		}
		orders = append(orders, dto.OrderBy{
			Field: sub[0],
			Desc:  strings.EqualFold(sub[1], "desc"),
		})
	}
	return orders, nil
}

func presetNames() []string {
	names := make([]string, 0, len(analyticsCLI.Presets))
	for k := range analyticsCLI.Presets {
		names = append(names, k)
	}
	return names
}
