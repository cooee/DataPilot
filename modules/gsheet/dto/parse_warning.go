package dto

import "fmt"

// ParseWarning 记录 CSV 宽容解析时遇到的字段级问题
type ParseWarning struct {
	RowNo   int
	Field   string
	RawVal  string
	Message string
}

func (w ParseWarning) Error() string {
	return fmt.Sprintf("row %d field %q raw=%q: %s", w.RowNo, w.Field, w.RawVal, w.Message)
}
