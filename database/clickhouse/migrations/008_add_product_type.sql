-- Migration 008: 各层新增 product_type 字段
-- product_type: 'paid'（付费产品）/ 'free'（免费产品）
-- ODS 已有 _source 字段可区分，但为让 DWD/DWS 也能按类型聚合，需显式新增列。

-- ODS: 追加 product_type 列（位于 _source 之前）
ALTER TABLE ods_product_daily_report
    ADD COLUMN IF NOT EXISTS product_type LowCardinality(String) DEFAULT 'paid';

-- DIM: 追加 product_type
ALTER TABLE dim_product
    ADD COLUMN IF NOT EXISTS product_type LowCardinality(String) DEFAULT 'paid';

-- DWD: 追加 product_type
-- 注意：DWD ORDER BY 包含 (date, product_code)，不含 product_type；
-- 如果同一产品同日期有 paid/free 两条记录，ReplacingMergeTree 会保留最新版本。
-- 若未来需要 paid/free 分开保留，需重建表并把 product_type 加入 ORDER BY。
ALTER TABLE dwd_product_daily_metric
    ADD COLUMN IF NOT EXISTS product_type LowCardinality(String) DEFAULT 'paid';

-- DWS product (team/bu 汇总层通过 WHERE product_type 过滤，无需加列)
ALTER TABLE dws_product_daily
    ADD COLUMN IF NOT EXISTS product_type LowCardinality(String) DEFAULT 'paid'
