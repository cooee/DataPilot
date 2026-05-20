package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"unicode/utf8"

	"github.com/Caknoooo/go-gin-clean-starter/modules/analytics/dto"
)

// PrintResult 按 format 输出查询结果：json | table。
func PrintResult(format string, result *dto.QueryResult, meta *dto.QueryMeta) error {
	switch format {
	case "json":
		out := map[string]any{
			"columns": result.Columns,
			"rows":    result.Rows,
			"meta":    meta,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	case "table":
		printTable(result, meta)
		return nil
	default:
		return fmt.Errorf("未知输出格式 %q，支持 json | table", format)
	}
}

func printTable(result *dto.QueryResult, meta *dto.QueryMeta) {
	if meta != nil {
		fmt.Fprintf(os.Stderr, "dataset=%s rows=%d truncated=%v\n", meta.Dataset, meta.RowCount, meta.Truncated)
		if meta.SQL != "" {
			fmt.Fprintf(os.Stderr, "sql: %s\n", meta.SQL)
		}
	}

	if len(result.Rows) == 0 {
		fmt.Println("(no rows)")
		if meta != nil && meta.Dataset == "ads_product_anomaly_daily" {
			fmt.Fprintln(os.Stderr, "提示: 无 ±3σ 异常。可尝试 -preset=anomaly-watch 查看偏离度；")
			fmt.Fprintln(os.Stderr, "      或补录更多历史日期后重跑 sync（每产品需≥2天才能计算 std）。")
		}
		return
	}

	names := make([]string, len(result.Columns))
	for i, c := range result.Columns {
		names[i] = c.Name
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	headers := make([]string, len(names))
	for i, n := range names {
		headers[i] = n
	}
	fmt.Fprintln(w, strings.Join(headers, "\t"))

	widths := make([]int, len(names))
	for i, h := range headers {
		widths[i] = utf8.RuneCountInString(h)
	}

	cells := make([][]string, len(result.Rows))
	for ri, row := range result.Rows {
		cells[ri] = make([]string, len(names))
		for ci, name := range names {
			s := formatCell(row[name])
			cells[ri][ci] = s
			if l := utf8.RuneCountInString(s); l > widths[ci] {
				widths[ci] = l
			}
		}
	}

	for _, row := range cells {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	_ = w.Flush()
}

func formatCell(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", val), "0"), ".")
	case float32:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", val), "0"), ".")
	default:
		return fmt.Sprint(v)
	}
}

// PrintSchema 输出语义层 schema。
func PrintSchema(format string, schema *dto.SchemaResponse) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(schema)
	}

	for _, ds := range schema.Datasets {
		fmt.Printf("\n[%s] %s\n", ds.Name, ds.Label)
		if ds.Description != "" {
			fmt.Printf("  %s\n", ds.Description)
		}
		if ds.DateColumn != "" {
			fmt.Printf("  date_column: %s\n", ds.DateColumn)
		}
		fmt.Println("  dimensions:")
		for _, d := range ds.Dimensions {
			fmt.Printf("    - %s (%s)\n", d.Name, d.Kind)
		}
		fmt.Println("  metrics:")
		for _, m := range ds.Metrics {
			zh := m.MetricNameZH
			if zh != "" {
				fmt.Printf("    - %s [%s] agg=%s — %s\n", m.Name, zh, m.AggFunc, m.MetricKey)
			} else {
				fmt.Printf("    - %s agg=%s\n", m.Name, m.AggFunc)
			}
		}
	}
	fmt.Println()
	return nil
}
