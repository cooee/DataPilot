package service_test

import (
	"testing"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/service"
	"github.com/stretchr/testify/assert"
)

func TestMonthPartitions_SingleMonth(t *testing.T) {
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	parts := service.MonthPartitionsExported(from, to)
	assert.Equal(t, []string{"202603"}, parts)
}

func TestMonthPartitions_MultiMonth(t *testing.T) {
	from := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	parts := service.MonthPartitionsExported(from, to)
	assert.Equal(t, []string{"202603", "202604", "202605"}, parts)
}

func TestMonthPartitions_SameDay(t *testing.T) {
	d := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	parts := service.MonthPartitionsExported(d, d)
	assert.Equal(t, []string{"202605"}, parts)
}
