CREATE OR REPLACE VIEW ads_product_health_overview AS
SELECT
    product_code, product_name, team, business_unit,
    avg(dau)                   AS avg_dau_30d,
    sum(recharge_total_amt)    AS recharge_total_30d,
    avg(retention_d7_ratio)    AS avg_retention_d7_30d,
    avg(payment_success_ratio) AS avg_payment_success_30d,
    avg(arppu)                 AS avg_arppu_30d,
    least(1, greatest(0,
          0.3 * (avg(retention_d7_ratio) / 0.05)
        + 0.3 * (avg(payment_success_ratio) / 0.7)
        + 0.4 * least(1, toFloat64(sum(recharge_total_amt)) / 1000000)
    )) AS health_score
FROM dws_product_daily
WHERE date >= today() - 30
GROUP BY product_code, product_name, team, business_unit;

CREATE OR REPLACE VIEW ads_team_performance_daily AS
SELECT
    date, team, business_unit, active_product_cnt, dau_sum,
    recharge_total_amt_sum, new_paying_user_cnt_sum, recharge_order_cnt_sum,
    if(retention_d7_ratio_den > 0,
       retention_d7_ratio_num / retention_d7_ratio_den, 0) AS retention_d7_ratio_wavg
FROM dws_team_daily
WHERE date >= today() - 90;

CREATE OR REPLACE VIEW ads_anomaly_baseline AS
SELECT product_code, 'dau' AS metric,
    avg(dau) AS mean, stddevPop(dau) AS std,
    avg(dau) + 3*stddevPop(dau) AS upper_3sigma,
    avg(dau) - 3*stddevPop(dau) AS lower_3sigma
FROM dws_product_daily WHERE date >= today() - 30 GROUP BY product_code
UNION ALL
SELECT product_code, 'recharge_total_amt',
    avg(recharge_total_amt), stddevPop(recharge_total_amt),
    avg(recharge_total_amt) + 3*stddevPop(recharge_total_amt),
    avg(recharge_total_amt) - 3*stddevPop(recharge_total_amt)
FROM dws_product_daily WHERE date >= today() - 30 GROUP BY product_code
UNION ALL
SELECT product_code, 'retention_d7_ratio',
    avg(retention_d7_ratio), stddevPop(retention_d7_ratio),
    avg(retention_d7_ratio) + 3*stddevPop(retention_d7_ratio),
    avg(retention_d7_ratio) - 3*stddevPop(retention_d7_ratio)
FROM dws_product_daily WHERE date >= today() - 30 GROUP BY product_code
