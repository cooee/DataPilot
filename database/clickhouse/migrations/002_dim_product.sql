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
ORDER BY (product_code, effective_from)
