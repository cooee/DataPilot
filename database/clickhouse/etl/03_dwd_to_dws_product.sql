-- ETL 03: DWD + DIM → dws_product_daily（产品日汇总，带维度冗余）
-- 调用前 ETLRunner 已按月 DROP PARTITION，此处直接 INSERT。
-- 使用 LEFT JOIN dim_product 冗余产品名称/小组/部门，方便 BI 直接查询。
--
-- 参数:
--   {date_from:Date}
--   {date_to:Date}

INSERT INTO dws_product_daily
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
