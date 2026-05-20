-- site ETL 02: ODS → dwd_site_product_daily_metric
-- 参数: {date_from:Date}  {date_to:Date}

INSERT INTO dwd_site_product_daily_metric (
    date, product_code,
    dau, dau_chain_ratio,
    lead_new_cnt, lead_new_chain_ratio,
    lead_recharge_amt, lead_recharge_chain_ratio,
    product_type, _src_row_no, _ingested_at
)
SELECT
    `日期`             AS date,
    `产品编号`         AS product_code,
    `日活跃数`         AS dau,
    `日活环比`         AS dau_chain_ratio,
    `日导量新增`       AS lead_new_cnt,
    `日导量新增环比`   AS lead_new_chain_ratio,
    `日导量充值`       AS lead_recharge_amt,
    `日导量充值环比`   AS lead_recharge_chain_ratio,
    'site'             AS product_type,
    `_src_row_no`,
    now64()            AS _ingested_at
FROM ods_site_product_daily FINAL
WHERE `日期` BETWEEN {date_from:Date} AND {date_to:Date}
