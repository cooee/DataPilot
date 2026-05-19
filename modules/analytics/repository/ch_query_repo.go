package repository

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/shopspring/decimal"
)

type CHQueryRepo struct {
	conn driver.Conn
}

func NewCHQueryRepo(conn driver.Conn) *CHQueryRepo {
	return &CHQueryRepo{conn: conn}
}

func (r *CHQueryRepo) Query(ctx context.Context, sql string, args []any) ([]map[string]any, error) {
	rows, err := r.conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("clickhouse query: %w", err)
	}
	defer rows.Close()

	columns := rows.Columns()
	columnTypes := rows.ColumnTypes()
	var result []map[string]any

	for rows.Next() {
		scanTargets := make([]any, len(columnTypes))
		for i, ct := range columnTypes {
			scanTargets[i] = reflect.New(ct.ScanType()).Interface()
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		row := make(map[string]any, len(columns))
		for i, col := range columns {
			row[col] = normalizeCHValue(reflect.ValueOf(scanTargets[i]).Elem().Interface())
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeCHValue(v any) any {
	switch val := v.(type) {
	case time.Time:
		return val.Format("2006-01-02")
	case decimal.Decimal:
		f, _ := val.Float64()
		return f
	case *decimal.Decimal:
		if val == nil {
			return nil
		}
		f, _ := val.Float64()
		return f
	case []byte:
		return string(val)
	default:
		return val
	}
}
