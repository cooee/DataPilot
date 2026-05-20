-- Migration 010: 站点产品平行管线（gid=553168897, product_type=site）

CREATE TABLE IF NOT EXISTS ods_site_product_daily (
    `日期`               Date,
    `产品名称`           String,
    `产品编号`           LowCardinality(String),
    `小组`               LowCardinality(String),
    `部门`               LowCardinality(String),
    `日活跃数`           UInt64,
    `日活环比`           Float64,
    `日导量新增`         UInt64,
    `日导量新增环比`     Float64,
    `日导量充值`         Float64,
    `日导量充值环比`     Float64,
    product_type         LowCardinality(String) DEFAULT 'site',
    `_source`            LowCardinality(String) DEFAULT 'gsheet:site',
    `_src_row_no`        UInt32,
    `_ingested_at`       DateTime64(3) DEFAULT now64(),
    `_version`           UInt64 MATERIALIZED toUnixTimestamp64Milli(`_ingested_at`)
)
ENGINE = ReplacingMergeTree(`_version`)
PARTITION BY toYYYYMM(`日期`)
ORDER BY (`_source`, `日期`, `产品编号`)
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS dim_site_product (
    product_code      LowCardinality(String),
    product_name      String,
    team              LowCardinality(String),
    business_unit     LowCardinality(String),
    product_type      LowCardinality(String) DEFAULT 'site',
    is_active         UInt8    DEFAULT 1,
    effective_from    Date     DEFAULT today(),
    effective_to      Nullable(Date),
    `_source`         LowCardinality(String) DEFAULT 'gsheet:site',
    `_ingested_at`    DateTime64(3) DEFAULT now64(),
    `_version`        UInt64 MATERIALIZED toUnixTimestamp64Milli(`_ingested_at`)
)
ENGINE = ReplacingMergeTree(`_version`)
ORDER BY (product_code, effective_from);

CREATE TABLE IF NOT EXISTS dwd_site_product_daily_metric (
    date                      Date,
    product_code              LowCardinality(String),
    dau                       UInt64,
    dau_chain_ratio           Float64,
    lead_new_cnt              UInt64,
    lead_new_chain_ratio      Float64,
    lead_recharge_amt         Float64,
    lead_recharge_chain_ratio Float64,
    product_type              LowCardinality(String) DEFAULT 'site',
    `_src_row_no`             UInt32,
    `_ingested_at`            DateTime64(3) DEFAULT now64(),
    `_version`                UInt64 MATERIALIZED toUnixTimestamp64Milli(`_ingested_at`)
)
ENGINE = ReplacingMergeTree(`_version`)
PARTITION BY toYYYYMM(date)
ORDER BY (date, product_code);

CREATE TABLE IF NOT EXISTS dws_site_product_daily (
    date                      Date,
    product_code              LowCardinality(String),
    product_name              String,
    team                      LowCardinality(String),
    business_unit             LowCardinality(String),
    dau                       UInt64,
    dau_chain_ratio           Float64,
    lead_new_cnt              UInt64,
    lead_new_chain_ratio      Float64,
    lead_recharge_amt         Float64,
    lead_recharge_chain_ratio Float64,
    product_type              LowCardinality(String) DEFAULT 'site',
    `_built_at`               DateTime64(3) DEFAULT now64()
)
ENGINE = MergeTree()
PARTITION BY toYYYYMM(date)
ORDER BY (date, product_code);
