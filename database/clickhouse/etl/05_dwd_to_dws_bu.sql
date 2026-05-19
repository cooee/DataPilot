-- ETL 05: DWD + DIM → dws_bu_daily（部门日汇总）
-- 结构与 dws_team_daily 对称，聚合粒度为 business_unit。
-- 调用前 ETLRunner 已按月 DROP PARTITION，此处直接 INSERT。
--
-- 参数:
--   {date_from:Date}
--   {date_to:Date}

INSERT INTO dws_bu_daily
SELECT
    d.date,
    coalesce(dim.business_unit, '') AS business_unit,
    countDistinct(d.product_code)   AS active_product_cnt,
    sum(d.dau)                      AS dau_sum,
    sum(d.recharge_total_amt)       AS recharge_total_amt_sum,
    sum(d.recharge_organic_amt)     AS recharge_organic_amt_sum,
    sum(d.recharge_channel_amt)     AS recharge_channel_amt_sum,
    sum(d.recharge_internal_amt)    AS recharge_internal_amt_sum,
    sum(d.new_user_total_cnt)       AS new_user_total_cnt_sum,
    sum(d.new_user_organic_cnt)     AS new_user_organic_cnt_sum,
    sum(d.new_user_channel_cnt)     AS new_user_channel_cnt_sum,
    sum(d.new_paying_user_cnt)      AS new_paying_user_cnt_sum,
    sum(d.old_paying_user_cnt)      AS old_paying_user_cnt_sum,
    sum(d.recharge_order_cnt)       AS recharge_order_cnt_sum,
    sum(d.old_user_dau)             AS old_user_dau_sum,
    sum(d.retention_d7_ratio * d.dau) AS retention_d7_ratio_num,
    sum(d.dau)                        AS retention_d7_ratio_den,
    now64() AS _built_at
FROM dwd_product_daily_metric FINAL AS d
LEFT JOIN (
    SELECT
        product_code,
        argMax(business_unit, _ingested_at) AS business_unit
    FROM dim_product FINAL
    GROUP BY product_code
) AS dim ON d.product_code = dim.product_code
WHERE d.date BETWEEN {date_from:Date} AND {date_to:Date}
GROUP BY d.date, dim.business_unit
