# DataPilot 数据中台设计文档
**Google Sheet → ClickHouse 五层数仓**

- **版本**：v1.0
- **日期**：2026-05-19
- **状态**：待实施
- **作者**：moyucheng（AI 辅助设计）

---

## 1. 背景

DataPilot 项目目标是将运营团队通过 Google Sheet 填报的产品运营日报数据，经过结构化采集、分层建模后存入 ClickHouse，最终支撑 BI 报表查询和未来 AI Agent 的趋势预测与异常检测。

**现有技术栈**（本期复用，不引入新中间件）：

| 组件 | 版本/说明 |
|------|---------|
| Go | 1.26（gin + samber/do DI） |
| PostgreSQL | via GORM，已有 migration 体系 |
| ClickHouse | 原生 `driver.Conn`（clickhouse-go/v2），已注入 DI |
| Google Sheets | `google.golang.org/api`（Service Account + 公开 CSV 两种方式） |
| 部署 | 本地 Docker / 未来 k8s CronJob |

**当前状态**：
- `make test-gsheet-public` 已能拉取目标 Sheet（236 行 × 40 列运营日报）
- PG + CH + Google API 均已注入 DI 容器

---

## 2. 范围

### 本期（v1.0）包含

- 五层数仓完整建模：ODS / DIM / DWD / DWS / ADS
- 基础指标语义层（`meta_metric_dict`，放 PG）
- 多源扩展点（`_source` 列，为未来多 Sheet 预留）
- CLI 触发的同步命令（`make sync-all` 等）
- ODS → DWD → DWS 的 ETL SQL 文件体系
- ADS 应用层视图（3 个起步视图）
- 数据质量校验（基础规则，结果记日志）

### 本期**不**包含

- AI Agent / LLM 集成（下一期）
- Admin UI / 数据血缘平台（YAGNI）
- 业务 PG 库接入（只接 Sheet）
- DIM SCD-2 历史快照完整流程（仅最新版本覆盖）
- 指标计算口径版本管理
- 性能压测 / Chaos Engineering

---

## 3. 关键决策

| 议题 | 决策 | 理由 |
|------|------|------|
| ETL 实现方式 | Go 管采集（CSV→ODS），CH SQL 管 ETL（ODS→DWD→DWS） | "工程归 Go，数仓归 SQL"；SQL 改字段不重新部署；CH 内部 INSERT...SELECT 性能最优 |
| ODS 字段命名 | 保留中文列名（加反引号） | 贴近源，出问题可对照原始 Sheet 反查 |
| DWD/DIM/DWS/ADS 字段命名 | 英文 snake_case + 单位后缀（`_cnt`/`_amt`/`_ratio`/`_chain_ratio`/`_multi`） | 统一语义，BI/Agent text-to-SQL 友好 |
| 数值单位 | 全栈保留小数原值（如 0.7173），靠字段后缀标注 | 不丢精度；跨业务汇总时不会出现单位不一致 |
| 更新语义 | `ReplacingMergeTree(_version)`，`_version = toUnixTimestamp64Milli(now64())` | 统一支持：增量追加 / 补述历史（重插新 version）/ 全量重刷（全表重插）三种模式 |
| 自然主键 | DWD: `(date, product_code)`；DIM: `(product_code)`；DWS: `(date, <group_key>)` | 避免使用代理键，业务含义明确 |
| 触发方式 | 仅 CLI（`make sync-*`），调度交给外部 cron/k8s | 本期 YAGNI；CLI 既是调试工具也是调度入口 |
| DWS 重算幂等性 | `ALTER TABLE DROP PARTITION + INSERT` | 杜绝读到半成品；比 TRUNCATE 粒度更细 |
| 多源扩展 | ODS 表统一带 `_source` 列进 `ORDER BY` | 未来新接 Sheet 只需新建 `ods_xxx` 或同表写不同 `_source`，DIM/DWD 通过 `_source` 做 ID 映射 |
| 元数据位置 | `meta_metric_dict` 放 PG | 后期可接 admin 编辑，PG 事务/审计更完善 |

---

## 4. 顶层架构

```
┌─────────────────────────────┐
│  Google Sheet (公开 CSV)     │
│  ods_product_daily_report   │  ← 未来可接更多 Sheet
└─────────────┬───────────────┘
              │ ① sync-ods: Go 拉 CSV，PrepareBatch 写入
              ▼
┌─────────────────────────────┐
│           ODS 层             │
│  ods_product_daily_report   │  ReplacingMergeTree，中文列名，1:1 原始
└──────┬────────────┬─────────┘
       │ ② sync-dim  │ ③ sync-dwd
       ▼             ▼
┌──────────┐  ┌────────────────────────┐
│  DIM 层  │  │        DWD 层           │
│dim_product│  │dwd_product_daily_metric│  ReplacingMergeTree，英文列名
└──────────┘  └──────────┬─────────────┘
      │ join              │
      └─────────┬─────────┘
                │ ④ sync-dws
                ▼
┌────────────────────────────────────────┐
│                 DWS 层                  │
│  dws_product_daily / dws_team_daily    │
│  dws_bu_daily                          │  MergeTree，按月分区，JOIN DIM 后的宽表
└────────────────────┬───────────────────┘
                     │ ⑤ sync-ads (CREATE/REPLACE VIEW)
                     ▼
┌────────────────────────────────────────┐
│                 ADS 层                  │
│  ads_product_health_overview           │
│  ads_team_performance_daily            │  VIEW，不物化，灵活演进
│  ads_anomaly_baseline                  │
└────────────────────┬───────────────────┘
                     │
                     ▼
              BI / 未来 AI Agent

PG: meta_metric_dict（指标语义层，中英对照、单位、口径）
```

---

## 5. 各层完整 DDL

### 5.1 ODS — `ods_product_daily_report`

```sql
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

    -- 元数据列（调用方无需填写）
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

**读取最新版本**：

```sql
-- 方式 A：FINAL（语义清晰，中小规模首选）
SELECT * FROM ods_product_daily_report FINAL WHERE `日期` = '2026-05-01';

-- 方式 B：argMax（大规模分析时更快）
SELECT `日期`, `产品编号`, argMax(`日活`, `_version`) AS `日活`
FROM ods_product_daily_report
GROUP BY `日期`, `产品编号`;
```

---

### 5.2 DIM — `dim_product`

```sql
CREATE TABLE IF NOT EXISTS dim_product (
    product_code      LowCardinality(String),
    product_name      String,
    team              LowCardinality(String),
    business_unit     LowCardinality(String),
    product_category  Nullable(LowCardinality(String)),  -- 付费/免费/站点/备份，初期可 NULL
    is_active         UInt8    DEFAULT 1,
    effective_from    Date     DEFAULT today(),
    effective_to      Nullable(Date),                    -- NULL = 当前生效，SCD-2 扩展点
    `_source`         LowCardinality(String) DEFAULT 'gsheet:测试-写入',
    `_ingested_at`    DateTime64(3) DEFAULT now64(),
    `_version`        UInt64 MATERIALIZED toUnixTimestamp64Milli(`_ingested_at`)
)
ENGINE = ReplacingMergeTree(`_version`)
ORDER BY (product_code, effective_from);
```

**从 ODS 更新 DIM 的 ETL**（`etl/ods_to_dim.sql`）：

```sql
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

---

### 5.3 DWD — `dwd_product_daily_metric`

**字段单位后缀约定**：

| 后缀 | 类型 | 含义 |
|------|------|------|
| `_cnt` | `UInt64` | 计数（用户数、订单数） |
| `_amt` | `Decimal(18,2)` | 金额（元） |
| `_ratio` | `Float64` | 比率，值域 [0, 1]（如留存率、付款成功率） |
| `_chain_ratio` | `Float64` | 环比，值 = (当期-上期)/上期，可负可大于 1 |
| `_multi` | `Float64` | 倍数型指标（如总转化 4.66 表示每获客带来的倍数收益） |
| 无后缀 | `Decimal(18,4)` | 复合指标如 `arppu` |

```sql
CREATE TABLE IF NOT EXISTS dwd_product_daily_metric (
    -- 维度键
    date                              Date,
    product_code                      LowCardinality(String),

    -- 活跃
    dau                               UInt64,
    dau_chain_ratio                   Float64,
    old_user_dau                      UInt64,

    -- 充值金额
    recharge_total_amt                Decimal(18, 2),
    recharge_total_chain_ratio        Float64,
    recharge_organic_amt              Decimal(18, 2),
    recharge_organic_chain_ratio      Float64,
    recharge_channel_amt              Decimal(18, 2),
    recharge_channel_chain_ratio      Float64,
    recharge_internal_amt             Decimal(18, 2),
    recharge_internal_chain_ratio     Float64,

    -- 新增用户
    new_user_total_cnt                UInt64,
    new_user_total_chain_ratio        Float64,
    new_user_organic_cnt              UInt64,
    new_user_organic_chain_ratio      Float64,
    new_user_channel_cnt              UInt64,
    new_user_channel_chain_ratio      Float64,
    new_user_internal_cnt             UInt64,
    new_user_internal_chain_ratio     Float64,

    -- 付费用户
    new_paying_user_cnt               UInt64,
    old_paying_user_cnt               UInt64,
    new_paying_user_amt               Decimal(18, 2),
    old_paying_user_amt               Decimal(18, 2),
    recharge_order_cnt                UInt64,
    payment_success_ratio             Float64,

    -- 留存
    retention_d1_ratio                Float64,
    retention_d3_ratio                Float64,
    retention_d7_ratio                Float64,

    -- 落地页
    landing_page_visit_cnt            UInt64,
    landing_page_click_cnt            UInt64,
    landing_page_download_ratio       Float64,

    -- 转化（倍数型）
    conversion_total_multi            Float64,
    conversion_new_paying_multi       Float64,
    conversion_old_paying_multi       Float64,

    -- ARPPU
    arppu                             Decimal(18, 4),

    -- 元数据
    `_src_row_no`                     UInt32,
    `_ingested_at`                    DateTime64(3) DEFAULT now64(),
    `_version`                        UInt64 MATERIALIZED toUnixTimestamp64Milli(`_ingested_at`)
)
ENGINE = ReplacingMergeTree(`_version`)
PARTITION BY toYYYYMM(date)
ORDER BY (date, product_code);
```

**ODS → DWD ETL**（`etl/ods_to_dwd.sql`）：

```sql
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

---

### 5.4 DWS — 主题聚合层

#### `dws_product_daily`（产品日粒度宽表）

DWD 需要 `FINAL` 才能去重，DWS 是"清洁宽表"，BI/Agent 直查，无需关心 FINAL。

```sql
CREATE TABLE IF NOT EXISTS dws_product_daily (
    date                      Date,
    product_code              LowCardinality(String),
    product_name              String,
    team                      LowCardinality(String),
    business_unit             LowCardinality(String),

    -- 复制 DWD 全部 metric（与 DWD 字段完全一致，此处省略重复列声明）
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

    `_built_at`               DateTime64(3) DEFAULT now64()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(date)
ORDER BY (date, product_code);
```

**DWD → DWS 重算 ETL**（`etl/dwd_to_dws_product.sql`，幂等按分区重建）：

```sql
-- Step 1: 删除目标分区（参数化）
ALTER TABLE dws_product_daily DROP PARTITION {partition:String};

-- Step 2: 重新计算写入
INSERT INTO dws_product_daily
SELECT
    d.date,
    d.product_code,
    p.product_name,
    p.team,
    p.business_unit,
    d.dau,
    d.dau_chain_ratio,
    d.old_user_dau,
    d.recharge_total_amt,
    d.recharge_total_chain_ratio,
    d.recharge_organic_amt,
    d.recharge_organic_chain_ratio,
    d.recharge_channel_amt,
    d.recharge_channel_chain_ratio,
    d.recharge_internal_amt,
    d.recharge_internal_chain_ratio,
    d.new_user_total_cnt,
    d.new_user_total_chain_ratio,
    d.new_user_organic_cnt,
    d.new_user_organic_chain_ratio,
    d.new_user_channel_cnt,
    d.new_user_channel_chain_ratio,
    d.new_user_internal_cnt,
    d.new_user_internal_chain_ratio,
    d.new_paying_user_cnt,
    d.old_paying_user_cnt,
    d.new_paying_user_amt,
    d.old_paying_user_amt,
    d.recharge_order_cnt,
    d.payment_success_ratio,
    d.retention_d1_ratio,
    d.retention_d3_ratio,
    d.retention_d7_ratio,
    d.landing_page_visit_cnt,
    d.landing_page_click_cnt,
    d.landing_page_download_ratio,
    d.conversion_total_multi,
    d.conversion_new_paying_multi,
    d.conversion_old_paying_multi,
    d.arppu,
    now64()
FROM dwd_product_daily_metric d FINAL
LEFT JOIN dim_product p FINAL USING (product_code)
WHERE d.date >= {from_date:Date}
  AND d.date <= {to_date:Date};
```

#### `dws_team_daily`（小组日粒度）

```sql
CREATE TABLE IF NOT EXISTS dws_team_daily (
    date                          Date,
    team                          LowCardinality(String),
    business_unit                 LowCardinality(String),
    active_product_cnt            UInt32,          -- 当日有数据的产品数
    dau_sum                       UInt64,
    recharge_total_amt_sum        Decimal(18, 2),
    recharge_organic_amt_sum      Decimal(18, 2),
    recharge_channel_amt_sum      Decimal(18, 2),
    recharge_internal_amt_sum     Decimal(18, 2),
    new_user_total_cnt_sum        UInt64,
    new_user_organic_cnt_sum      UInt64,
    new_user_channel_cnt_sum      UInt64,
    new_paying_user_cnt_sum       UInt64,
    old_paying_user_cnt_sum       UInt64,
    recharge_order_cnt_sum        UInt64,
    old_user_dau_sum              UInt64,
    -- 比率类不求和，存分子分母供查询时计算
    retention_d7_ratio_num        Float64,        -- sum(dau * retention_d7_ratio)
    retention_d7_ratio_den        Float64,        -- sum(dau)（加权均值分母）
    `_built_at`                   DateTime64(3) DEFAULT now64()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(date)
ORDER BY (date, team);
```

**ETL**（`etl/dwd_to_dws_team.sql`）：

```sql
ALTER TABLE dws_team_daily DROP PARTITION {partition:String};

INSERT INTO dws_team_daily
SELECT
    d.date,
    p.team,
    p.business_unit,
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

#### `dws_bu_daily`（部门日粒度，与 team 结构一致，GROUP BY business_unit）

结构与 `dws_team_daily` 完全相同，`ORDER BY (date, business_unit)`，ETL 类推（`etl/dwd_to_dws_bu.sql`）。

---

### 5.5 ADS — 应用视图

```sql
-- 视图 1：产品健康度总览（最近 30 天）
CREATE OR REPLACE VIEW ads_product_health_overview AS
SELECT
    product_code,
    product_name,
    team,
    business_unit,
    avg(dau)                  AS avg_dau_30d,
    sum(recharge_total_amt)   AS recharge_total_30d,
    avg(retention_d7_ratio)   AS avg_retention_d7_30d,
    avg(payment_success_ratio) AS avg_payment_success_30d,
    avg(arppu)                AS avg_arppu_30d,
    -- 简单健康分 [0,1]，越高越健康
    least(1, greatest(0,
          0.3 * (avg(retention_d7_ratio) / 0.05)
        + 0.3 * (avg(payment_success_ratio) / 0.7)
        + 0.4 * least(1, toFloat64(sum(recharge_total_amt)) / 1000000)
    )) AS health_score
FROM dws_product_daily
WHERE date >= today() - 30
GROUP BY product_code, product_name, team, business_unit;

-- 视图 2：小组日绩效（最近 90 天）
CREATE OR REPLACE VIEW ads_team_performance_daily AS
SELECT
    date,
    team,
    business_unit,
    active_product_cnt,
    dau_sum,
    recharge_total_amt_sum,
    new_paying_user_cnt_sum,
    recharge_order_cnt_sum,
    -- 加权7留率
    if(retention_d7_ratio_den > 0,
       retention_d7_ratio_num / retention_d7_ratio_den, 0) AS retention_d7_ratio_wavg
FROM dws_team_daily
WHERE date >= today() - 90;

-- 视图 3：异常检测基线（30天均值 ± 3σ，供 Agent 对比）
CREATE OR REPLACE VIEW ads_anomaly_baseline AS
SELECT
    product_code,
    'dau'               AS metric,
    avg(dau)            AS mean,
    stddevPop(dau)      AS std,
    avg(dau) + 3 * stddevPop(dau) AS upper_3sigma,
    avg(dau) - 3 * stddevPop(dau) AS lower_3sigma
FROM dws_product_daily WHERE date >= today() - 30
GROUP BY product_code
UNION ALL
SELECT product_code, 'recharge_total_amt', avg(recharge_total_amt),
    stddevPop(recharge_total_amt),
    avg(recharge_total_amt) + 3 * stddevPop(recharge_total_amt),
    avg(recharge_total_amt) - 3 * stddevPop(recharge_total_amt)
FROM dws_product_daily WHERE date >= today() - 30
GROUP BY product_code
UNION ALL
SELECT product_code, 'retention_d7_ratio', avg(retention_d7_ratio),
    stddevPop(retention_d7_ratio),
    avg(retention_d7_ratio) + 3 * stddevPop(retention_d7_ratio),
    avg(retention_d7_ratio) - 3 * stddevPop(retention_d7_ratio)
FROM dws_product_daily WHERE date >= today() - 30
GROUP BY product_code;
```

---

### 5.6 元数据 — PG `meta_metric_dict`

```sql
CREATE TABLE meta_metric_dict (
    id              BIGSERIAL PRIMARY KEY,
    metric_code     VARCHAR(128)  UNIQUE NOT NULL,  -- 'dau'
    metric_name_zh  VARCHAR(128)  NOT NULL,          -- '日活'
    metric_name_en  VARCHAR(128)  NOT NULL,          -- 'Daily Active Users'
    unit            VARCHAR(16)   NOT NULL,          -- 'cnt'|'amount'|'ratio'|'multi'
    formula         TEXT,                             -- 口径文字描述
    layer           VARCHAR(8)    NOT NULL,          -- 'DWD'|'DWS'|'ADS'
    owner_team      VARCHAR(64),
    description     TEXT,
    is_active       BOOLEAN DEFAULT TRUE,
    created_at      TIMESTAMP DEFAULT now(),
    updated_at      TIMESTAMP DEFAULT now()
);

-- 本期 seed（40 个指标）
INSERT INTO meta_metric_dict (metric_code, metric_name_zh, metric_name_en, unit, layer) VALUES
  ('dau',                      '日活',         'Daily Active Users',           'cnt',    'DWD'),
  ('dau_chain_ratio',          '日活环比',     'DAU Chain Ratio',              'ratio',  'DWD'),
  ('old_user_dau',             '老用户日活',   'Old User DAU',                 'cnt',    'DWD'),
  ('recharge_total_amt',       '总充值',       'Total Recharge Amount',        'amount', 'DWD'),
  ('recharge_total_chain_ratio','总充值环比',  'Total Recharge Chain Ratio',   'ratio',  'DWD'),
  ('recharge_organic_amt',     '自然充值',     'Organic Recharge Amount',      'amount', 'DWD'),
  ('recharge_organic_chain_ratio','自然充值环比','Organic Recharge Chain Ratio','ratio', 'DWD'),
  ('recharge_channel_amt',     '渠道充值',     'Channel Recharge Amount',      'amount', 'DWD'),
  ('recharge_channel_chain_ratio','渠道充值环比','Channel Recharge Chain Ratio','ratio', 'DWD'),
  ('recharge_internal_amt',    '内导充值',     'Internal Recharge Amount',     'amount', 'DWD'),
  ('recharge_internal_chain_ratio','内导充值环比','Internal Recharge Chain Ratio','ratio','DWD'),
  ('new_user_total_cnt',       '总新增',       'Total New Users',              'cnt',    'DWD'),
  ('new_user_total_chain_ratio','总新增环比',  'Total New Users Chain Ratio',  'ratio',  'DWD'),
  ('new_user_organic_cnt',     '自然新增',     'Organic New Users',            'cnt',    'DWD'),
  ('new_user_organic_chain_ratio','自然新增环比','Organic New Users Chain Ratio','ratio','DWD'),
  ('new_user_channel_cnt',     '渠道新增',     'Channel New Users',            'cnt',    'DWD'),
  ('new_user_channel_chain_ratio','渠道新增环比','Channel New Users Chain Ratio','ratio','DWD'),
  ('new_user_internal_cnt',    '内导新增',     'Internal New Users',           'cnt',    'DWD'),
  ('new_user_internal_chain_ratio','内导新增环比','Internal New Users Chain Ratio','ratio','DWD'),
  ('new_paying_user_cnt',      '新充人数',     'New Paying Users',             'cnt',    'DWD'),
  ('old_paying_user_cnt',      '老充人数',     'Old Paying Users',             'cnt',    'DWD'),
  ('new_paying_user_amt',      '新用户充值',   'New User Recharge Amount',     'amount', 'DWD'),
  ('old_paying_user_amt',      '老用户充值',   'Old User Recharge Amount',     'amount', 'DWD'),
  ('recharge_order_cnt',       '充值单数',     'Recharge Order Count',         'cnt',    'DWD'),
  ('payment_success_ratio',    '付款成功率',   'Payment Success Ratio',        'ratio',  'DWD'),
  ('retention_d1_ratio',       '次留率',       'D1 Retention Ratio',           'ratio',  'DWD'),
  ('retention_d3_ratio',       '3留率',        'D3 Retention Ratio',           'ratio',  'DWD'),
  ('retention_d7_ratio',       '7留率',        'D7 Retention Ratio',           'ratio',  'DWD'),
  ('landing_page_visit_cnt',   '下载页访问数', 'Landing Page Visit Count',     'cnt',    'DWD'),
  ('landing_page_click_cnt',   '下载页点击数', 'Landing Page Click Count',     'cnt',    'DWD'),
  ('landing_page_download_ratio','落地页下载率','Landing Page Download Ratio',  'ratio',  'DWD'),
  ('conversion_total_multi',   '总转化',       'Total Conversion Multiplier',  'multi',  'DWD'),
  ('conversion_new_paying_multi','新充转化',   'New Paying Conversion Multi',  'multi',  'DWD'),
  ('conversion_old_paying_multi','老充转化',   'Old Paying Conversion Multi',  'multi',  'DWD'),
  ('arppu',                    'ARPPU',        'Avg Revenue Per Paying User',  'amount', 'DWD');
```

---

## 6. ETL 流程与 CLI 设计

### 6.1 CLI 子命令一览

| Makefile 命令 | `cmd/main.go` flag | 行为 |
|---|---|---|
| `make ch-migrate` | `--ch:migrate` | 按编号顺序执行 `database/clickhouse/migrations/*.sql`，建表/视图 |
| `make sync-ods [mode=incremental\|full] [source=xxx]` | `--sync:ods` | Sheet → ODS；`incremental` 默认（当天+前 7 天），`full` 全量 |
| `make sync-dim [source=xxx]` | `--sync:dim` | ODS → DIM，更新产品维表 |
| `make sync-dwd [from=YYYY-MM-DD] [to=YYYY-MM-DD]` | `--sync:dwd` | ODS → DWD，默认最近 7 天 |
| `make sync-dws [from=...] [to=...]` | `--sync:dws` | DWD → DWS（product + team + bu 三张表） |
| `make sync-ads` | `--sync:ads` | 创建/替换 3 个 ADS 视图 |
| `make sync-all [from=...] [to=...]` | `--sync:all` | 按顺序：ods → dim → dwd → dws → ads |

### 6.2 ETL 依赖顺序

```
sync-ods ──→ sync-dim ──┐
                         ├──→ sync-dwd ──→ sync-dws ──→ sync-ads
sync-ods ──────────────-┘
```

`sync-all` 按此顺序串行执行，任意一步失败立即 abort（CLI exit 1）。

### 6.3 SQL 文件目录

```
database/clickhouse/
├── migrations/               # 按编号建表，ch-migrate 顺序执行
│   ├── 001_ods_product_daily_report.sql
│   ├── 002_dim_product.sql
│   ├── 003_dwd_product_daily_metric.sql
│   ├── 004_dws_product_daily.sql
│   ├── 005_dws_team_daily.sql
│   ├── 006_dws_bu_daily.sql
│   └── 007_ads_views.sql
└── etl/                      # ETL 模板（参数化），Go 读取后绑参执行
    ├── ods_to_dim.sql
    ├── ods_to_dwd.sql
    ├── dwd_to_dws_product.sql
    ├── dwd_to_dws_team.sql
    └── dwd_to_dws_bu.sql
```

---

## 7. 错误处理与幂等性

### 7.1 Sheet 拉取

| 场景 | 策略 |
|------|------|
| HTTP 超时 / 503 | 指数退避 3 次（1s / 3s / 9s），全部失败则 CLI exit 1 |
| 302 跳到 `accounts.google.com` | 报错"Sheet 未公开"，提示用户检查共享权限 |
| 最终 Content-Type 非 CSV | 报错，避免把 HTML 当 CSV 解析 |

### 7.2 CSV 解析

| 场景 | 策略 |
|------|------|
| 某行列数 ≠ 表头 | 丢弃该行，记 `WARN` 日志（带 `_src_row_no`），继续处理其余行 |
| 异常行比例 > 5% | 整批 abort，防止脏数据大规模写入 |
| 数值字段为空/非数字 | 宽容解析：空→ 0 / NULL；写 `WARN` |

### 7.3 CH 写入

| 场景 | 策略 |
|------|------|
| ODS 重跑 | `ReplacingMergeTree` 天然幂等：同 `(date, product_code, _source)` 的行以 `_version` 最大的为准 |
| DWD ETL 重跑 | `INSERT INTO ... SELECT FINAL` 是整批原子操作，重跑安全 |
| DWS 重算 | `DROP PARTITION + INSERT`；分两步执行，崩溃后重跑时先检查分区是否存在再决定是否先 DROP |
| 并发同时跑 sync-all | 本期通过运维约定避免（CLI 人工调用），不做分布式锁（YAGNI） |

### 7.4 数据质量自检（每次 sync-dwd 后自动执行）

```sql
-- 规则 1：DWD 主键唯一性
SELECT count() - count(DISTINCT (date, product_code)) AS dup_cnt
FROM dwd_product_daily_metric FINAL
WHERE date >= {from_date:Date} AND date <= {to_date:Date};
-- dup_cnt > 0 → WARN 日志

-- 规则 2：关键字段非负
SELECT count() AS invalid_cnt
FROM dwd_product_daily_metric FINAL
WHERE date >= {from_date:Date}
  AND (dau < 0 OR recharge_total_amt < 0);
-- > 0 → WARN 日志

-- 规则 3：比率类字段范围合理（环比 [-1, 50] 以外视为异常）
SELECT count() AS out_of_range
FROM dwd_product_daily_metric FINAL
WHERE date >= {from_date:Date}
  AND (dau_chain_ratio < -1 OR dau_chain_ratio > 50);
-- > 0 → WARN 日志（不 abort，打警告）
```

---

## 8. 测试策略

| 层 | 工具 | 测试目标 |
|----|------|---------|
| **单元测试** | `go test` | CSV 解析器（空 cell、BOM、非数字宽容）；字段 mapper（中→英）；类型转换（"0.7173" → Float64） |
| **集成测试** | `go test` + Docker Compose CH | 一条假数据走完整 ods → dwd → dws → ads 断言最终查询结果 |
| **Migration smoke** | `make ch-migrate` | 7 张表 + 3 视图都被成功建出 |
| **DQ 自检** | sync-dwd 后自动跑 §7.4 规则 | 发现问题写 WARN 日志 |
| **Golden 回归** | 用固定 CSV fixture 对比 SELECT 结果 | 修改 mapper 时不破坏既有行为 |

---

## 9. 代码组织

### 新增 / 修改文件一览

```
DataPilot/
├── cmd/
│   ├── main.go              [改] 增加 --ch:migrate / --sync:* dispatch
│   ├── test_gsheet.go       [不动]
│   └── sync_gsheet.go       [新] CLI 参数解析 + 各 sync 子命令入口
│
├── database/
│   ├── migrations/          [不动] 现有 PG migrations
│   ├── pg_seeds/            [新] meta_metric_dict seed SQL
│   └── clickhouse/          [新]
│       ├── migrations/      [新] 001~007 建表/视图 SQL
│       └── etl/             [新] 5 个 ETL 模板 SQL
│
├── modules/
│   └── gsheet/              [新模块，仿 user/auth 风格]
│       ├── dto/
│       │   ├── ods_row.go       [新] 中文字段结构体（与 ODS DDL 对齐）
│       │   └── parse_warning.go [新] 解析警告结构
│       ├── mapper/
│       │   └── ods_mapper.go    [新] 中文→英文字段映射（唯一耦合点）
│       ├── repository/
│       │   ├── ods_repo.go      [新] PrepareBatch 写 ODS
│       │   └── etl_repo.go      [新] 读 SQL 文件 + 绑参执行 + DQ 检查
│       └── service/
│           ├── fetcher.go       [新] CSV 拉取（公开 HTTP）
│           ├── ods_loader.go    [新] CSV → ODS 主流程（解析 + 校验 + 写入）
│           ├── etl_runner.go    [新] 按依赖顺序跑各 ETL SQL
│           └── dq_checker.go   [新] DQ 规则执行器
│
├── providers/
│   └── core.go              [改] 注入 GSheetODSLoader、ETLRunner 等
│
├── pkg/constants/
│   └── common.go            [改] 增加 GSheetODSLoader 等 DI key
│
├── Makefile                 [改] 增加 ch-migrate-* / sync-* targets
│
└── docs/superpowers/specs/
    └── 2026-05-19-gsheet-to-clickhouse-datapilot-design.md  [本文档]
```

### 模块化原则

- **一个文件一件事**：`ods_loader.go` 只做 CSV→ODS，`etl_runner.go` 只做 SQL 按序执行
- **SQL 文件与 Go 分离**：所有 CH SQL 在 `.sql` 文件里，Go 仅读取并绑定参数执行
- **Mapper 是唯一耦合点**：中英字段映射全在 `ods_mapper.go`，新增源字段只改这一处
- **DI 一致**：沿用 `samber/do` + `do.ProvideNamed`，与现有 PG/CH/Google API 风格一致

---

## 10. 风险评估

| 风险 | 概率 | 影响 | 缓解 |
|------|------|------|------|
| Sheet 结构变更（加列、改列名） | 中 | 高（ODS 写入失败） | ODS 采用"宽容解析"；新列落 NULL 而非 abort；mapper 缺失的列发 WARN |
| "总转化"字段语义确认错误（_multi vs _ratio） | 低 | 中（ADS 分析结论偏差） | 先用 `_multi` 落库，视图层可随时修改计算逻辑无需迁移 |
| CH ReplacingMergeTree 延迟去重 | 低 | 低（FINAL 查询时自动处理） | 所有 DWD 以上层统一走 FINAL 或在 DWS 重算时消化 |
| 多 Sheet 接入时 product_code 冲突 | 中 | 中（DIM/DWD 数据错乱） | `_source` 在 ORDER BY 中已隔离；DIM 按 `(product_code, _source)` 分表管理 |
| 公开 Sheet URL 失效 / 权限被撤 | 低 | 高（采集中断） | 告警：CSV 拉取失败写 ERROR 日志 + cron 邮件通知；监控连续 N 次失败 |

---

## 11. 下一期展望

| 方向 | 说明 |
|------|------|
| **AI Agent（最高优先级）** | 基于 ADS 层 + `meta_metric_dict`，接入 LLM（Claude/GPT），能回答"哪个产品7留率异常"、"预测下周充值趋势"等自然语言问题 |
| **更多 Sheet 接入** | 新 Sheet 只需：① 新建 `ods_<source>` 表；② 新写一份 mapper；③ 新写 etl SQL。现有分层不变 |
| **业务 PG/MySQL 接入** | 通过 Debezium CDC 或定时抽取，走相同 ODS → DWD 分层，DIM 做 ID 映射 |
| **定时调度管理** | 在 Gin 里加 `/admin/sync` HTTP API，配合外部调度器（Airflow/k8s CronJob）或内置 cron goroutine |
| **DQ Dashboard** | 把 `§7.4` 的数据质量检查结果写入 PG `etl_dq_reports` 表，配合简单 Gin API 展示 |
| **DIM SCD-2** | 当产品归属小组/部门发生变更时记录历史快照，DWD 按日期做时态 JOIN |
| **指标口径版本管理** | `meta_metric_dict` 加 `version` + `deprecated_at`，支持口径变更追溯 |
