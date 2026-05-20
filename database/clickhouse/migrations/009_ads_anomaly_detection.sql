-- Migration 009: 产品指标异常检测（滚动基线，不含当日）
-- ≥2 个历史样本：±3σ；仅 1 个历史样本：环比偏离 ≥30% 视为异常。

CREATE OR REPLACE VIEW ads_product_metric_long AS
SELECT
    d.date,
    d.product_code,
    d.product_name,
    d.team,
    d.product_type,
    m.1 AS metric,
    m.2 AS value
FROM dws_product_daily AS d
ARRAY JOIN [
    tuple('dau', toFloat64(d.dau)),
    tuple('recharge_total_amt', toFloat64(d.recharge_total_amt)),
    tuple('retention_d7_ratio', d.retention_d7_ratio)
] AS m;

CREATE OR REPLACE VIEW ads_anomaly_baseline AS
SELECT
    product_code,
    metric,
    avg(value) AS mean,
    stddevPop(value) AS std,
    mean + 3 * std AS upper_3sigma,
    mean - 3 * std AS lower_3sigma
FROM ads_product_metric_long
WHERE date >= today() - 30
  AND date < today()
GROUP BY product_code, metric
HAVING std > 0;

CREATE OR REPLACE VIEW ads_product_anomaly_daily AS
SELECT
    date,
    product_code,
    product_name,
    team,
    product_type,
    metric,
    value,
    baseline_mean,
    baseline_std,
    baseline_cnt,
    baseline_mean + 3 * baseline_std AS upper_3sigma,
    greatest(baseline_mean - 3 * baseline_std, 0) AS lower_3sigma,
    multiIf(
        baseline_std > 0, abs(value - baseline_mean) / baseline_std,
        baseline_mean > 0, abs(value - baseline_mean) / baseline_mean,
        0
    ) AS z_score,
    multiIf(
        baseline_cnt = 0, 'insufficient_baseline',
        baseline_std > 0 AND (value > baseline_mean + 3 * baseline_std OR value < baseline_mean - 3 * baseline_std), 'high_low_3sigma',
        baseline_cnt = 1 AND baseline_mean > 0 AND abs(value - baseline_mean) / baseline_mean >= 0.3, 'pct_deviation_30',
        'normal'
    ) AS anomaly_direction,
    if(
        (baseline_std > 0 AND (value > baseline_mean + 3 * baseline_std OR value < baseline_mean - 3 * baseline_std))
        OR (baseline_cnt = 1 AND baseline_mean > 0 AND abs(value - baseline_mean) / baseline_mean >= 0.3),
        1, 0
    ) AS is_anomaly
FROM (
    SELECT
        date,
        product_code,
        product_name,
        team,
        product_type,
        metric,
        value,
        avg(value) OVER w AS baseline_mean,
        stddevPop(value) OVER w AS baseline_std,
        count() OVER w AS baseline_cnt
    FROM ads_product_metric_long
    WINDOW w AS (
        PARTITION BY product_code, metric
        ORDER BY date
        ROWS BETWEEN 30 PRECEDING AND 1 PRECEDING
    )
) AS rolled;
