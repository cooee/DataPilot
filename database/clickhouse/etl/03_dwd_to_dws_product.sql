-- ETL 03: DWD + DIM → dws_product_daily
-- 使用显式列名，避免 ALTER TABLE 追加列后的列序错位。
--
-- 参数:
--   {date_from:Date}
--   {date_to:Date}

INSERT INTO dws_product_daily (
    date, product_code, product_name, team, business_unit,
    dau, dau_chain_ratio, old_user_dau,
    recharge_total_amt, recharge_total_chain_ratio,
    recharge_organic_amt, recharge_organic_chain_ratio,
    recharge_channel_amt, recharge_channel_chain_ratio,
    recharge_internal_amt, recharge_internal_chain_ratio,
    new_user_total_cnt, new_user_total_chain_ratio,
    new_user_organic_cnt, new_user_organic_chain_ratio,
    new_user_channel_cnt, new_user_channel_chain_ratio,
    new_user_internal_cnt, new_user_internal_chain_ratio,
    new_paying_user_cnt, old_paying_user_cnt,
    new_paying_user_amt, old_paying_user_amt,
    recharge_order_cnt, payment_success_ratio,
    retention_d1_ratio, retention_d3_ratio, retention_d7_ratio,
    landing_page_visit_cnt, landing_page_click_cnt, landing_page_download_ratio,
    conversion_total_multi, conversion_new_paying_multi, conversion_old_paying_multi,
    arppu, product_type, _built_at
)
SELECT
    d.date,
    d.product_code,
    coalesce(dim.product_name, '') AS product_name,
    coalesce(dim.team,         '') AS team,
    coalesce(dim.business_unit,'') AS business_unit,
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
    d.product_type,
    now64() AS _built_at
FROM dwd_product_daily_metric AS d FINAL
LEFT JOIN (
    SELECT
        product_code,
        argMax(product_name,   _ingested_at) AS product_name,
        argMax(team,           _ingested_at) AS team,
        argMax(business_unit,  _ingested_at) AS business_unit
    FROM dim_product FINAL
    GROUP BY product_code
) AS dim ON d.product_code = dim.product_code
WHERE d.date BETWEEN {date_from:Date} AND {date_to:Date}
