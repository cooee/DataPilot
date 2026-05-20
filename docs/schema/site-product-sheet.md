# 站点产品 Sheet 结构（gid=553168897）

## 来源

| 项 | 值 |
|----|-----|
| Spreadsheet ID | `1sJZBMAHBa3QtmSKQ8oLGaiC_w-8kloGJizRNHnbQ-jw` |
| gid | `553168897` |
| Tab 名称 | 站点产品（业务口径：导量 + 活跃） |
| CSV 导出 | `https://docs.google.com/spreadsheets/d/{id}/export?format=csv&gid=553168897` |
| `product_type` | **`site`**（与 `paid` / `free` 隔离） |

## 表头（11 列）

| # | 列名 | DWD 英文字段 | 类型 |
|---|------|-------------|------|
| 1 | 日期 | `date` | Date |
| 2 | 产品名称 | _(dim)_ | String |
| 3 | 产品编号 | `product_code` | String |
| 4 | 小组 | _(dim)_ | String，可空 |
| 5 | 部门 | _(dim)_ | String，可空 |
| 6 | 日活跃数 | `dau` | UInt64 |
| 7 | 日活环比 | `dau_chain_ratio` | Float64 |
| 8 | 日导量新增 | `lead_new_cnt` | UInt64 |
| 9 | 日导量新增环比 | `lead_new_chain_ratio` | Float64 |
| 10 | 日导量充值 | `lead_recharge_amt` | Float64 |
| 11 | 日导量充值环比 | `lead_recharge_chain_ratio` | Float64 |

> 付费/免费 Sheet 为 40 列（`日活` / `总充值` 等），**不可**复用 `mapper.ExpectedHeaders`。

## 数据概况（2026-05 抽样）

- 数据行约 **183**（随 Sheet 更新变化）
- 日期约 `2026-05-01` ~ `2026-05-18`
- 产品 **11** 个，编号均为 `JHA-*`
- 与付费 sheet（gid=0）**产品编号无交集**
- `JHA-2` 小组/部门常为空；`JHA-1020` 自 5/3 起出现；末日可能不完整

## 数仓表（平行支线）

| 层 | 表名 |
|----|------|
| ODS | `ods_site_product_daily` |
| DIM | `dim_site_product` |
| DWD | `dwd_site_product_daily_metric` |
| DWS | `dws_site_product_daily` |

## CLI

```bash
# 建表
make ch-migrate

# 仅 ODS
./datapilot --sync:ods:site -from=2026-05-01 -to=2026-05-18

# ODS → DIM → DWD → DWS
./datapilot --sync:site:all -from=2026-05-01 -to=2026-05-18
```

`--sync:all` 已包含站点产品（step 3/3）；也可单独用 `--sync:ods:site` / `--sync:site:all`。

## 与 paid 指标对比注意

- 站点 `dau`（日活跃数）与 paid `dau`（日活）列名不同，业务需确认是否同口径再对比。
- 站点 `lead_recharge_amt` ≠ paid `recharge_total_amt`。
