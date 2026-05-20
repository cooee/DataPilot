# DataPilot AI CLI 参考（供 LLM / Agent 使用）

> **目的**：让大模型在编排查询时，知道如何从 ClickHouse 获取 **付费 / 免费 / 站点** 三类产品数据，且不误用表或指标。  
> 人类用户也可作速查；完整参数见 [docs/cli.md](../cli.md)。

---

## 1. 三条产品线（必须区分）

| 产品线 | `product_type` | Google Sheet gid | DWS 查询表 | 说明 |
|--------|----------------|------------------|------------|------|
| 付费产品 | `paid` | `0` | `dws_product_daily` | 40 列宽表：日活、总充值、留存、ARPPU 等 |
| 免费产品 | `free` | `469519483` | `dws_product_daily` | 同上，用 `product_type=free` 过滤 |
| **站点产品** | `site` | `553168897` | **`dws_site_product_daily`** | **11 列独立表**：日活跃数、日导量新增、日导量充值 |

**硬性规则（LLM 编排时遵守）：**

1. 站点问题 **禁止** 使用 `product-type-compare`、`dws_product_daily` + `product_type=site`（该表无 site 行）。
2. 站点指标 **禁止** 使用 `recharge_total_amt`、`retention_d7_ratio`、`new_paying_user_cnt`（站点表无这些列）。
3. 站点充值口径为 **`lead_recharge_amt`（日导量充值）**，≠ 付费 **`recharge_total_amt`（总充值）**。
4. 站点日活字段在 Sheet 为「日活跃数」，语义层指标键仍为 **`dau`**。
5. 异常检测 preset（`anomaly-detection`）**仅覆盖 paid/free ADS**，站点暂无 ±3σ 视图。

Schema 细节：[docs/schema/site-product-sheet.md](../schema/site-product-sheet.md)。

---

## 2. 数据就绪：同步 CLI（查询前必做）

```bash
go build -o datapilot ./cmd
make ch-migrate   # 首次建表（含 ods/dim/dwd/dws_site_*）

# 推荐：一键同步三条线 ODS + 各自 ETL
./datapilot --sync:all -from=YYYY-MM-DD -to=YYYY-MM-DD

# 仅站点（ODS → DIM → DWD → DWS）
./datapilot --sync:site:all -from=YYYY-MM-DD -to=YYYY-MM-DD
./datapilot --sync:ods:site -from=YYYY-MM-DD -to=YYYY-MM-DD   # 仅 ODS + DQ
```

`--sync:all` 顺序：gid=0 付费 → gid=469519483 免费 → gid=553168897 站点 → paid/free ETL → site ETL。

**验证站点 DWS 有数：**

```bash
docker exec clickhouse-server clickhouse-client --password 123456 -q "
SELECT date, count() AS rows, sum(dau) AS dau, sum(lead_recharge_amt) AS lead_recharge
FROM dws_site_product_daily GROUP BY date ORDER BY date DESC LIMIT 5"
```

---

## 3. 语义层查询：`--analytics:query`

白名单数据集（`./datapilot --analytics:schema` 可列出全集）。

### 3.1 付费 / 免费（`dws_product_daily`）

| preset | 适用问题 | 主要维度 |
|--------|----------|----------|
| `product-type-compare` | 某日付费 vs 免费汇总 | `product_type` |
| `product-detail` | 产品明细、充值排序 | `date`, `product_code`, `product_name`, `team` |
| `product-trend` | 多日 paid/free 走势 | `date`, `product_type` |
| `team-performance` | 小组日表现（ADS） | `date`, `team` |
| `product-health` | 健康度 Top N | `product_code` |
| `anomaly-detection` | 报告日 ±3σ 异常 | `product_code`, `metric` |
| `anomaly-watch` | 偏离度 Top（含未达 3σ） | 同上 |
| `anomaly-baseline` | 30 日基线区间 | `product_code`, `metric` |

```bash
./datapilot --analytics:query -preset=product-type-compare -from=2026-05-18 -to=2026-05-18
./datapilot --analytics:query -preset=product-detail -from=2026-05-18 -to=2026-05-18 \
  -filter=product_type:eq:paid
```

### 3.2 站点产品（`dws_site_product_daily`）★

| preset | 适用问题 | 主要指标 |
|--------|----------|----------|
| `site-product-summary` | 站点日汇总 | `dau`, `lead_new_cnt`, `lead_recharge_amt` |
| `site-product-detail` | 站点产品明细（按导量充值降序） | 同上 + 环比 |
| `site-product-trend` | 站点多日走势 | 按 `date` 聚合 |
| `site-team-summary` | 站点按小组汇总 | `team` + 上述指标 |

```bash
./datapilot --analytics:query -preset=site-product-summary -from=2026-05-18 -to=2026-05-18
./datapilot --analytics:query -preset=site-product-detail -from=2026-05-18 -to=2026-05-18
./datapilot --analytics:query -preset=site-product-trend -from=2026-05-01 -to=2026-05-18
./datapilot --analytics:query -preset=site-team-summary -from=2026-05-18 -to=2026-05-18
```

**自定义查询（站点）：**

```bash
./datapilot --analytics:query \
  -dataset=dws_site_product_daily \
  -metrics=dau,lead_new_cnt,lead_recharge_amt \
  -dimensions=product_code,product_name \
  -from=2026-05-18 -to=2026-05-18 \
  -order=lead_recharge_amt:desc -limit=20
```

输出：`-format=table`（默认）或 `-format=json`（推荐 Agent 解析）。

---

## 4. 自然语言 Agent：`--agent:ask`

```
用户问题 → Compiler（LLM 优先，失败则 RulePlanner）
         → 选定 preset + QueryRequest
         → AnalyticsService 查 ClickHouse
         → 中文 answer 摘要 + JSON（query / data / meta）
```

### 4.1 环境

```bash
# .env
AGENT_LLM_PROVIDER=gemini          # 或 stub（纯规则）
GEMINI_API_KEY=...
GEMINI_MODEL=gemini-2.0-flash
AGENT_SKILL_PATH=skills/datapilot-daily-ops-report/SKILL.md
```

### 4.2 站点 NL → preset 映射（规则引擎）

| 用户表述关键词 | 选用 preset |
|----------------|-------------|
| 站点、站点产品、导量、日导量、日活跃数 | `site-product-summary`（默认） |
| 站点 + 明细 / 排行 / 列表 | `site-product-detail` |
| 站点 + 趋势 / 走势 / 近7天 | `site-product-trend` |
| 站点 + 小组 / 团队 | `site-team-summary` |
| 付费、免费、对比、vs | `product-type-compare`（**非站点**） |
| 异常、离群、告警 | `anomaly-detection`（**非站点**） |

```bash
./datapilot --agent:ask -q="2026-05-18 站点产品日活跃和导量充值汇总" -date=2026-05-18
./datapilot --agent:ask -q="站点产品明细排行" -date=2026-05-18
./datapilot --agent:ask -preset=site-product-detail -date=2026-05-18   # 跳过 NL，直接指定
```

响应字段：`preset`、`source`（`llm`|`rule`）、`interpretation`、`answer`（中文）、`data`（行数据）、`query`（语义层请求体）。

### 4.3 LLM 编排 JSON 约定

Gemini 须输出 preset 名（英文，与上表一致）。站点相关示例：

```json
{
  "intent": "metric_qa",
  "preset": "site-product-summary",
  "interpretation": "查询 2026-05-18 站点产品日汇总",
  "query": {
    "dataset": "dws_site_product_daily",
    "metrics": ["dau", "lead_new_cnt", "lead_recharge_amt"],
    "dimensions": ["date"],
    "date_range": { "from": "2026-05-18", "to": "2026-05-18" },
    "limit": 10
  }
}
```

---

## 5. 指标词典（`meta_metric_dict`）

站点专用键（`make seed` 写入 PostgreSQL，供 Agent glossary）：

| metric_key | 中文名 | 数据集 |
|------------|--------|--------|
| `dau` | 日活 / 日活跃数 | 三线共用键名；站点来自「日活跃数」列 |
| `lead_new_cnt` | 日导量新增 | 仅 `dws_site_product_daily` |
| `lead_new_chain_ratio` | 日导量新增环比 | 仅站点 |
| `lead_recharge_amt` | 日导量充值 | 仅站点 |
| `lead_recharge_chain_ratio` | 日导量充值环比 | 仅站点 |

付费/免费常用键：`recharge_total_amt`、`retention_d7_ratio`、`new_paying_user_cnt`、`arppu` 等——**不要用于站点 preset**。

---

## 6. 三线对比时的正确做法

当前 **没有** 单一 preset 同时返回 paid + free + site（口径不同）。

LLM 应 **分两次查询**，再合并叙述：

1. `./datapilot --analytics:query -preset=product-type-compare -from=DATE -to=DATE`
2. `./datapilot --analytics:query -preset=site-product-summary -from=DATE -to=DATE`

或两次 `--agent:ask`，分别问付费免费与站点。

---

## 7. HTTP API（可选）

需 JWT，与 CLI 同一语义层：

- `GET /api/analytics/schema`
- `POST /api/analytics/query` — body 同 `QueryRequest` JSON
- `POST /api/agent/metrics/ask` — `{"question":"...","date":"YYYY-MM-DD"}`
- `POST /api/agent/anomalies` — 仅 paid/free 异常

---

## 8. 故障排查

| 现象 | 原因 | 处理 |
|------|------|------|
| 站点查询 0 行 | 未 sync 站点 | `--sync:all` 或 `--sync:site:all` |
| Agent 仍走 product-type-compare | 问题未含「站点/导量」 | 问题中明确「站点产品」，或 `-preset=site-product-*` |
| 未知 dataset | 用了中文表名 | 必须用 `dws_site_product_daily` |
| 指标报错 | 在站点表查 `recharge_total_amt` | 改用 `lead_recharge_amt` |

---

## 9. 相关文档

- [docs/cli.md](../cli.md) — 全量 CLI / Makefile
- [docs/agent/README.md](README.md) — Agent 架构与 Gemini 配置
- [docs/schema/site-product-sheet.md](../schema/site-product-sheet.md) — Sheet 列与数仓表
- [skills/datapilot-daily-ops-report/SKILL.md](../../skills/datapilot-daily-ops-report/SKILL.md) — 注入 Gemini 的 Skill 正文
