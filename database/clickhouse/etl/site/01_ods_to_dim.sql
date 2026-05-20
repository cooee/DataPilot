-- site ETL 01: ODS → dim_site_product
-- 参数: {date_from:Date}  {date_to:Date}

INSERT INTO dim_site_product
    (product_code, product_name, team, business_unit, product_type, _source, _ingested_at)
SELECT
    `产品编号`                          AS product_code,
    argMax(`产品名称`,  `_ingested_at`) AS product_name,
    argMax(`小组`,      `_ingested_at`) AS team,
    argMax(`部门`,      `_ingested_at`) AS business_unit,
    'site'                              AS product_type,
    'ods:sync:site'                     AS _source,
    now64()                             AS _ingested_at
FROM ods_site_product_daily FINAL
WHERE `日期` BETWEEN {date_from:Date} AND {date_to:Date}
GROUP BY `产品编号`
