CREATE TABLE IF NOT EXISTS dws_team_daily (
    date                       Date,
    team                       LowCardinality(String),
    business_unit              LowCardinality(String),
    active_product_cnt         UInt32,
    dau_sum                    UInt64,
    recharge_total_amt_sum     Decimal(18, 2),
    recharge_organic_amt_sum   Decimal(18, 2),
    recharge_channel_amt_sum   Decimal(18, 2),
    recharge_internal_amt_sum  Decimal(18, 2),
    new_user_total_cnt_sum     UInt64,
    new_user_organic_cnt_sum   UInt64,
    new_user_channel_cnt_sum   UInt64,
    new_paying_user_cnt_sum    UInt64,
    old_paying_user_cnt_sum    UInt64,
    recharge_order_cnt_sum     UInt64,
    old_user_dau_sum           UInt64,
    retention_d7_ratio_num     Float64,
    retention_d7_ratio_den     Float64,
    `_built_at`                DateTime64(3) DEFAULT now64()
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(date)
ORDER BY (date, team)
