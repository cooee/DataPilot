# DataPilot GSheet→ClickHouse 数仓 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 Google Sheet 运营日报数据通过五层数仓（ODS/DIM/DWD/DWS/ADS）结构化存入 ClickHouse，并提供 CLI 触发的全量/增量同步命令。

**Architecture:** Go 负责 CSV 采集与解析（Sheet→ODS），SQL 文件负责层间 ETL（ODS→DWD→DWS→ADS）；Go 读取 SQL 文件并用参数化方式执行。各层用 `ReplacingMergeTree(_version)` 保证 upsert 幂等。CLI 子命令沿用现有 `--migrate:` 风格。

**Tech Stack:** Go 1.26 / clickhouse-go/v2 / samber/do DI / google.golang.org/api / GORM(PG) / ClickHouse ReplacingMergeTree

---

## 文件结构总览

```
database/clickhouse/
├── migrations/
│   ├── 001_ods_product_daily_report.sql
│   ├── 002_dim_product.sql
│   ├── 003_dwd_product_daily_metric.sql
│   ├── 004_dws_product_daily.sql
│   ├── 005_dws_team_daily.sql
│   ├── 006_dws_bu_daily.sql
│   └── 007_ads_views.sql
└── etl/
    ├── ods_to_dim.sql
    ├── ods_to_dwd.sql
    ├── dwd_to_dws_product.sql
    ├── dwd_to_dws_team.sql
    └── dwd_to_dws_bu.sql

database/pg_seeds/
└── 001_meta_metric_dict.sql

modules/gsheet/
├── dto/
│   ├── ods_row.go
│   └── parse_warning.go
├── mapper/
│   └── ods_mapper.go
├── repository/
│   ├── ods_repo.go
│   └── etl_repo.go
└── service/
    ├── fetcher.go
    ├── ods_loader.go
    ├── etl_runner.go
    └── dq_checker.go

cmd/sync_gsheet.go          (新)
cmd/main.go                 (改)
providers/core.go           (改)
pkg/constants/common.go     (改)
Makefile                    (改)
```

---

## Task 1：ClickHouse Migration 体系 + DDL 建表

**Files:**
- Create: `database/clickhouse/migrations/001_ods_product_daily_report.sql`
- Create: `database/clickhouse/migrations/002_dim_product.sql`
- Create: `database/clickhouse/migrations/003_dwd_product_daily_metric.sql`
- Create: `database/clickhouse/migrations/004_dws_product_daily.sql`
- Create: `database/clickhouse/migrations/005_dws_team_daily.sql`
- Create: `database/clickhouse/migrations/006_dws_bu_daily.sql`
- Create: `database/clickhouse/migrations/007_ads_views.sql`
- Create: `modules/gsheet/repository/etl_repo.go` (仅 migration runner 部分)
- Modify: `cmd/main.go`
- Modify: `Makefile`

- [ ] **Step 1: 创建 001_ods_product_daily_report.sql**

```sql
-- database/clickhouse/migrations/001_ods_product_daily_report.sql
CREATE TABLE IF NOT EXISTS ods_product_daily_report (
    `日期`             Date,
    `产品名称`         String,
    `产品编号`         LowCardinality(String),
    `小组`             LowCardinality(String),
    `部门`             LowCardinality(String),
    `日活`             UInt64,
    `日活环比`         Float64,
    `总充值`           Decimal(18, 2),
    `总充值环比`       Float64,
    `自然充值`         Decimal(18, 2),
    `自然充值环比`     Float64,
    `渠道充值`         Decimal(18, 2),
    `渠道充值环比`     Float64,
    `内导充值`         Decimal(18, 2),
    `内导充值环比`     Float64,
    `总新增`           UInt64,
    `总新增环比`       Float64,
    `自然新增`         UInt64,
    `自然新增环比`     Float64,
    `渠道新增`         UInt64,
    `渠道新增环比`     Float64,
    `内导新增`         UInt64,
    `内导新增环比`     Float64,
    `新充人数`         UInt64,
    `老充人数`         UInt64,
    `新用户充值`       Decimal(18, 2),
    `老用户充值`       Decimal(18, 2),
    `充值单数`         UInt64,
    `付款成功率`       Float64,
    `次留率`           Float64,
    `3留率`            Float64,
    `7留率`            Float64,
    `下载页访问数`     UInt64,
    `下载页点击数`     UInt64,
    `落地页下载率`     Float64,
    `总转化`           Float64,
    `新充转化`         Float64,
    `老充转化`         Float64,
    `老用户日活`       UInt64,
    `ARPPU`            Decimal(18, 4),
    `_source`          LowCardinality(String) DEFAULT 'gsheet:测试-写入',
    `_src_row_no`      UInt32,
    `_ingested_at`     DateTime64(3) DEFAULT now64(),
    `_version`         UInt64 MATERIALIZED toUnixTimestamp64Milli(`_ingested_at`)
)
ENGINE = ReplacingMergeTree(`_version`)
PARTITION BY toYYYYMM(`日期`)
ORDER BY (`_source`, `日期`, `产品编号`)
SETTINGS index_granularity = 8192;
```

- [ ] **Step 2: 创建 002_dim_product.sql**

```sql
-- database/clickhouse/migrations/002_dim_product.sql
CREATE TABLE IF NOT EXISTS dim_product (
    product_code      LowCardinality(String),
    product_name      String,
    team              LowCardinality(String),
    business_unit     LowCardinality(String),
    product_category  Nullable(LowCardinality(String)),
    is_active         UInt8    DEFAULT 1,
    effective_from    Date     DEFAULT today(),
    effective_to      Nullable(Date),
    `_source`         LowCardinality(String) DEFAULT 'gsheet:测试-写入',
    `_ingested_at`    DateTime64(3) DEFAULT now64(),
    `_version`        UInt64 MATERIALIZED toUnixTimestamp64Milli(`_ingested_at`)
)
ENGINE = ReplacingMergeTree(`_version`)
ORDER BY (product_code, effective_from);
```

- [ ] **Step 3: 创建 003_dwd_product_daily_metric.sql**

```sql
-- database/clickhouse/migrations/003_dwd_product_daily_metric.sql
CREATE TABLE IF NOT EXISTS dwd_product_daily_metric (
    date                              Date,
    product_code                      LowCardinality(String),
    dau                               UInt64,
    dau_chain_ratio                   Float64,
    old_user_dau                      UInt64,
    recharge_total_amt                Decimal(18, 2),
    recharge_total_chain_ratio        Float64,
    recharge_organic_amt              Decimal(18, 2),
    recharge_organic_chain_ratio      Float64,
    recharge_channel_amt              Decimal(18, 2),
    recharge_channel_chain_ratio      Float64,
    recharge_internal_amt             Decimal(18, 2),
    recharge_internal_chain_ratio     Float64,
    new_user_total_cnt                UInt64,
    new_user_total_chain_ratio        Float64,
    new_user_organic_cnt              UInt64,
    new_user_organic_chain_ratio      Float64,
    new_user_channel_cnt              UInt64,
    new_user_channel_chain_ratio      Float64,
    new_user_internal_cnt             UInt64,
    new_user_internal_chain_ratio     Float64,
    new_paying_user_cnt               UInt64,
    old_paying_user_cnt               UInt64,
    new_paying_user_amt               Decimal(18, 2),
    old_paying_user_amt               Decimal(18, 2),
    recharge_order_cnt                UInt64,
    payment_success_ratio             Float64,
    retention_d1_ratio                Float64,
    retention_d3_ratio                Float64,
    retention_d7_ratio                Float64,
    landing_page_visit_cnt            UInt64,
    landing_page_click_cnt            UInt64,
    landing_page_download_ratio       Float64,
    conversion_total_multi            Float64,
    conversion_new_paying_multi       Float64,
    conversion_old_paying_multi       Float64,
    arppu                             Decimal(18, 4),
    `_src_row_no`                     UInt32,
    `_ingested_at`                    DateTime64(3) DEFAULT now64(),
    `_version`                        UInt64 MATERIALIZED toUnixTimestamp64Milli(`_ingested_at`)
)
ENGINE = ReplacingMergeTree(`_version`)
PARTITION BY toYYYYMM(date)
ORDER BY (date, product_code);
```

- [ ] **Step 4: 创建 004_dws_product_daily.sql**

```sql
-- database/clickhouse/migrations/004_dws_product_daily.sql
CREATE TABLE IF NOT EXISTS dws_product_daily (
    date                          Date,
    product_code                  LowCardinality(String),
    product_name                  String,
    team                          LowCardinality(String),
    business_unit                 LowCardinality(String),
    dau                           UInt64,
    dau_chain_ratio               Float64,
    old_user_dau                  UInt64,
    recharge_total_amt            Decimal(18, 2),
    recharge_total_chain_ratio    Float64,
    recharge_organic_amt          Decimal(18, 2),
    recharge_organic_chain_ratio  Float64,
    recharge_channel_amt          Decimal(18, 2),
    recharge_channel_chain_ratio  Float64,
    recharge_internal_amt         Decimal(18, 2),
    recharge_internal_chain_ratio Float64,
    new_user_total_cnt            UInt64,
    new_user_total_chain_ratio    Float64,
    new_user_organic_cnt          UInt64,
    new_user_organic_chain_ratio  Float64,
    new_user_channel_cnt          UInt64,
    new_user_channel_chain_ratio  Float64,
    new_user_internal_cnt         UInt64,
    new_user_internal_chain_ratio Float64,
    new_paying_user_cnt           UInt64,
    old_paying_user_cnt           UInt64,
    new_paying_user_amt           Decimal(18, 2),
    old_paying_user_amt           Decimal(18, 2),
    recharge_order_cnt            UInt64,
    payment_success_ratio         Float64,
    retention_d1_ratio            Float64,
    retention_d3_ratio            Float64,
    retention_d7_ratio            Float64,
    landing_page_visit_cnt        UInt64,
    landing_page_click_cnt        UInt64,
    landing_page_download_ratio   Float64,
    conversion_total_multi        Float64,
    conversion_new_paying_multi   Float64,
    conversion_old_paying_multi   Float64,
    arppu                         Decimal(18, 4),
    `_built_at`                   DateTime64(3) DEFAULT now64()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(date)
ORDER BY (date, product_code);
```

- [ ] **Step 5: 创建 005_dws_team_daily.sql**

```sql
-- database/clickhouse/migrations/005_dws_team_daily.sql
CREATE TABLE IF NOT EXISTS dws_team_daily (
    date                       Date,
    team                       LowCardinality(String),
    business_unit              LowCardinality(String),
    active_product_cnt         UInt32,
    dau_sum                    UInt64,
    recharge_total_amt_sum     Decimal(18, 2),
    recharge_organic_amt_sum   Decimal(18, 2),
    recharge_channel_amt_sum   Decimal(18, 2),
    recharge_internal_amt_sum  Decimal(18, 2),
    new_user_total_cnt_sum     UInt64,
    new_user_organic_cnt_sum   UInt64,
    new_user_channel_cnt_sum   UInt64,
    new_paying_user_cnt_sum    UInt64,
    old_paying_user_cnt_sum    UInt64,
    recharge_order_cnt_sum     UInt64,
    old_user_dau_sum           UInt64,
    retention_d7_ratio_num     Float64,
    retention_d7_ratio_den     Float64,
    `_built_at`                DateTime64(3) DEFAULT now64()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(date)
ORDER BY (date, team);
```

- [ ] **Step 6: 创建 006_dws_bu_daily.sql**

```sql
-- database/clickhouse/migrations/006_dws_bu_daily.sql
CREATE TABLE IF NOT EXISTS dws_bu_daily (
    date                       Date,
    business_unit              LowCardinality(String),
    active_product_cnt         UInt32,
    dau_sum                    UInt64,
    recharge_total_amt_sum     Decimal(18, 2),
    recharge_organic_amt_sum   Decimal(18, 2),
    recharge_channel_amt_sum   Decimal(18, 2),
    recharge_internal_amt_sum  Decimal(18, 2),
    new_user_total_cnt_sum     UInt64,
    new_user_organic_cnt_sum   UInt64,
    new_user_channel_cnt_sum   UInt64,
    new_paying_user_cnt_sum    UInt64,
    old_paying_user_cnt_sum    UInt64,
    recharge_order_cnt_sum     UInt64,
    old_user_dau_sum           UInt64,
    retention_d7_ratio_num     Float64,
    retention_d7_ratio_den     Float64,
    `_built_at`                DateTime64(3) DEFAULT now64()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(date)
ORDER BY (date, business_unit);
```

- [ ] **Step 7: 创建 007_ads_views.sql**

```sql
-- database/clickhouse/migrations/007_ads_views.sql
CREATE OR REPLACE VIEW ads_product_health_overview AS
SELECT
    product_code, product_name, team, business_unit,
    avg(dau)                   AS avg_dau_30d,
    sum(recharge_total_amt)    AS recharge_total_30d,
    avg(retention_d7_ratio)    AS avg_retention_d7_30d,
    avg(payment_success_ratio) AS avg_payment_success_30d,
    avg(arppu)                 AS avg_arppu_30d,
    least(1, greatest(0,
          0.3 * (avg(retention_d7_ratio) / 0.05)
        + 0.3 * (avg(payment_success_ratio) / 0.7)
        + 0.4 * least(1, toFloat64(sum(recharge_total_amt)) / 1000000)
    )) AS health_score
FROM dws_product_daily
WHERE date >= today() - 30
GROUP BY product_code, product_name, team, business_unit;

CREATE OR REPLACE VIEW ads_team_performance_daily AS
SELECT
    date, team, business_unit, active_product_cnt, dau_sum,
    recharge_total_amt_sum, new_paying_user_cnt_sum, recharge_order_cnt_sum,
    if(retention_d7_ratio_den > 0,
       retention_d7_ratio_num / retention_d7_ratio_den, 0) AS retention_d7_ratio_wavg
FROM dws_team_daily
WHERE date >= today() - 90;

CREATE OR REPLACE VIEW ads_anomaly_baseline AS
SELECT product_code, 'dau' AS metric,
    avg(dau) AS mean, stddevPop(dau) AS std,
    avg(dau) + 3*stddevPop(dau) AS upper_3sigma,
    avg(dau) - 3*stddevPop(dau) AS lower_3sigma
FROM dws_product_daily WHERE date >= today() - 30 GROUP BY product_code
UNION ALL
SELECT product_code, 'recharge_total_amt',
    avg(recharge_total_amt), stddevPop(recharge_total_amt),
    avg(recharge_total_amt) + 3*stddevPop(recharge_total_amt),
    avg(recharge_total_amt) - 3*stddevPop(recharge_total_amt)
FROM dws_product_daily WHERE date >= today() - 30 GROUP BY product_code
UNION ALL
SELECT product_code, 'retention_d7_ratio',
    avg(retention_d7_ratio), stddevPop(retention_d7_ratio),
    avg(retention_d7_ratio) + 3*stddevPop(retention_d7_ratio),
    avg(retention_d7_ratio) - 3*stddevPop(retention_d7_ratio)
FROM dws_product_daily WHERE date >= today() - 30 GROUP BY product_code;
```

- [ ] **Step 8: 实现 CH migration runner（`modules/gsheet/repository/etl_repo.go` 骨架）**

```go
// modules/gsheet/repository/etl_repo.go
package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type EtlRepo struct {
	conn driver.Conn
}

func NewEtlRepo(conn driver.Conn) *EtlRepo {
	return &EtlRepo{conn: conn}
}

// RunMigrations 按文件名编号顺序执行 database/clickhouse/migrations/*.sql
func (r *EtlRepo) RunMigrations(ctx context.Context, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, filepath.Join(migrationsDir, e.Name()))
		}
	}
	sort.Strings(files)

	for _, f := range files {
		sql, err := os.ReadFile(f)
		if err != nil {
			return fmt.Errorf("read %s: %w", f, err)
		}
		// 007_ads_views.sql 含多条语句，按分号拆分逐条执行
		for _, stmt := range splitSQL(string(sql)) {
			if err := r.conn.Exec(ctx, stmt); err != nil {
				return fmt.Errorf("exec %s: %w", f, err)
			}
		}
		fmt.Printf("[ch-migrate] applied: %s\n", filepath.Base(f))
	}
	return nil
}

// splitSQL 把一个 SQL 文件拆成多条语句（按分号+换行，容忍行尾空格）
func splitSQL(content string) []string {
	var stmts []string
	for _, s := range strings.Split(content, ";\n") {
		s = strings.TrimSpace(s)
		if s != "" {
			stmts = append(stmts, s)
		}
	}
	return stmts
}

// ExecETL 读取 ETL SQL 文件，将 named params 绑定后执行
func (r *EtlRepo) ExecETL(ctx context.Context, sqlFile string, params map[string]interface{}) error {
	sql, err := os.ReadFile(sqlFile)
	if err != nil {
		return fmt.Errorf("read etl sql %s: %w", sqlFile, err)
	}
	// 把 map 转成 clickhouse.NamedValue slice
	named := make([]driver.NamedValue, 0, len(params))
	for k, v := range params {
		named = append(named, driver.NamedValue{Name: k, Value: v})
	}
	ctx2, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	return r.conn.Exec(ctx2, string(sql), named...)
}

// DropPartition 删除指定分区（DWS 重算幂等用）
func (r *EtlRepo) DropPartition(ctx context.Context, table, partition string) error {
	return r.conn.Exec(ctx,
		fmt.Sprintf("ALTER TABLE %s DROP PARTITION '%s'", table, partition))
}
```

- [ ] **Step 9: 在 `cmd/main.go` 加 `--ch:migrate` dispatch**

```go
// cmd/main.go — 在现有 --test:gsheet:public / --test:gsheet dispatch 后追加
for _, arg := range os.Args[1:] {
    switch arg {
    case "--test:gsheet:public":
        runTestGSheetPublic()
        return
    case "--test:gsheet":
        runTestGSheet(injector)
        return
    case "--ch:migrate":
        runCHMigrate(injector)
        return
    case "--sync:ods", "--sync:dim", "--sync:dwd", "--sync:dws", "--sync:ads", "--sync:all":
        runSyncGSheet(injector, arg)
        return
    }
}
```

- [ ] **Step 10: 在 Makefile 加 ch-migrate target**

```makefile
# ClickHouse migrations（建表/视图）
ch-migrate:
	go run ./cmd --ch:migrate
```

- [ ] **Step 11: 验证 DDL 能被 CH 执行**

```bash
make ch-migrate
```

期望输出：
```
[ch-migrate] applied: 001_ods_product_daily_report.sql
[ch-migrate] applied: 002_dim_product.sql
[ch-migrate] applied: 003_dwd_product_daily_metric.sql
[ch-migrate] applied: 004_dws_product_daily.sql
[ch-migrate] applied: 005_dws_team_daily.sql
[ch-migrate] applied: 006_dws_bu_daily.sql
[ch-migrate] applied: 007_ads_views.sql
```

- [ ] **Step 12: Commit**

```bash
git add database/clickhouse/ modules/gsheet/repository/etl_repo.go cmd/main.go Makefile
git commit -m "feat: add CH migrations runner and 7 DDL files (ODS/DIM/DWD/DWS/ADS)"
```

---

## Task 2：DTO、Mapper 与 CSV 解析器

**Files:**
- Create: `modules/gsheet/dto/ods_row.go`
- Create: `modules/gsheet/dto/parse_warning.go`
- Create: `modules/gsheet/mapper/ods_mapper.go`

- [ ] **Step 1: 创建 `modules/gsheet/dto/ods_row.go`**

```go
// modules/gsheet/dto/ods_row.go
package dto

import "time"

// ODSRow 与 ods_product_daily_report 表 1:1 对应，字段名为中文拼音注释
type ODSRow struct {
	Date                    time.Time `ch:"日期"`
	ProductName             string    `ch:"产品名称"`
	ProductCode             string    `ch:"产品编号"`
	Team                    string    `ch:"小组"`
	BusinessUnit            string    `ch:"部门"`
	DAU                     uint64    `ch:"日活"`
	DAUChain                float64   `ch:"日活环比"`
	RechargeTotal           float64   `ch:"总充值"`
	RechargeTotalChain      float64   `ch:"总充值环比"`
	RechargeOrganic         float64   `ch:"自然充值"`
	RechargeOrganicChain    float64   `ch:"自然充值环比"`
	RechargeChannel         float64   `ch:"渠道充值"`
	RechargeChannelChain    float64   `ch:"渠道充值环比"`
	RechargeInternal        float64   `ch:"内导充值"`
	RechargeInternalChain   float64   `ch:"内导充值环比"`
	NewUserTotal            uint64    `ch:"总新增"`
	NewUserTotalChain       float64   `ch:"总新增环比"`
	NewUserOrganic          uint64    `ch:"自然新增"`
	NewUserOrganicChain     float64   `ch:"自然新增环比"`
	NewUserChannel          uint64    `ch:"渠道新增"`
	NewUserChannelChain     float64   `ch:"渠道新增环比"`
	NewUserInternal         uint64    `ch:"内导新增"`
	NewUserInternalChain    float64   `ch:"内导新增环比"`
	NewPayingUserCnt        uint64    `ch:"新充人数"`
	OldPayingUserCnt        uint64    `ch:"老充人数"`
	NewPayingUserAmt        float64   `ch:"新用户充值"`
	OldPayingUserAmt        float64   `ch:"老用户充值"`
	RechargeOrderCnt        uint64    `ch:"充值单数"`
	PaymentSuccessRatio     float64   `ch:"付款成功率"`
	RetentionD1             float64   `ch:"次留率"`
	RetentionD3             float64   `ch:"3留率"`
	RetentionD7             float64   `ch:"7留率"`
	LandingPageVisit        uint64    `ch:"下载页访问数"`
	LandingPageClick        uint64    `ch:"下载页点击数"`
	LandingPageDownload     float64   `ch:"落地页下载率"`
	ConversionTotal         float64   `ch:"总转化"`
	ConversionNewPaying     float64   `ch:"新充转化"`
	ConversionOldPaying     float64   `ch:"老充转化"`
	OldUserDAU              uint64    `ch:"老用户日活"`
	ARPPU                   float64   `ch:"ARPPU"`

	// 元数据
	Source    string    `ch:"_source"`
	SrcRowNo  uint32    `ch:"_src_row_no"`
	IngestedAt time.Time `ch:"_ingested_at"`
}
```

- [ ] **Step 2: 创建 `modules/gsheet/dto/parse_warning.go`**

```go
// modules/gsheet/dto/parse_warning.go
package dto

import "fmt"

type ParseWarning struct {
	RowNo   int
	Field   string
	RawVal  string
	Message string
}

func (w ParseWarning) Error() string {
	return fmt.Sprintf("row %d field %q raw=%q: %s", w.RowNo, w.Field, w.RawVal, w.Message)
}
```

- [ ] **Step 3: 创建 `modules/gsheet/mapper/ods_mapper.go`**

这是唯一耦合"中文列名→字段索引"的地方。表头顺序来自 Sheet，Parse 时按索引读值。

```go
// modules/gsheet/mapper/ods_mapper.go
package mapper

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/dto"
)

// ExpectedHeaders 与 Sheet 表头严格对应（索引即列序）
var ExpectedHeaders = []string{
	"日期", "产品名称", "产品编号", "小组", "部门",
	"日活", "日活环比", "总充值", "总充值环比",
	"自然充值", "自然充值环比", "渠道充值", "渠道充值环比",
	"内导充值", "内导充值环比", "总新增", "总新增环比",
	"自然新增", "自然新增环比", "渠道新增", "渠道新增环比",
	"内导新增", "内导新增环比", "新充人数", "老充人数",
	"新用户充值", "老用户充值", "充值单数", "付款成功率",
	"次留率", "3留率", "7留率",
	"下载页访问数", "下载页点击数", "落地页下载率",
	"总转化", "新充转化", "老充转化", "老用户日活", "ARPPU",
}

// MapRow 把 CSV 一行（[]string）映射成 ODSRow。
// rowNo 是 1-based 数据行编号（不含表头）。
// warnings 收集宽容解析时遇到的问题；err 是致命错误（如列数严重不符）。
func MapRow(row []string, rowNo int, source string) (dto.ODSRow, []dto.ParseWarning, error) {
	minCols := len(ExpectedHeaders)
	if len(row) < minCols {
		return dto.ODSRow{}, nil, fmt.Errorf("row %d: got %d cols, need %d", rowNo, len(row), minCols)
	}

	var warns []dto.ParseWarning
	warn := func(field, raw, msg string) {
		warns = append(warns, dto.ParseWarning{RowNo: rowNo, Field: field, RawVal: raw, Message: msg})
	}

	parseF := func(field, s string) float64 {
		s = strings.TrimSpace(s)
		if s == "" || s == "-" {
			warn(field, s, "empty, defaulting to 0")
			return 0
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			warn(field, s, "not a number, defaulting to 0")
			return 0
		}
		return v
	}
	parseU := func(field, s string) uint64 {
		f := parseF(field, s)
		if f < 0 {
			warn(field, s, "negative uint, clamped to 0")
			return 0
		}
		return uint64(f)
	}
	parseDate := func(s string) (time.Time, error) {
		s = strings.TrimSpace(s)
		return time.Parse("2006-01-02", s)
	}

	date, err := parseDate(row[0])
	if err != nil {
		return dto.ODSRow{}, nil, fmt.Errorf("row %d: invalid date %q: %w", rowNo, row[0], err)
	}

	r := dto.ODSRow{
		Date:                  date,
		ProductName:           strings.TrimSpace(row[1]),
		ProductCode:           strings.TrimSpace(row[2]),
		Team:                  strings.TrimSpace(row[3]),
		BusinessUnit:          strings.TrimSpace(row[4]),
		DAU:                   parseU("日活", row[5]),
		DAUChain:              parseF("日活环比", row[6]),
		RechargeTotal:         parseF("总充值", row[7]),
		RechargeTotalChain:    parseF("总充值环比", row[8]),
		RechargeOrganic:       parseF("自然充值", row[9]),
		RechargeOrganicChain:  parseF("自然充值环比", row[10]),
		RechargeChannel:       parseF("渠道充值", row[11]),
		RechargeChannelChain:  parseF("渠道充值环比", row[12]),
		RechargeInternal:      parseF("内导充值", row[13]),
		RechargeInternalChain: parseF("内导充值环比", row[14]),
		NewUserTotal:          parseU("总新增", row[15]),
		NewUserTotalChain:     parseF("总新增环比", row[16]),
		NewUserOrganic:        parseU("自然新增", row[17]),
		NewUserOrganicChain:   parseF("自然新增环比", row[18]),
		NewUserChannel:        parseU("渠道新增", row[19]),
		NewUserChannelChain:   parseF("渠道新增环比", row[20]),
		NewUserInternal:       parseU("内导新增", row[21]),
		NewUserInternalChain:  parseF("内导新增环比", row[22]),
		NewPayingUserCnt:      parseU("新充人数", row[23]),
		OldPayingUserCnt:      parseU("老充人数", row[24]),
		NewPayingUserAmt:      parseF("新用户充值", row[25]),
		OldPayingUserAmt:      parseF("老用户充值", row[26]),
		RechargeOrderCnt:      parseU("充值单数", row[27]),
		PaymentSuccessRatio:   parseF("付款成功率", row[28]),
		RetentionD1:           parseF("次留率", row[29]),
		RetentionD3:           parseF("3留率", row[30]),
		RetentionD7:           parseF("7留率", row[31]),
		LandingPageVisit:      parseU("下载页访问数", row[32]),
		LandingPageClick:      parseU("下载页点击数", row[33]),
		LandingPageDownload:   parseF("落地页下载率", row[34]),
		ConversionTotal:       parseF("总转化", row[35]),
		ConversionNewPaying:   parseF("新充转化", row[36]),
		ConversionOldPaying:   parseF("老充转化", row[37]),
		OldUserDAU:            parseU("老用户日活", row[38]),
		ARPPU:                 parseF("ARPPU", row[39]),
		Source:                source,
		SrcRowNo:              uint32(rowNo),
		IngestedAt:            time.Now(),
	}
	return r, warns, nil
}
```

- [ ] **Step 4: 写单元测试**

```go
// modules/gsheet/mapper/ods_mapper_test.go
package mapper_test

import (
	"testing"

	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/mapper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapRow_HappyPath(t *testing.T) {
	row := []string{
		"2026-05-01", "海角乱伦", "JHA-031", "运营1组", "一部",
		"44702", "0.26753055", "116150", "2.78338762",
		"45450", "2.28158845", "112800", "2.96485062",
		"200", "0", "15578", "1.46370394",
		"5835", "2.34001145", "9667", "1.13682582",
		"3", "-0.625", "106", "538",
		"18950", "97200", "644", "0.67013528",
		"0.0863", "0.0451", "0.0226",
		"54744", "27775", "0.50736154",
		"7.456", "1.2165", "3.3375", "29124", "180.3571",
	}
	r, warns, err := mapper.MapRow(row, 1, "gsheet:test")
	require.NoError(t, err)
	assert.Empty(t, warns)
	assert.Equal(t, "JHA-031", r.ProductCode)
	assert.Equal(t, uint64(44702), r.DAU)
	assert.InDelta(t, 0.26753055, r.DAUChain, 1e-8)
	assert.InDelta(t, 180.3571, r.ARPPU, 1e-4)
}

func TestMapRow_EmptyNumericField(t *testing.T) {
	row := []string{
		"2026-05-01", "测试产品", "JHA-999", "运营1组", "一部",
		"", "0", "0", "0", "0", "0", "0", "0", "0", "0",
		"0", "0", "0", "0", "0", "0", "0", "0", "0", "0",
		"0", "0", "0", "0", "0", "0", "0", "0", "0", "0",
		"0", "0", "0", "0", "0",
	}
	r, warns, err := mapper.MapRow(row, 2, "gsheet:test")
	require.NoError(t, err)
	assert.Equal(t, uint64(0), r.DAU)
	assert.NotEmpty(t, warns) // 空字段应产生警告
}

func TestMapRow_InvalidDate(t *testing.T) {
	row := make([]string, 40)
	row[0] = "not-a-date"
	_, _, err := mapper.MapRow(row, 3, "gsheet:test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid date")
}

func TestMapRow_TooFewCols(t *testing.T) {
	row := []string{"2026-05-01", "产品", "JHA-001"}
	_, _, err := mapper.MapRow(row, 4, "gsheet:test")
	assert.Error(t, err)
}
```

- [ ] **Step 5: 运行测试验证**

```bash
cd /Users/moyucheng/work/tm/DataPilot
go test ./modules/gsheet/mapper/... -v
```

期望：4 个测试全部 PASS

- [ ] **Step 6: Commit**

```bash
git add modules/gsheet/dto/ modules/gsheet/mapper/
git commit -m "feat: add ODS DTO, ParseWarning, and CSV→struct mapper with tests"
```

---

## Task 3：ODS Repository（批量写入 CH）

**Files:**
- Create: `modules/gsheet/repository/ods_repo.go`

- [ ] **Step 1: 创建 `modules/gsheet/repository/ods_repo.go`**

```go
// modules/gsheet/repository/ods_repo.go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/dto"
)

type ODSRepo struct {
	conn driver.Conn
}

func NewODSRepo(conn driver.Conn) *ODSRepo {
	return &ODSRepo{conn: conn}
}

// BatchInsert 把 rows 用 PrepareBatch 批量写入 ods_product_daily_report
func (r *ODSRepo) BatchInsert(ctx context.Context, rows []dto.ODSRow) error {
	if len(rows) == 0 {
		return nil
	}
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	batch, err := r.conn.PrepareBatch(ctx2, `INSERT INTO ods_product_daily_report (
		` + "`日期`" + `,` + "`产品名称`" + `,` + "`产品编号`" + `,` + "`小组`" + `,` + "`部门`" + `,
		` + "`日活`" + `,` + "`日活环比`" + `,` + "`总充值`" + `,` + "`总充值环比`" + `,
		` + "`自然充值`" + `,` + "`自然充值环比`" + `,` + "`渠道充值`" + `,` + "`渠道充值环比`" + `,
		` + "`内导充值`" + `,` + "`内导充值环比`" + `,` + "`总新增`" + `,` + "`总新增环比`" + `,
		` + "`自然新增`" + `,` + "`自然新增环比`" + `,` + "`渠道新增`" + `,` + "`渠道新增环比`" + `,
		` + "`内导新增`" + `,` + "`内导新增环比`" + `,` + "`新充人数`" + `,` + "`老充人数`" + `,
		` + "`新用户充值`" + `,` + "`老用户充值`" + `,` + "`充值单数`" + `,` + "`付款成功率`" + `,
		` + "`次留率`" + `,` + "`3留率`" + `,` + "`7留率`" + `,
		` + "`下载页访问数`" + `,` + "`下载页点击数`" + `,` + "`落地页下载率`" + `,
		` + "`总转化`" + `,` + "`新充转化`" + `,` + "`老充转化`" + `,` + "`老用户日活`" + `,` + "`ARPPU`" + `,
		_source, _src_row_no, _ingested_at
	)`)
	if err != nil {
		return fmt.Errorf("prepare batch: %w", err)
	}

	for _, row := range rows {
		if err := batch.Append(
			row.Date, row.ProductName, row.ProductCode, row.Team, row.BusinessUnit,
			row.DAU, row.DAUChain, row.RechargeTotal, row.RechargeTotalChain,
			row.RechargeOrganic, row.RechargeOrganicChain,
			row.RechargeChannel, row.RechargeChannelChain,
			row.RechargeInternal, row.RechargeInternalChain,
			row.NewUserTotal, row.NewUserTotalChain,
			row.NewUserOrganic, row.NewUserOrganicChain,
			row.NewUserChannel, row.NewUserChannelChain,
			row.NewUserInternal, row.NewUserInternalChain,
			row.NewPayingUserCnt, row.OldPayingUserCnt,
			row.NewPayingUserAmt, row.OldPayingUserAmt,
			row.RechargeOrderCnt, row.PaymentSuccessRatio,
			row.RetentionD1, row.RetentionD3, row.RetentionD7,
			row.LandingPageVisit, row.LandingPageClick, row.LandingPageDownload,
			row.ConversionTotal, row.ConversionNewPaying, row.ConversionOldPaying,
			row.OldUserDAU, row.ARPPU,
			row.Source, row.SrcRowNo, row.IngestedAt,
		); err != nil {
			return fmt.Errorf("append row %d: %w", row.SrcRowNo, err)
		}
	}
	return batch.Send()
}
```

- [ ] **Step 2: Commit**

```bash
git add modules/gsheet/repository/ods_repo.go
git commit -m "feat: add ODS repository with PrepareBatch insert"
```

---

## Task 4：Fetcher + ODS Loader Service

**Files:**
- Create: `modules/gsheet/service/fetcher.go`
- Create: `modules/gsheet/service/ods_loader.go`

- [ ] **Step 1: 创建 `modules/gsheet/service/fetcher.go`**

```go
// modules/gsheet/service/fetcher.go
package service

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchPublicSheetCSV 拉取公开 Google Sheet 的 CSV 内容
// spreadsheetID: Sheet 的 ID（URL 中 /d/ 后面的部分）
// gid: 子 sheet 的 gid（URL 中 #gid= 的值，0 代表第一张）
func FetchPublicSheetCSV(spreadsheetID string, gid int) ([][]string, error) {
	url := fmt.Sprintf(
		"https://docs.google.com/spreadsheets/d/%s/export?format=csv&gid=%d",
		spreadsheetID, gid,
	)

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if strings.HasPrefix(req.URL.Host, "accounts.google.") {
				return fmt.Errorf("sheet is not publicly accessible (redirected to login)")
			}
			return nil
		},
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch sheet: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode, string(body))
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" &&
		!strings.HasPrefix(ct, "text/csv") &&
		!strings.HasPrefix(ct, "application/csv") {
		return nil, fmt.Errorf("unexpected Content-Type %q; sheet may not be public", ct)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) // strip BOM

	r := csv.NewReader(bytes.NewReader(raw))
	r.FieldsPerRecord = -1
	return r.ReadAll()
}
```

- [ ] **Step 2: 创建 `modules/gsheet/service/ods_loader.go`**

```go
// modules/gsheet/service/ods_loader.go
package service

import (
	"context"
	"fmt"
	"log"

	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/dto"
	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/mapper"
	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/repository"
)

const maxWarnRatio = 0.05 // 超过 5% 异常行则 abort

// ODSLoader 把 Google Sheet 数据加载到 ODS 表
type ODSLoader struct {
	odsRepo *repository.ODSRepo
}

func NewODSLoader(odsRepo *repository.ODSRepo) *ODSLoader {
	return &ODSLoader{odsRepo: odsRepo}
}

// LoadOptions 控制加载行为
type LoadOptions struct {
	SpreadsheetID string
	GID           int
	Source        string // 写入 _source 字段，如 "gsheet:测试-写入"
}

// Load 拉取 CSV → 解析 → 写入 ODS
func (l *ODSLoader) Load(ctx context.Context, opts LoadOptions) error {
	rows, err := FetchPublicSheetCSV(opts.SpreadsheetID, opts.GID)
	if err != nil {
		return fmt.Errorf("fetch CSV: %w", err)
	}
	if len(rows) < 2 {
		log.Printf("[ods-loader] no data rows (got %d)", len(rows))
		return nil
	}

	header := rows[0]
	if err := validateHeader(header); err != nil {
		return err
	}

	var odsRows []dto.ODSRow
	var totalWarns int
	dataRows := rows[1:]

	for i, row := range dataRows {
		odsRow, warns, err := mapper.MapRow(row, i+1, opts.Source)
		if err != nil {
			log.Printf("[ods-loader] WARN skip row %d: %v", i+1, err)
			totalWarns++
			continue
		}
		for _, w := range warns {
			log.Printf("[ods-loader] WARN %s", w.Error())
		}
		totalWarns += len(warns)
		odsRows = append(odsRows, odsRow)
	}

	if float64(totalWarns)/float64(len(dataRows)) > maxWarnRatio {
		return fmt.Errorf("abort: warn ratio %.1f%% exceeds threshold %.1f%%",
			float64(totalWarns)/float64(len(dataRows))*100, maxWarnRatio*100)
	}

	log.Printf("[ods-loader] writing %d rows to ODS (source=%s)", len(odsRows), opts.Source)
	return l.odsRepo.BatchInsert(ctx, odsRows)
}

func validateHeader(header []string) error {
	if len(header) < len(mapper.ExpectedHeaders) {
		return fmt.Errorf("header has %d cols, expected at least %d", len(header), len(mapper.ExpectedHeaders))
	}
	for i, expected := range mapper.ExpectedHeaders {
		if header[i] != expected {
			return fmt.Errorf("header col %d: got %q, expected %q", i, header[i], expected)
		}
	}
	return nil
}
```

- [ ] **Step 3: Commit**

```bash
git add modules/gsheet/service/fetcher.go modules/gsheet/service/ods_loader.go
git commit -m "feat: add CSV fetcher and ODS loader service"
```

---

## Task 5：ETL SQL 文件 + ETL Runner Service

**Files:**
- Create: `database/clickhouse/etl/ods_to_dim.sql`
- Create: `database/clickhouse/etl/ods_to_dwd.sql`
- Create: `database/clickhouse/etl/dwd_to_dws_product.sql`
- Create: `database/clickhouse/etl/dwd_to_dws_team.sql`
- Create: `database/clickhouse/etl/dwd_to_dws_bu.sql`
- Create: `modules/gsheet/service/etl_runner.go`

- [ ] **Step 1: 创建 `database/clickhouse/etl/ods_to_dim.sql`**

```sql
-- database/clickhouse/etl/ods_to_dim.sql
INSERT INTO dim_product (product_code, product_name, team, business_unit, `_source`, `_ingested_at`)
SELECT DISTINCT
    `产品编号`  AS product_code,
    `产品名称`  AS product_name,
    `小组`      AS team,
    `部门`      AS business_unit,
    `_source`,
    now64()
FROM ods_product_daily_report FINAL
WHERE `_source` = {source:String};
```

- [ ] **Step 2: 创建 `database/clickhouse/etl/ods_to_dwd.sql`**

```sql
-- database/clickhouse/etl/ods_to_dwd.sql
INSERT INTO dwd_product_daily_metric
SELECT
    `日期`             AS date,
    `产品编号`         AS product_code,
    `日活`             AS dau,
    `日活环比`         AS dau_chain_ratio,
    `老用户日活`       AS old_user_dau,
    `总充值`           AS recharge_total_amt,
    `总充值环比`       AS recharge_total_chain_ratio,
    `自然充值`         AS recharge_organic_amt,
    `自然充值环比`     AS recharge_organic_chain_ratio,
    `渠道充值`         AS recharge_channel_amt,
    `渠道充值环比`     AS recharge_channel_chain_ratio,
    `内导充值`         AS recharge_internal_amt,
    `内导充值环比`     AS recharge_internal_chain_ratio,
    `总新增`           AS new_user_total_cnt,
    `总新增环比`       AS new_user_total_chain_ratio,
    `自然新增`         AS new_user_organic_cnt,
    `自然新增环比`     AS new_user_organic_chain_ratio,
    `渠道新增`         AS new_user_channel_cnt,
    `渠道新增环比`     AS new_user_channel_chain_ratio,
    `内导新增`         AS new_user_internal_cnt,
    `内导新增环比`     AS new_user_internal_chain_ratio,
    `新充人数`         AS new_paying_user_cnt,
    `老充人数`         AS old_paying_user_cnt,
    `新用户充值`       AS new_paying_user_amt,
    `老用户充值`       AS old_paying_user_amt,
    `充值单数`         AS recharge_order_cnt,
    `付款成功率`       AS payment_success_ratio,
    `次留率`           AS retention_d1_ratio,
    `3留率`            AS retention_d3_ratio,
    `7留率`            AS retention_d7_ratio,
    `下载页访问数`     AS landing_page_visit_cnt,
    `下载页点击数`     AS landing_page_click_cnt,
    `落地页下载率`     AS landing_page_download_ratio,
    `总转化`           AS conversion_total_multi,
    `新充转化`         AS conversion_new_paying_multi,
    `老充转化`         AS conversion_old_paying_multi,
    `ARPPU`            AS arppu,
    `_src_row_no`,
    now64()            AS `_ingested_at`
FROM ods_product_daily_report FINAL
WHERE `_source` = {source:String}
  AND `日期` >= {from_date:Date}
  AND `日期` <= {to_date:Date};
```

- [ ] **Step 3: 创建 `database/clickhouse/etl/dwd_to_dws_product.sql`**

```sql
-- database/clickhouse/etl/dwd_to_dws_product.sql
-- Step A: 删除目标月份分区（幂等）
ALTER TABLE dws_product_daily DROP PARTITION {partition:String};

-- Step B: 重算写入
INSERT INTO dws_product_daily
SELECT
    d.date, d.product_code,
    coalesce(p.product_name, d.product_code) AS product_name,
    coalesce(p.team, '')                      AS team,
    coalesce(p.business_unit, '')             AS business_unit,
    d.dau, d.dau_chain_ratio, d.old_user_dau,
    d.recharge_total_amt, d.recharge_total_chain_ratio,
    d.recharge_organic_amt, d.recharge_organic_chain_ratio,
    d.recharge_channel_amt, d.recharge_channel_chain_ratio,
    d.recharge_internal_amt, d.recharge_internal_chain_ratio,
    d.new_user_total_cnt, d.new_user_total_chain_ratio,
    d.new_user_organic_cnt, d.new_user_organic_chain_ratio,
    d.new_user_channel_cnt, d.new_user_channel_chain_ratio,
    d.new_user_internal_cnt, d.new_user_internal_chain_ratio,
    d.new_paying_user_cnt, d.old_paying_user_cnt,
    d.new_paying_user_amt, d.old_paying_user_amt,
    d.recharge_order_cnt, d.payment_success_ratio,
    d.retention_d1_ratio, d.retention_d3_ratio, d.retention_d7_ratio,
    d.landing_page_visit_cnt, d.landing_page_click_cnt, d.landing_page_download_ratio,
    d.conversion_total_multi, d.conversion_new_paying_multi, d.conversion_old_paying_multi,
    d.arppu,
    now64() AS `_built_at`
FROM dwd_product_daily_metric d FINAL
LEFT JOIN dim_product p FINAL USING (product_code)
WHERE d.date >= {from_date:Date} AND d.date <= {to_date:Date};
```

- [ ] **Step 4: 创建 `database/clickhouse/etl/dwd_to_dws_team.sql`**

```sql
-- database/clickhouse/etl/dwd_to_dws_team.sql
ALTER TABLE dws_team_daily DROP PARTITION {partition:String};

INSERT INTO dws_team_daily
SELECT
    d.date,
    coalesce(p.team, '') AS team,
    coalesce(p.business_unit, '') AS business_unit,
    count()                            AS active_product_cnt,
    sum(d.dau)                         AS dau_sum,
    sum(d.recharge_total_amt)          AS recharge_total_amt_sum,
    sum(d.recharge_organic_amt)        AS recharge_organic_amt_sum,
    sum(d.recharge_channel_amt)        AS recharge_channel_amt_sum,
    sum(d.recharge_internal_amt)       AS recharge_internal_amt_sum,
    sum(d.new_user_total_cnt)          AS new_user_total_cnt_sum,
    sum(d.new_user_organic_cnt)        AS new_user_organic_cnt_sum,
    sum(d.new_user_channel_cnt)        AS new_user_channel_cnt_sum,
    sum(d.new_paying_user_cnt)         AS new_paying_user_cnt_sum,
    sum(d.old_paying_user_cnt)         AS old_paying_user_cnt_sum,
    sum(d.recharge_order_cnt)          AS recharge_order_cnt_sum,
    sum(d.old_user_dau)                AS old_user_dau_sum,
    sum(d.dau * d.retention_d7_ratio)  AS retention_d7_ratio_num,
    sum(d.dau)                         AS retention_d7_ratio_den,
    now64()
FROM dwd_product_daily_metric d FINAL
LEFT JOIN dim_product p FINAL USING (product_code)
WHERE d.date >= {from_date:Date} AND d.date <= {to_date:Date}
GROUP BY d.date, p.team, p.business_unit;
```

- [ ] **Step 5: 创建 `database/clickhouse/etl/dwd_to_dws_bu.sql`**

```sql
-- database/clickhouse/etl/dwd_to_dws_bu.sql
ALTER TABLE dws_bu_daily DROP PARTITION {partition:String};

INSERT INTO dws_bu_daily
SELECT
    d.date,
    coalesce(p.business_unit, '') AS business_unit,
    count()                            AS active_product_cnt,
    sum(d.dau)                         AS dau_sum,
    sum(d.recharge_total_amt)          AS recharge_total_amt_sum,
    sum(d.recharge_organic_amt)        AS recharge_organic_amt_sum,
    sum(d.recharge_channel_amt)        AS recharge_channel_amt_sum,
    sum(d.recharge_internal_amt)       AS recharge_internal_amt_sum,
    sum(d.new_user_total_cnt)          AS new_user_total_cnt_sum,
    sum(d.new_user_organic_cnt)        AS new_user_organic_cnt_sum,
    sum(d.new_user_channel_cnt)        AS new_user_channel_cnt_sum,
    sum(d.new_paying_user_cnt)         AS new_paying_user_cnt_sum,
    sum(d.old_paying_user_cnt)         AS old_paying_user_cnt_sum,
    sum(d.recharge_order_cnt)          AS recharge_order_cnt_sum,
    sum(d.old_user_dau)                AS old_user_dau_sum,
    sum(d.dau * d.retention_d7_ratio)  AS retention_d7_ratio_num,
    sum(d.dau)                         AS retention_d7_ratio_den,
    now64()
FROM dwd_product_daily_metric d FINAL
LEFT JOIN dim_product p FINAL USING (product_code)
WHERE d.date >= {from_date:Date} AND d.date <= {to_date:Date}
GROUP BY d.date, p.business_unit;
```

- [ ] **Step 6: 创建 `modules/gsheet/service/etl_runner.go`**

```go
// modules/gsheet/service/etl_runner.go
package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/repository"
)

// ETLRunner 按依赖顺序执行 ODS→DIM→DWD→DWS→ADS 的 ETL
type ETLRunner struct {
	etlRepo *repository.EtlRepo
	etlDir  string // database/clickhouse/etl 目录路径
}

func NewETLRunner(etlRepo *repository.EtlRepo, etlDir string) *ETLRunner {
	return &ETLRunner{etlRepo: etlRepo, etlDir: etlDir}
}

// RunDIM 更新维表
func (e *ETLRunner) RunDIM(ctx context.Context, source string) error {
	log.Printf("[etl] running DIM (source=%s)", source)
	return e.etlRepo.ExecETL(ctx, e.etlDir+"/ods_to_dim.sql", map[string]interface{}{
		"source": source,
	})
}

// RunDWD 运行 ODS→DWD，按日期范围
func (e *ETLRunner) RunDWD(ctx context.Context, source string, from, to time.Time) error {
	log.Printf("[etl] running DWD (source=%s from=%s to=%s)", source, from.Format("2006-01-02"), to.Format("2006-01-02"))
	return e.etlRepo.ExecETL(ctx, e.etlDir+"/ods_to_dwd.sql", map[string]interface{}{
		"source":    source,
		"from_date": from,
		"to_date":   to,
	})
}

// RunDWS 运行 DWD→DWS（三张宽表），按月分区重算
func (e *ETLRunner) RunDWS(ctx context.Context, from, to time.Time) error {
	// 枚举涉及的月份分区
	partitions := monthPartitions(from, to)
	for _, part := range partitions {
		log.Printf("[etl] running DWS product partition=%s", part)
		if err := e.etlRepo.ExecETL(ctx, e.etlDir+"/dwd_to_dws_product.sql", map[string]interface{}{
			"partition": part,
			"from_date": from,
			"to_date":   to,
		}); err != nil {
			return fmt.Errorf("dws_product partition %s: %w", part, err)
		}

		log.Printf("[etl] running DWS team partition=%s", part)
		if err := e.etlRepo.ExecETL(ctx, e.etlDir+"/dwd_to_dws_team.sql", map[string]interface{}{
			"partition": part,
			"from_date": from,
			"to_date":   to,
		}); err != nil {
			return fmt.Errorf("dws_team partition %s: %w", part, err)
		}

		log.Printf("[etl] running DWS bu partition=%s", part)
		if err := e.etlRepo.ExecETL(ctx, e.etlDir+"/dwd_to_dws_bu.sql", map[string]interface{}{
			"partition": part,
			"from_date": from,
			"to_date":   to,
		}); err != nil {
			return fmt.Errorf("dws_bu partition %s: %w", part, err)
		}
	}
	return nil
}

// monthPartitions 返回 from→to 之间涉及的所有年月字符串（格式 "YYYYMM"）
func monthPartitions(from, to time.Time) []string {
	var parts []string
	cur := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC)
	for !cur.After(end) {
		parts = append(parts, cur.Format("200601"))
		cur = cur.AddDate(0, 1, 0)
	}
	return parts
}
```

- [ ] **Step 7: Commit**

```bash
git add database/clickhouse/etl/ modules/gsheet/service/etl_runner.go
git commit -m "feat: add ETL SQL templates and ETLRunner service"
```

---

## Task 6：DQ Checker

**Files:**
- Create: `modules/gsheet/service/dq_checker.go`

- [ ] **Step 1: 创建 `modules/gsheet/service/dq_checker.go`**

```go
// modules/gsheet/service/dq_checker.go
package service

import (
	"context"
	"log"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

// DQChecker 在 ETL 后执行基础数据质量检查，结果只记日志不阻塞
type DQChecker struct {
	conn driver.Conn
}

func NewDQChecker(conn driver.Conn) *DQChecker {
	return &DQChecker{conn: conn}
}

// CheckDWD 对指定日期范围的 dwd_product_daily_metric 做三条规则检查
func (d *DQChecker) CheckDWD(ctx context.Context, from, to time.Time) {
	ctx2, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// 规则 1：主键唯一性
	var dupCnt uint64
	_ = d.conn.QueryRow(ctx2, `
		SELECT count() - countDistinct(date, product_code) AS dup_cnt
		FROM dwd_product_daily_metric FINAL
		WHERE date >= {from_date:Date} AND date <= {to_date:Date}`,
		driver.NamedValue{Name: "from_date", Value: from},
		driver.NamedValue{Name: "to_date", Value: to},
	).Scan(&dupCnt)
	if dupCnt > 0 {
		log.Printf("[dq] WARN dwd duplicate (date,product_code) count=%d", dupCnt)
	}

	// 规则 2：负值检查
	var negCnt uint64
	_ = d.conn.QueryRow(ctx2, `
		SELECT count() FROM dwd_product_daily_metric FINAL
		WHERE date >= {from_date:Date} AND date <= {to_date:Date}
		  AND (dau < 0 OR recharge_total_amt < 0)`,
		driver.NamedValue{Name: "from_date", Value: from},
		driver.NamedValue{Name: "to_date", Value: to},
	).Scan(&negCnt)
	if negCnt > 0 {
		log.Printf("[dq] WARN dwd negative value rows count=%d", negCnt)
	}

	// 规则 3：环比异常范围（超过 50 倍视为异常）
	var outCnt uint64
	_ = d.conn.QueryRow(ctx2, `
		SELECT count() FROM dwd_product_daily_metric FINAL
		WHERE date >= {from_date:Date} AND date <= {to_date:Date}
		  AND (dau_chain_ratio < -1 OR dau_chain_ratio > 50)`,
		driver.NamedValue{Name: "from_date", Value: from},
		driver.NamedValue{Name: "to_date", Value: to},
	).Scan(&outCnt)
	if outCnt > 0 {
		log.Printf("[dq] WARN dwd out-of-range dau_chain_ratio rows count=%d", outCnt)
	}

	log.Printf("[dq] DWD quality check done (from=%s to=%s)", from.Format("2006-01-02"), to.Format("2006-01-02"))
}
```

- [ ] **Step 2: Commit**

```bash
git add modules/gsheet/service/dq_checker.go
git commit -m "feat: add DQ checker (dedup, negative, out-of-range)"
```

---

## Task 7：DI 注入 + CLI 子命令入口

**Files:**
- Create: `cmd/sync_gsheet.go`
- Modify: `cmd/main.go`
- Modify: `providers/core.go`
- Modify: `pkg/constants/common.go`
- Modify: `Makefile`

- [ ] **Step 1: 在 `pkg/constants/common.go` 追加 DI key**

```go
// pkg/constants/common.go（在现有 GoogleDriveService 后追加）
const (
    // ... 已有常量保留 ...

    GSheetODSRepo   = "gsheetODSRepo"
    GSheetETLRepo   = "gsheetETLRepo"
    GSheetODSLoader = "gsheetODSLoader"
    GSheetETLRunner = "gsheetETLRunner"
    GSheetDQChecker = "gsheetDQChecker"
)
```

- [ ] **Step 2: 在 `providers/core.go` 注入新 service**

```go
// providers/core.go — 在 RegisterDependencies 中追加
import (
    // ... 已有 import ...
    gsheetRepo    "github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/repository"
    gsheetService "github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/service"
)

func InitGSheetServices(injector *do.Injector) {
    do.ProvideNamed(injector, constants.GSheetODSRepo, func(i *do.Injector) (*gsheetRepo.ODSRepo, error) {
        ch := do.MustInvokeNamed[driver.Conn](i, constants.ClickHouseDB)
        return gsheetRepo.NewODSRepo(ch), nil
    })
    do.ProvideNamed(injector, constants.GSheetETLRepo, func(i *do.Injector) (*gsheetRepo.EtlRepo, error) {
        ch := do.MustInvokeNamed[driver.Conn](i, constants.ClickHouseDB)
        return gsheetRepo.NewEtlRepo(ch), nil
    })
    do.ProvideNamed(injector, constants.GSheetODSLoader, func(i *do.Injector) (*gsheetService.ODSLoader, error) {
        repo := do.MustInvokeNamed[*gsheetRepo.ODSRepo](i, constants.GSheetODSRepo)
        return gsheetService.NewODSLoader(repo), nil
    })
    do.ProvideNamed(injector, constants.GSheetETLRunner, func(i *do.Injector) (*gsheetService.ETLRunner, error) {
        repo := do.MustInvokeNamed[*gsheetRepo.EtlRepo](i, constants.GSheetETLRepo)
        return gsheetService.NewETLRunner(repo, "database/clickhouse/etl"), nil
    })
    do.ProvideNamed(injector, constants.GSheetDQChecker, func(i *do.Injector) (*gsheetService.DQChecker, error) {
        ch := do.MustInvokeNamed[driver.Conn](i, constants.ClickHouseDB)
        return gsheetService.NewDQChecker(ch), nil
    })
}

// 在 RegisterDependencies 末尾追加
func RegisterDependencies(injector *do.Injector) {
    InitDatabase(injector)
    InitGoogleServices(injector)
    InitGSheetServices(injector)  // 新增
    // ... 已有 JWT / user / auth ...
}
```

- [ ] **Step 3: 创建 `cmd/sync_gsheet.go`**

```go
// cmd/sync_gsheet.go
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	gsheetRepo    "github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/repository"
	gsheetService "github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/service"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/constants"
	"github.com/samber/do"
)

const (
	defaultSpreadsheetID = "1sJZBMAHBa3QtmSKQ8oLGaiC_w-8kloGJizRNHnbQ-jw"
	defaultSource        = "gsheet:测试-写入"
	defaultGID           = 0
)

// runCHMigrate 按编号顺序执行 CH migration SQL 文件
func runCHMigrate(injector *do.Injector) {
	etlRepo := do.MustInvokeNamed[*gsheetRepo.EtlRepo](injector, constants.GSheetETLRepo)
	if err := etlRepo.RunMigrations(context.Background(), "database/clickhouse/migrations"); err != nil {
		log.Fatalf("[ch-migrate] FAILED: %v", err)
	}
	log.Println("[ch-migrate] all migrations applied successfully")
}

// runSyncGSheet 根据 flag 分发到各个 sync 子命令
func runSyncGSheet(injector *do.Injector, flag string) {
	fs := flag_parse()

	loader := do.MustInvokeNamed[*gsheetService.ODSLoader](injector, constants.GSheetODSLoader)
	runner := do.MustInvokeNamed[*gsheetService.ETLRunner](injector, constants.GSheetETLRunner)
	checker := do.MustInvokeNamed[*gsheetService.DQChecker](injector, constants.GSheetDQChecker)

	ctx := context.Background()
	from, to := fs.from, fs.to

	switch flag {
	case "--sync:ods":
		runODS(ctx, loader, fs)
	case "--sync:dim":
		runDIM(ctx, runner, fs.source)
	case "--sync:dwd":
		runDWD(ctx, runner, checker, fs.source, from, to)
	case "--sync:dws":
		runDWS(ctx, runner, from, to)
	case "--sync:ads":
		log.Println("[sync-ads] ADS views are created by ch-migrate; nothing to run here.")
	case "--sync:all":
		runODS(ctx, loader, fs)
		runDIM(ctx, runner, fs.source)
		runDWD(ctx, runner, checker, fs.source, from, to)
		runDWS(ctx, runner, from, to)
		log.Println("[sync-all] done")
	}
}

type syncFlags struct {
	spreadsheetID string
	gid           int
	source        string
	from          time.Time
	to            time.Time
}

func flag_parse() syncFlags {
	args := os.Args[2:] // skip binary and the --sync:xxx flag
	fs := flag.NewFlagSet("sync", flag.ExitOnError)
	spreadsheetID := fs.String("id", defaultSpreadsheetID, "Google Spreadsheet ID")
	gid := fs.Int("gid", defaultGID, "Sheet GID (tab index)")
	source := fs.String("source", defaultSource, "_source value written to ODS")
	fromStr := fs.String("from", time.Now().AddDate(0, 0, -7).Format("2006-01-02"), "from date (YYYY-MM-DD)")
	toStr := fs.String("to", time.Now().Format("2006-01-02"), "to date (YYYY-MM-DD)")
	_ = fs.Parse(args)

	from, _ := time.Parse("2006-01-02", *fromStr)
	to, _ := time.Parse("2006-01-02", *toStr)
	return syncFlags{
		spreadsheetID: *spreadsheetID,
		gid:           *gid,
		source:        *source,
		from:          from,
		to:            to,
	}
}

func runODS(ctx context.Context, loader *gsheetService.ODSLoader, fs syncFlags) {
	log.Printf("[sync-ods] loading sheet id=%s gid=%d source=%s", fs.spreadsheetID, fs.gid, fs.source)
	if err := loader.Load(ctx, gsheetService.LoadOptions{
		SpreadsheetID: fs.spreadsheetID,
		GID:           fs.gid,
		Source:        fs.source,
	}); err != nil {
		log.Fatalf("[sync-ods] FAILED: %v", err)
	}
	log.Println("[sync-ods] done")
}

func runDIM(ctx context.Context, runner *gsheetService.ETLRunner, source string) {
	log.Printf("[sync-dim] source=%s", source)
	if err := runner.RunDIM(ctx, source); err != nil {
		log.Fatalf("[sync-dim] FAILED: %v", err)
	}
	log.Println("[sync-dim] done")
}

func runDWD(ctx context.Context, runner *gsheetService.ETLRunner, checker *gsheetService.DQChecker, source string, from, to time.Time) {
	log.Printf("[sync-dwd] source=%s from=%s to=%s", source, from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err := runner.RunDWD(ctx, source, from, to); err != nil {
		log.Fatalf("[sync-dwd] FAILED: %v", err)
	}
	checker.CheckDWD(ctx, from, to)
	log.Println("[sync-dwd] done")
}

func runDWS(ctx context.Context, runner *gsheetService.ETLRunner, from, to time.Time) {
	log.Printf("[sync-dws] from=%s to=%s", from.Format("2006-01-02"), to.Format("2006-01-02"))
	if err := runner.RunDWS(ctx, from, to); err != nil {
		log.Fatalf("[sync-dws] FAILED: %v", err)
	}
	log.Println("[sync-dws] done")
}
```

- [ ] **Step 4: 在 Makefile 追加 sync-* targets**

```makefile
# ClickHouse migration
ch-migrate:
	go run ./cmd --ch:migrate

# GSheet 同步命令（可追加 -id=xxx -from=YYYY-MM-DD -to=YYYY-MM-DD 等参数）
sync-ods:
	go run ./cmd --sync:ods $(ARGS)

sync-dim:
	go run ./cmd --sync:dim $(ARGS)

sync-dwd:
	go run ./cmd --sync:dwd $(ARGS)

sync-dws:
	go run ./cmd --sync:dws $(ARGS)

sync-ads:
	go run ./cmd --sync:ads

sync-all:
	go run ./cmd --sync:all $(ARGS)
```

- [ ] **Step 5: 编译验证**

```bash
go build ./...
```

期望：无报错

- [ ] **Step 6: Commit**

```bash
git add cmd/sync_gsheet.go cmd/main.go providers/core.go pkg/constants/common.go Makefile
git commit -m "feat: wire DI for gsheet services and add sync-* CLI commands"
```

---

## Task 8：PG 语义层 seed

**Files:**
- Create: `database/pg_seeds/001_meta_metric_dict.sql`

- [ ] **Step 1: 创建 PG 建表 + seed SQL**

```sql
-- database/pg_seeds/001_meta_metric_dict.sql
CREATE TABLE IF NOT EXISTS meta_metric_dict (
    id              BIGSERIAL PRIMARY KEY,
    metric_code     VARCHAR(128)  UNIQUE NOT NULL,
    metric_name_zh  VARCHAR(128)  NOT NULL,
    metric_name_en  VARCHAR(128)  NOT NULL,
    unit            VARCHAR(16)   NOT NULL,
    formula         TEXT,
    layer           VARCHAR(8)    NOT NULL,
    owner_team      VARCHAR(64),
    description     TEXT,
    is_active       BOOLEAN DEFAULT TRUE,
    created_at      TIMESTAMP DEFAULT now(),
    updated_at      TIMESTAMP DEFAULT now()
);

INSERT INTO meta_metric_dict (metric_code, metric_name_zh, metric_name_en, unit, layer) VALUES
  ('dau',                          '日活',       'Daily Active Users',              'cnt',    'DWD'),
  ('dau_chain_ratio',              '日活环比',   'DAU Chain Ratio',                 'ratio',  'DWD'),
  ('old_user_dau',                 '老用户日活', 'Old User DAU',                    'cnt',    'DWD'),
  ('recharge_total_amt',           '总充值',     'Total Recharge Amount',           'amount', 'DWD'),
  ('recharge_total_chain_ratio',   '总充值环比', 'Total Recharge Chain Ratio',      'ratio',  'DWD'),
  ('recharge_organic_amt',         '自然充值',   'Organic Recharge Amount',         'amount', 'DWD'),
  ('recharge_organic_chain_ratio', '自然充值环比','Organic Recharge Chain Ratio',   'ratio',  'DWD'),
  ('recharge_channel_amt',         '渠道充值',   'Channel Recharge Amount',         'amount', 'DWD'),
  ('recharge_channel_chain_ratio', '渠道充值环比','Channel Recharge Chain Ratio',   'ratio',  'DWD'),
  ('recharge_internal_amt',        '内导充值',   'Internal Recharge Amount',        'amount', 'DWD'),
  ('recharge_internal_chain_ratio','内导充值环比','Internal Recharge Chain Ratio',  'ratio',  'DWD'),
  ('new_user_total_cnt',           '总新增',     'Total New Users',                 'cnt',    'DWD'),
  ('new_user_total_chain_ratio',   '总新增环比', 'Total New Users Chain Ratio',     'ratio',  'DWD'),
  ('new_user_organic_cnt',         '自然新增',   'Organic New Users',               'cnt',    'DWD'),
  ('new_user_organic_chain_ratio', '自然新增环比','Organic New Users Chain Ratio',  'ratio',  'DWD'),
  ('new_user_channel_cnt',         '渠道新增',   'Channel New Users',               'cnt',    'DWD'),
  ('new_user_channel_chain_ratio', '渠道新增环比','Channel New Users Chain Ratio',  'ratio',  'DWD'),
  ('new_user_internal_cnt',        '内导新增',   'Internal New Users',              'cnt',    'DWD'),
  ('new_user_internal_chain_ratio','内导新增环比','Internal New Users Chain Ratio', 'ratio',  'DWD'),
  ('new_paying_user_cnt',          '新充人数',   'New Paying Users',                'cnt',    'DWD'),
  ('old_paying_user_cnt',          '老充人数',   'Old Paying Users',                'cnt',    'DWD'),
  ('new_paying_user_amt',          '新用户充值', 'New User Recharge Amount',        'amount', 'DWD'),
  ('old_paying_user_amt',          '老用户充值', 'Old User Recharge Amount',        'amount', 'DWD'),
  ('recharge_order_cnt',           '充值单数',   'Recharge Order Count',            'cnt',    'DWD'),
  ('payment_success_ratio',        '付款成功率', 'Payment Success Ratio',           'ratio',  'DWD'),
  ('retention_d1_ratio',           '次留率',     'D1 Retention Ratio',              'ratio',  'DWD'),
  ('retention_d3_ratio',           '3留率',      'D3 Retention Ratio',              'ratio',  'DWD'),
  ('retention_d7_ratio',           '7留率',      'D7 Retention Ratio',              'ratio',  'DWD'),
  ('landing_page_visit_cnt',       '下载页访问数','Landing Page Visit Count',       'cnt',    'DWD'),
  ('landing_page_click_cnt',       '下载页点击数','Landing Page Click Count',       'cnt',    'DWD'),
  ('landing_page_download_ratio',  '落地页下载率','Landing Page Download Ratio',    'ratio',  'DWD'),
  ('conversion_total_multi',       '总转化',     'Total Conversion Multiplier',     'multi',  'DWD'),
  ('conversion_new_paying_multi',  '新充转化',   'New Paying Conversion Multi',     'multi',  'DWD'),
  ('conversion_old_paying_multi',  '老充转化',   'Old Paying Conversion Multi',     'multi',  'DWD'),
  ('arppu',                        'ARPPU',      'Avg Revenue Per Paying User',     'amount', 'DWD')
ON CONFLICT (metric_code) DO NOTHING;
```

- [ ] **Step 2: 在 Makefile 加 seed target**

```makefile
seed-meta:
	psql -h $(DB_HOST) -U $(DB_USER) -d $(DB_NAME) -f database/pg_seeds/001_meta_metric_dict.sql
```

- [ ] **Step 3: 执行 seed**

```bash
make seed-meta
```

期望：35 行 INSERT 成功（`INSERT 0 35`）

- [ ] **Step 4: Commit**

```bash
git add database/pg_seeds/ Makefile
git commit -m "feat: add PG meta_metric_dict table and 35-row seed"
```

---

## Task 9：端到端验证

- [ ] **Step 1: 启动依赖**

```bash
docker ps | grep -E "postgres-server|clickhouse-server"
# 确认两个容器都在 Up 状态
```

- [ ] **Step 2: 建 CH 表**

```bash
make ch-migrate
```

期望：7 个 SQL 文件全部 applied，无报错

- [ ] **Step 3: 跑完整同步链路**

```bash
make sync-all
```

期望日志（顺序）：
```
[sync-ods] loading sheet id=1sJZBMAHBa3QtmSKQ8oLGaiC_w-8kloGJizRNHnbQ-jw gid=0 source=gsheet:测试-写入
[ods-loader] writing 235 rows to ODS (source=gsheet:测试-写入)
[sync-ods] done
[sync-dim] source=gsheet:测试-写入
[etl] running DIM (source=gsheet:测试-写入)
[sync-dim] done
[sync-dwd] source=gsheet:测试-写入 from=... to=...
[etl] running DWD ...
[dq] DWD quality check done
[sync-dwd] done
[etl] running DWS product partition=202603
[etl] running DWS team partition=202603
[etl] running DWS bu partition=202603
...
[sync-dws] done
[sync-all] done
```

- [ ] **Step 4: 用 clickhouse-client 验证数据**

```bash
docker exec -it clickhouse-server clickhouse-client --password 123456 \
  --query "SELECT count() FROM ods_product_daily_report FINAL"
# 期望: 235

docker exec -it clickhouse-server clickhouse-client --password 123456 \
  --query "SELECT count() FROM dwd_product_daily_metric FINAL"
# 期望: 235

docker exec -it clickhouse-server clickhouse-client --password 123456 \
  --query "SELECT * FROM ads_product_health_overview ORDER BY health_score DESC LIMIT 5 FORMAT Pretty"
# 期望: 5 行产品健康度数据

docker exec -it clickhouse-server clickhouse-client --password 123456 \
  --query "SELECT count(DISTINCT product_code) FROM dim_product FINAL"
# 期望: 产品数量（约 14 个）
```

- [ ] **Step 5: 验证幂等性（重跑不产生重复数据）**

```bash
make sync-all
docker exec -it clickhouse-server clickhouse-client --password 123456 \
  --query "SELECT count() FROM ods_product_daily_report FINAL"
# 应仍为 235，不翻倍
```

- [ ] **Step 6: 最终 Commit**

```bash
git add .
git commit -m "feat: complete GSheet→ClickHouse 5-layer warehouse pipeline"
```

---

## Self-Review

**Spec coverage 检查：**

| Spec 要求 | 覆盖 Task |
|---------|---------|
| 五层数仓 DDL | Task 1 |
| ODS 中文列名 | Task 1 Step 1 + Task 2 |
| DWD 英文列名+单位后缀 | Task 1 Step 3 + Task 5 Step 2 |
| ReplacingMergeTree(_version) | Task 1 所有 DDL |
| DIM SCD-2 扩展点 | Task 1 Step 2（effective_from/to 字段已留） |
| 多源 _source 扩展点 | Task 1 ODS DDL + Task 2 mapper |
| CSV 宽容解析 + 5% abort | Task 2 mapper + Task 4 ods_loader |
| PrepareBatch 批量写入 | Task 3 |
| ETL SQL 文件 + 参数化执行 | Task 5 + Task 1 Step 8 EtlRepo |
| DWS DROP PARTITION + INSERT 幂等 | Task 5 Steps 3-5 |
| DQ 三条规则 | Task 6 |
| DI 注入 | Task 7 Steps 1-2 |
| CLI sync-* 子命令 | Task 7 Steps 3-4 |
| Makefile targets | Task 7 Step 4 + Task 8 Step 2 |
| PG 语义层 seed | Task 8 |
| 端到端验证 + 幂等性验证 | Task 9 |

**Placeholder 扫描：** 无 TBD/TODO/类似 Task N 的引用，所有代码均为完整可执行版本。

**类型一致性：** `ETLRunner.RunDWD` 接收 `time.Time` from/to，`EtlRepo.ExecETL` 的 `map[string]interface{}` 中对应传 `time.Time`，与 CH driver 的 `{from_date:Date}` 参数类型匹配。`ODSLoader.Load` 调用 `ODSRepo.BatchInsert`，参数类型 `[]dto.ODSRow` 在 Task 2 Step 1 定义，Task 3 使用——一致。
