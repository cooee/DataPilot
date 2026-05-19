-- ETL 02: ODS → DWD（中文列 → 英文规范列）
-- 使用 FINAL 保证 ReplacingMergeTree 去重后再读取。
-- dwd_product_daily_metric 同样使用 ReplacingMergeTree(ORDER BY date,product_code)，
-- 重复写入自动以最新 _version 获胜，幂等安全。
--
-- 参数:
--   {date_from:Date}
--   {date_to:Date}

INSERT INTO dwd_product_daily_metric
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
    `_src_row_no`,
    now64()            AS _ingested_at
FROM ods_product_daily_report FINAL
WHERE `日期` BETWEEN {date_from:Date} AND {date_to:Date}
