package mapper_test

import (
	"testing"

	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/mapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeSiteRow() []string {
	return []string{
		"2026-05-01", "91看片", "JHA-1006", "增长1组", "四部",
		"5162", "0", "21", "0", "200", "0",
	}
}

func TestMapSiteRow_HappyPath(t *testing.T) {
	r, warns, err := mapper.MapSiteRow(makeSiteRow(), 1, "gsheet:test:gid553168897")
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Equal(t, "JHA-1006", r.ProductCode)
	assert.Equal(t, uint64(5162), r.DAU)
	assert.Equal(t, uint64(21), r.LeadNewCnt)
	assert.InDelta(t, 200.0, r.LeadRechargeAmt, 1e-6)
	assert.Equal(t, "site", r.ProductType)
}

func TestMapSiteRow_InvalidDate(t *testing.T) {
	row := makeSiteRow()
	row[0] = "bad"
	_, _, err := mapper.MapSiteRow(row, 1, "src")
	require.Error(t, err)
}
