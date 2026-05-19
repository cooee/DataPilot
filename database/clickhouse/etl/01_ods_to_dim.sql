-- ETL 01: ODS → DIM（产品维表刷新）
-- 以 product_code 为主键去重，取最新出现的名称/小组/部门信息。
-- dim_product 使用 ReplacingMergeTree，重复写入幂等安全。
--
-- 参数:
--   {date_from:Date}  起始日期（含）
--   {date_to:Date}    结束日期（含）

INSERT INTO dim_product
    (product_code, product_name, team, business_unit, _source, _ingested_at)
SELECT
    `产品编号`              AS product_code,
    argMax(`产品名称`, `_ingested_at`) AS product_name,
    argMax(`小组`,     `_ingested_at`) AS team,
    argMax(`部门`,     `_ingested_at`) AS business_unit,
    'ods:sync'              AS _source,
    now64()                 AS _ingested_at
FROM ods_product_daily_report FINAL
WHERE `日期` BETWEEN {date_from:Date} AND {date_to:Date}
GROUP BY `产品编号`
