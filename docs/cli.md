# DataPilot CLI 操作手册

> 项目根目录下所有命令均通过 `make <target>` 或 `go run ./cmd <flag>` 执行。  
> `ARGS` 变量通过 `make sync-ods ARGS="..."` 的形式传入额外参数。

---

## 目录

1. [环境初始化](#1-环境初始化)
2. [ClickHouse 建表 / 迁移](#2-clickhouse-建表--迁移)
3. [PostgreSQL 建表 / Seed](#3-postgresql-建表--seed)
4. [数据录入（GSheet → ODS）](#4-数据录入gsheet--ods)
5. [ETL 流水线（ODS → DWD → DWS）](#5-etl-流水线ods--dwd--dws)
6. [一键全量同步](#6-一键全量同步)
7. [数据验证](#7-数据验证)
8. [数据清除 / 重置](#8-数据清除--重置)
9. [常用场景速查](#9-常用场景速查)

---

## 1. 环境初始化

### 启动依赖容器

```bash
# PostgreSQL
docker run -d --name postgres-server \
  -p 5432:5432 \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=123456 \
  -e POSTGRES_DB=postgres \
  postgres:latest

# ClickHouse
docker run -d --name clickhouse-server \
  -p 8123:8123 -p 9000:9000 \
  -e CLICKHOUSE_USER=default \
  -e CLICKHOUSE_PASSWORD=123456 \
  -e CLICKHOUSE_DB=default \
  clickhouse/clickhouse-server:latest
```

### 检查容器状态

```bash
docker ps --filter name=postgres-server --filter name=clickhouse-server \
  --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
```

---

## 2. ClickHouse 建表 / 迁移

> 对应文件：`database/clickhouse/migrations/*.sql`  
> 幂等安全（所有表使用 `CREATE TABLE IF NOT EXISTS`，`ADD COLUMN IF NOT EXISTS`）。

```bash
make ch-migrate
# 等价于：go run ./cmd --ch:migrate
```

**包含的表/视图：**

| 文件 | 对象 | 说明 |
|---|---|---|
| `001_ods_product_daily_report.sql` | `ods_product_daily_report` | ODS 原始数据层 |
| `002_dim_product.sql` | `dim_product` | 产品维表 |
| `003_dwd_product_daily_metric.sql` | `dwd_product_daily_metric` | DWD 指标明细层 |
| `004_dws_product_daily.sql` | `dws_product_daily` | DWS 产品日汇总 |
| `005_dws_team_daily.sql` | `dws_team_daily` | DWS 小组日汇总 |
| `006_dws_bu_daily.sql` | `dws_bu_daily` | DWS 部门日汇总 |
| `007_ads_views.sql` | `ads_*` 三个视图 | ADS 应用层视图 |
| `008_add_product_type.sql` | 各表新增 `product_type` 列 | paid/free 产品区分 |

---

## 3. PostgreSQL 建表 / Seed

```bash
# 执行 PG 迁移（建表）
make migrate

# 写入基础数据（含 meta_metric_dict 指标语义层，35 条记录）
make seed

# 一步完成建表 + 写入
make migrate-seed
```

**meta_metric_dict 包含的指标分类：**  
`dau` / `recharge` / `new_user` / `paying` / `retention` / `landing` / `conversion` / `arppu`

---

## 4. 数据录入（GSheet → ODS）

### 参数说明

| 参数 | 说明 | 默认值 |
|---|---|---|
| `-id` | Spreadsheet ID 或完整 URL | 代码内置默认 ID |
| `-gid` | Sheet 标签 gid（见 URL `#gid=xxx`） | `0` |
| `-from` | 起始日期 `YYYY-MM-DD` | 90 天前 |
| `-to` | 结束日期 `YYYY-MM-DD` | 今天 |
| `-type` | `paid` 或 `free`（可省略，自动从 gid 推断） | 自动推断 |

### 已知 Sheet 对应关系

| Sheet | gid | product_type |
|---|---|---|
| 付费产品 | `0` | `paid`（自动） |
| 免费产品 | `469519483` | `free`（自动） |

### 同步付费产品

```bash
make sync-ods ARGS="-gid=0 -from=2026-03-01 -to=2026-05-19"
```

### 同步免费产品

```bash
make sync-ods ARGS="-gid=469519483 -from=2026-03-01 -to=2026-05-19"
```

### 指定完整 URL（自动解析 ID 和 gid）

```bash
make sync-ods ARGS='-id="https://docs.google.com/spreadsheets/d/1sJZB.../edit?gid=0#gid=0" -from=2026-05-01 -to=2026-05-19'
```

> **写入后自动执行 DQ 检查（数据质量）：**  
> - 重复行检查（同日期+产品编号+来源）  
> - 负值检查（日活、总充值）  
> - 3σ 离群值检查（日活环比，仅警告不中断）

---

## 5. ETL 流水线（ODS → DWD → DWS）

> ODS 写入后，按层执行 ETL。每步均需传入日期范围。

```bash
# Step 1：ODS → DIM（产品维表刷新，幂等）
make sync-dim ARGS="-from=2026-03-01 -to=2026-05-19"

# Step 2：ODS → DWD（中英字段映射，幂等）
make sync-dwd ARGS="-from=2026-03-01 -to=2026-05-19"

# Step 3：DWD → DWS（product/team/bu 三张汇总，DROP PARTITION 后重写）
make sync-dws ARGS="-from=2026-03-01 -to=2026-05-19"
```

---

## 6. 一键全量同步

> 等价于：sync-ods（付费）+ sync-ods（免费）+ DIM + DWD + DWS

```bash
# 付费产品全量
make sync-all ARGS="-gid=0 -from=2026-03-01 -to=2026-05-19"

# 免费产品 ODS 追加后再跑 ETL
make sync-ods ARGS="-gid=469519483 -from=2026-03-01 -to=2026-05-19"
make sync-dim ARGS="-from=2026-03-01 -to=2026-05-19"
make sync-dwd ARGS="-from=2026-03-01 -to=2026-05-19"
make sync-dws ARGS="-from=2026-03-01 -to=2026-05-19"
```

---

## 7. 数据验证

### 验证各层行数

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
SELECT 'ODS' AS layer, product_type, count() AS rows, uniq(\`产品编号\`) AS products
FROM ods_product_daily_report FINAL GROUP BY product_type
UNION ALL
SELECT 'DWD', product_type, count(), uniq(product_code)
FROM dwd_product_daily_metric FINAL GROUP BY product_type
UNION ALL
SELECT 'DWS_product', product_type, count(), uniq(product_code)
FROM dws_product_daily GROUP BY product_type
ORDER BY layer, product_type
FORMAT PrettyCompact"
```

### 查看产品健康度（ADS 视图）

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
SELECT product_code, product_name, team, round(avg_dau_30d,0) AS avg_dau,
       round(recharge_total_30d,0) AS recharge_30d, round(health_score,3) AS health
FROM ads_product_health_overview
ORDER BY health DESC LIMIT 10
FORMAT PrettyCompact"
```

### 查看小组日表现（ADS 视图）

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
SELECT date, team, dau_sum, recharge_total_amt_sum,
       round(retention_d7_ratio_wavg, 4) AS retention_d7
FROM ads_team_performance_daily
ORDER BY date DESC, team LIMIT 20
FORMAT PrettyCompact"
```

### 按产品类型对比充值

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
SELECT product_type,
       sum(recharge_total_amt) AS recharge_total,
       sum(dau) AS dau_total,
       round(sum(recharge_total_amt)/sum(dau), 2) AS arpu
FROM dws_product_daily
WHERE date >= today() - 30
GROUP BY product_type
FORMAT PrettyCompact"
```

### 验证 PG 指标语义表

```bash
docker exec postgres-server psql -U postgres -d postgres -c \
  "SELECT metric_key, metric_name_zh, category, unit, direction FROM meta_metric_dict ORDER BY category, metric_key LIMIT 20;"
```

### 快速检查 GSheet 连通性（公开链接，零凭证）

```bash
make test-gsheet-public url="https://docs.google.com/spreadsheets/d/1sJZBMAHBa3QtmSKQ8oLGaiC_w-8kloGJizRNHnbQ-jw/edit?gid=0#gid=0"
make test-gsheet-public url="https://docs.google.com/spreadsheets/d/1sJZBMAHBa3QtmSKQ8oLGaiC_w-8kloGJizRNHnbQ-jw/edit?gid=469519483#gid=469519483"
```

---

## 8. 数据清除 / 重置

### 清空 ClickHouse 所有数据层

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
TRUNCATE TABLE ods_product_daily_report;
TRUNCATE TABLE dim_product;
TRUNCATE TABLE dwd_product_daily_metric;
TRUNCATE TABLE dws_product_daily;
TRUNCATE TABLE dws_team_daily;
TRUNCATE TABLE dws_bu_daily;"
```

### 清空单层（按日期分区删除，保留其他月份）

```bash
# 删除指定月份分区（格式 YYYYMM）
docker exec clickhouse-server clickhouse-client --password 123456 -q "
ALTER TABLE dws_product_daily DROP PARTITION '202605';
ALTER TABLE dws_team_daily    DROP PARTITION '202605';
ALTER TABLE dws_bu_daily      DROP PARTITION '202605';"
```

### 按产品类型删除（轻量删除，异步）

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
DELETE FROM ods_product_daily_report WHERE product_type = 'free';"
```

### 删除并重建所有 ClickHouse 表（完全重置）

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
DROP TABLE IF EXISTS ods_product_daily_report;
DROP TABLE IF EXISTS dim_product;
DROP TABLE IF EXISTS dwd_product_daily_metric;
DROP TABLE IF EXISTS dws_product_daily;
DROP TABLE IF EXISTS dws_team_daily;
DROP TABLE IF EXISTS dws_bu_daily;
DROP VIEW IF EXISTS ads_product_health_overview;
DROP VIEW IF EXISTS ads_team_performance_daily;
DROP VIEW IF EXISTS ads_anomaly_baseline;"

make ch-migrate
```

---

## 9. 常用场景速查

### 场景 A：首次部署

```bash
# 1. 启动容器（见第 1 节）
# 2. 建 ClickHouse 表
make ch-migrate
# 3. 建 PG 表 + 写入指标语义
make migrate-seed
# 4. 同步付费产品全量
make sync-all ARGS="-gid=0 -from=2026-03-01 -to=2026-05-19"
# 5. 追加免费产品 ODS，再跑 ETL
make sync-ods ARGS="-gid=469519483 -from=2026-03-01 -to=2026-05-19"
make sync-dim ARGS="-from=2026-03-01 -to=2026-05-19"
make sync-dwd ARGS="-from=2026-03-01 -to=2026-05-19"
make sync-dws ARGS="-from=2026-03-01 -to=2026-05-19"
```

### 场景 B：每日增量同步（昨天数据）

```bash
TODAY=$(date +%Y-%m-%d)
YESTERDAY=$(date -v-1d +%Y-%m-%d 2>/dev/null || date -d yesterday +%Y-%m-%d)

make sync-ods ARGS="-gid=0          -from=$YESTERDAY -to=$TODAY"
make sync-ods ARGS="-gid=469519483  -from=$YESTERDAY -to=$TODAY"
make sync-dim ARGS="-from=$YESTERDAY -to=$TODAY"
make sync-dwd ARGS="-from=$YESTERDAY -to=$TODAY"
make sync-dws ARGS="-from=$YESTERDAY -to=$TODAY"
```

### 场景 C：补录历史数据（指定月份）

```bash
make sync-all ARGS="-gid=0         -from=2026-03-01 -to=2026-03-31"
make sync-ods ARGS="-gid=469519483 -from=2026-03-01 -to=2026-03-31"
make sync-dim ARGS="-from=2026-03-01 -to=2026-03-31"
make sync-dwd ARGS="-from=2026-03-01 -to=2026-03-31"
make sync-dws ARGS="-from=2026-03-01 -to=2026-03-31"
```

### 场景 D：清除某月数据后重跑

```bash
# 清除 2026-05 分区
docker exec clickhouse-server clickhouse-client --password 123456 -q "
ALTER TABLE ods_product_daily_report DROP PARTITION '202605';
ALTER TABLE dwd_product_daily_metric DROP PARTITION '202605';
ALTER TABLE dws_product_daily        DROP PARTITION '202605';
ALTER TABLE dws_team_daily           DROP PARTITION '202605';
ALTER TABLE dws_bu_daily             DROP PARTITION '202605';"

# 重新录入
make sync-ods ARGS="-gid=0         -from=2026-05-01 -to=2026-05-31"
make sync-ods ARGS="-gid=469519483 -from=2026-05-01 -to=2026-05-31"
make sync-dim ARGS="-from=2026-05-01 -to=2026-05-31"
make sync-dwd ARGS="-from=2026-05-01 -to=2026-05-31"
make sync-dws ARGS="-from=2026-05-01 -to=2026-05-31"
```

### 场景 E：新增 Sheet（新产品类型）

1. 在 `cmd/sync_gsheet.go` 的 `knownFreeGIDs` map 中追加新 gid：

```go
var knownFreeGIDs = map[int]bool{
    469519483: true,
    987654321: true, // 新增的 sheet gid
}
```

2. 或通过 `-type` 手动指定：

```bash
make sync-ods ARGS="-gid=987654321 -type=free -from=2026-05-01 -to=2026-05-19"
```

---

> 文档对应代码版本：`main` 分支，2026-05-19  
> ClickHouse 地址：`localhost:9000`（native）/ `localhost:8123`（HTTP）  
> PostgreSQL 地址：`localhost:5432`
