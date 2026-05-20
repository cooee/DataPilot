-- site ETL 03: DWD + DIM → dws_site_product_daily
-- 参数: {date_from:Date}  {date_to:Date}

INSERT INTO dws_site_product_daily (
    date, product_code, product_name, team, business_unit,
    dau, dau_chain_ratio,
    lead_new_cnt, lead_new_chain_ratio,
    lead_recharge_amt, lead_recharge_chain_ratio,
    product_type, _built_at
)
SELECT
    d.date,
    d.product_code,
    coalesce(dim.product_name, '')  AS product_name,
    coalesce(dim.team, '')          AS team,
    coalesce(dim.business_unit, '') AS business_unit,
    d.dau,
    d.dau_chain_ratio,
    d.lead_new_cnt,
    d.lead_new_chain_ratio,
    d.lead_recharge_amt,
    d.lead_recharge_chain_ratio,
    d.product_type,
    now64() AS _built_at
FROM dwd_site_product_daily_metric AS d FINAL
LEFT JOIN (
    SELECT
        product_code,
        argMax(product_name,  _ingested_at) AS product_name,
        argMax(team,          _ingested_at) AS team,
        argMax(business_unit, _ingested_at) AS business_unit
    FROM dim_site_product FINAL
    GROUP BY product_code
) AS dim ON d.product_code = dim.product_code
WHERE d.date BETWEEN {date_from:Date} AND {date_to:Date}
