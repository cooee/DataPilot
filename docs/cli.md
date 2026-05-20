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
10. [Analytics 语义层查询](#10-analytics-语义层查询)
11. [每日运营日报（一键 SOP）](#11-每日运营日报一键-sop)
12. [AI Agent（指标问答 / 异常列举）](#12-ai-agent指标问答--异常列举)

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
| `010_site_product_pipeline.sql` | `ods/dim/dwd/dws_site_*` 四表 | 站点产品（gid=553168897） |
| `009_ads_anomaly_detection.sql` | `ads_product_anomaly_daily` | 30 日基线 ±3σ 异常检测 |

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
| **站点产品** | `553168897` | **`site`**（独立管线，见下文） |

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

> 等价于：**sync-ods（gid=0 付费）+ sync-ods（gid=469519483 免费）+ sync-ods:site（gid=553168897）+ 各自 DIM/DWD/DWS**  
> `--sync:all` **忽略 `-gid`**，固定同步三个 Sheet。

```bash
# 付费 + 免费 ODS，再跑全链路 ETL（推荐）
make sync-all ARGS="-from=2026-03-01 -to=2026-05-19"
./datapilot --sync:all -from=2026-03-01 -to=2026-05-20

# 仅同步某一个 Sheet 时用 sync:ods 并指定 -gid
make sync-ods ARGS="-gid=469519483 -from=2026-03-01 -to=2026-05-19"
```

### 站点产品（gid=553168897，平行管线）

> 11 列结构（日活跃数 / 日导量新增 / 日导量充值），已包含在 `--sync:all`；也可单独执行下方命令。  
> 详见 [docs/schema/site-product-sheet.md](schema/site-product-sheet.md)。

```bash
# 先执行迁移（首次）
make ch-migrate

# 仅 ODS + DQ
make sync-ods-site ARGS="-from=2026-05-01 -to=2026-05-18"
./datapilot --sync:ods:site -from=2026-05-01 -to=2026-05-18

# ODS → DIM → DWD → DWS
make sync-site-all ARGS="-from=2026-05-01 -to=2026-05-18"
./datapilot --sync:site:all -from=2026-05-01 -to=2026-05-18
```

验证站点 DWS：

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
SELECT date, count() AS rows, uniq(product_code) AS products, sum(dau) AS dau
FROM dws_site_product_daily
GROUP BY date ORDER BY date"
```

---

## 7. 数据验证

### 验证各层行数

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
SELECT layer, product_type, rows, products FROM (
    SELECT 'ODS' AS layer, product_type, count() AS rows, uniq(\`产品编号\`) AS products
    FROM ods_product_daily_report FINAL GROUP BY product_type
    UNION ALL
    SELECT 'DWD', product_type, count(), uniq(product_code)
    FROM dwd_product_daily_metric FINAL GROUP BY product_type
    UNION ALL
    SELECT 'DWS_product', product_type, count(), uniq(product_code)
    FROM dws_product_daily GROUP BY product_type
) ORDER BY layer, product_type
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

### 场景 F：某一天付费与免费产品数据对比

**方式一：按类型汇总对比（最常用）**

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
SELECT
    product_type,
    count()                                    AS product_cnt,
    sum(dau)                                   AS dau_total,
    round(sum(recharge_total_amt), 0)          AS recharge_total,
    round(avg(retention_d7_ratio), 4)          AS avg_retention_d7,
    round(sum(recharge_total_amt)/sum(dau), 2) AS arpu,
    round(avg(arppu), 2)                       AS avg_arppu,
    sum(new_user_total_cnt)                    AS new_user_total,
    sum(new_paying_user_cnt)                   AS new_paying_cnt
FROM dws_product_daily
WHERE date = '2026-05-18'
GROUP BY product_type
FORMAT PrettyCompact"
```

**方式二：每个产品明细并排**

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
SELECT
    product_type, product_code, product_name, team,
    dau,
    round(recharge_total_amt, 0)  AS recharge,
    round(retention_d7_ratio, 4)  AS retention_d7,
    round(arppu, 2)               AS arppu
FROM dws_product_daily
WHERE date = '2026-05-18'
ORDER BY product_type, recharge DESC
FORMAT PrettyCompact"
```

**方式三：指定任意一天（Shell 变量）**

```bash
DATE=2026-05-01

docker exec clickhouse-server clickhouse-client --password 123456 -q "
SELECT product_type, count() AS products, sum(dau) AS dau,
       round(sum(recharge_total_amt), 0) AS recharge,
       round(avg(retention_d7_ratio), 4) AS retention_d7
FROM dws_product_daily
WHERE date = '$DATE'
GROUP BY product_type
FORMAT PrettyCompact"
```

**方式四：多天趋势对比（paid vs free 走势）**

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
SELECT date, product_type,
       sum(dau)                          AS dau,
       round(sum(recharge_total_amt), 0) AS recharge,
       round(avg(retention_d7_ratio), 4) AS retention_d7
FROM dws_product_daily
WHERE date BETWEEN '2026-05-12' AND '2026-05-18'
GROUP BY date, product_type
ORDER BY date, product_type
FORMAT PrettyCompact"
```

---

## 10. Analytics 语义层查询

> 通过结构化参数查询 ClickHouse（非原始 SQL），输出表格或 JSON，便于终端使用和后续 AI 接入。  
> 若需指标中文名/单位等语义信息，请先执行 `make migrate-seed`（未执行时 schema/query 仍可用，仅无 PG 语义 enrichment）。  
> **大模型 / Agent 编排速查（含站点产品线）**：[docs/agent/ai-cli-reference.md](agent/ai-cli-reference.md)

### 查看可用数据集

```bash
make analytics-schema
# JSON：make analytics-schema ARGS="-format=json"
```

### 内置场景（preset）

| preset | 说明 |
|---|---|
| `product-type-compare` | 某天 paid vs free 汇总对比（场景 F-方式一） |
| `product-detail` | 产品明细按充值排序 |
| `product-trend` | 多天 paid/free 走势 |
| `team-performance` | 小组日表现（ADS 视图） |
| `product-health` | 产品健康度 Top N |
| `anomaly-detection` | 报告日异常（≥2 天历史用 ±3σ；仅 1 天历史用环比 ≥30%） |
| `anomaly-watch` | 报告日偏离度 Top 30（含无异常时的排查） |
| `anomaly-baseline` | 产品 30 日基线上下界 |
| `site-product-summary` | 站点产品日汇总（DAU / 导量新增 / 导量充值） |
| `site-product-detail` | 站点产品明细（按导量充值排序） |
| `site-product-trend` | 站点产品多日走势 |
| `site-team-summary` | 站点产品按小组汇总 |

```bash
# 付费 vs 免费对比（默认昨天）
make analytics-query ARGS="-preset=product-type-compare -from=2026-05-18 -to=2026-05-18"

# 产品明细
make analytics-query ARGS="-preset=product-detail -from=2026-05-18 -to=2026-05-18"

# 近 7 天走势
make analytics-query ARGS="-preset=product-trend -from=2026-05-12 -to=2026-05-18"

# JSON 输出（给 AI / 管道）
make analytics-query ARGS="-preset=product-health -format=json"

# 异常检测（需先 make ch-migrate 应用 009 视图）
make analytics-query ARGS="-preset=anomaly-detection -from=2026-05-18 -to=2026-05-18"
make analytics-query ARGS="-preset=anomaly-watch -from=2026-05-18 -to=2026-05-18"
make analytics-query ARGS="-preset=anomaly-baseline"

# 站点产品（需先 sync:all 或 sync:site:all 写入 dws_site_product_daily）
make analytics-query ARGS="-preset=site-product-summary -from=2026-05-18 -to=2026-05-18"
make analytics-query ARGS="-preset=site-product-detail -from=2026-05-18 -to=2026-05-18"
make analytics-query ARGS="-preset=site-product-trend -from=2026-05-01 -to=2026-05-18"
make analytics-query ARGS="-preset=site-team-summary -from=2026-05-18 -to=2026-05-18"
```

### 自定义查询

```bash
make analytics-query ARGS='-dataset=dws_product_daily \
  -metrics=dau,recharge_total_amt,retention_d7_ratio \
  -dimensions=product_type \
  -from=2026-05-18 -to=2026-05-18'

# 过滤：product_type 仅 paid
make analytics-query ARGS='-preset=product-detail -from=2026-05-18 -to=2026-05-18 \
  -filter=product_type:eq:paid'

# 完整 JSON 请求体
make analytics-query ARGS='-json-file=query.json -format=json'
```

**filter 格式：** `field:op:value`，多条用 `|` 分隔。支持 `eq` `in` `between` 等。

---

## 11. 每日运营日报（一键 SOP）

> 完整说明见 [docs/sop/daily-ops-report.md](sop/daily-ops-report.md)

```bash
# 编译
go build -o datapilot ./cmd

# 一键：付费+免费 ODS → ETL → DQ → Analytics 三段 preset
make daily-report
make daily-report ARGS="-date=2026-05-18"
make daily-report ARGS="-from=2026-05-12 -to=2026-05-18 -date=2026-05-18 -format=json"

# 仅刷新查询（跳过同步）
make daily-report ARGS="-skip-sync -date=2026-05-18"

# HTML 日报（异常 + 7 日线性预测，输出 reports/daily-YYYY-MM-DD.html）
make daily-report-html ARGS="-date=2026-05-18"
./datapilot --report:daily-html -skip-sync -date=2026-05-18 -out=reports/daily-2026-05-18.html
```

Cursor Skill：`.cursor/skills/datapilot-daily-ops-report/SKILL.md`。

---

## 12. AI Agent（指标问答 / 异常列举）

> - 架构与 Gemini：[docs/agent/README.md](agent/README.md)  
> - **LLM / Agent 数据获取速查（含站点产品）**：[docs/agent/ai-cli-reference.md](agent/ai-cli-reference.md)

```bash
# 指标问答（规则 NL→语义层）
make agent-ask ARGS='-q="2026-05-18 付费和免费日活充值对比" -date=2026-05-18'

# 站点产品（识别「站点/导量」→ site-* preset → dws_site_product_daily）
make agent-ask ARGS='-q="2026-05-18 站点产品导量充值汇总" -date=2026-05-18'
./datapilot --agent:ask -preset=site-product-detail -date=2026-05-18

# 站点：语义层直查（不经过 NL）
make analytics-query ARGS="-preset=site-product-summary -from=2026-05-18 -to=2026-05-18"

# 异常产品（仅 paid/free，不含站点）
make agent-anomalies ARGS="-date=2026-05-18"

# HTTP（需 JWT）
# POST /api/agent/metrics/ask
# POST /api/agent/anomalies
```

**给大模型的要点**：站点与 paid/free **不同表**；站点充值用 `lead_recharge_amt`，勿用 `recharge_total_amt`。完整 preset 表见 [ai-cli-reference.md](agent/ai-cli-reference.md)。

---

> 文档对应代码版本：`main` 分支，2026-05-19  
> ClickHouse 地址：`localhost:9000`（native）/ `localhost:8123`（HTTP）  
> PostgreSQL 地址：`localhost:5432`

http://localhost:8123/play?user=default&password=123456