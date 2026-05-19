-- ETL 02: ODS → DWD（中文列 → 英文规范列）
-- 使用显式列名避免 ALTER TABLE 追加列后的列序错位问题。
--
-- 参数:
--   {date_from:Date}
--   {date_to:Date}

INSERT INTO dwd_product_daily_metric (
    date, product_code,
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
    arppu, product_type, _src_row_no, _ingested_at
)
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
    product_type,
    `_src_row_no`,
    now64()            AS _ingested_at
FROM ods_product_daily_report FINAL
WHERE `日期` BETWEEN {date_from:Date} AND {date_to:Date}
