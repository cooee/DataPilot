# 每日运营日报 — 标准操作 SOP

> 目标：用真实 Google Sheet 数据完成 **同步 → 数仓 ETL → DQ 质检 → Analytics 看板查询**，输出可给运营/AI 的日报素材。  
> 前置：Docker 中 ClickHouse + PostgreSQL 已启动，项目根目录 `.env` 已配置，`make ch-migrate` 已执行。

---

## 一、环境检查（每次执行前）

```bash
# 1. 容器
docker ps --filter name=clickhouse-server --filter name=postgres-server

# 2. 编译 CLI（含全部子命令）
go build -o datapilot ./cmd

# 3.（首次）PG 指标语义
make migrate-seed
```

---

## 二、一键日报（推荐）

```bash
# 默认：同步「昨天」数据，报告日=昨天，输出表格
make daily-report

# 指定报告日（Sheet 同步区间默认同报告日）
make daily-report ARGS="-date=2026-05-18"

# 同步一周并以其最后一天为报告日
make daily-report ARGS="-from=2026-05-12 -to=2026-05-18 -date=2026-05-18"

# JSON 输出（给 AI / 存档）
make daily-report ARGS="-date=2026-05-18 -format=json"

# 数据已同步，仅刷新 Analytics 三段查询
make daily-report ARGS="-skip-sync -date=2026-05-18"
```

**等价二进制调用：**

```bash
./datapilot --report:daily -date=2026-05-18
```

### 流水线步骤（自动）

| 步骤 | 动作 | 说明 |
|------|------|------|
| 1 | ODS 付费 Sheet `gid=0` | 拉取 CSV → `ods_product_daily_report`，**自动 DQ** |
| 2 | ODS 免费 Sheet `gid=469519483` | 同上，`product_type=free` |
| 3 | ETL | DIM → DWD → DWS（按 `-from`/`-to` 分区重写） |
| 4 | DQ 汇总 | 重复行 / 负值 / 3σ 离群（离群仅警告） |
| 5 | Analytics | 见下表 |

### Analytics 输出（固定三段）

| 段落 | preset | 含义 |
|------|--------|------|
| ① | `product-type-compare` | 报告日 paid vs free 核心指标对比 |
| ② | `team-performance` | 报告日前后 7 日小组表现 |
| ③ | `product-health` | 近 30 日健康分 Top 20 |

---

## 三、分步执行（排障 / 补数时用）

与 [cli.md](../cli.md) 场景 B 一致：

```bash
DATE=2026-05-18   # 报告日
FROM=$DATE
TO=$DATE

# 1. 付费 ODS（含 DQ）
make sync-ods ARGS="-gid=0 -from=$FROM -to=$TO"

# 2. 免费 ODS（含 DQ）
make sync-ods ARGS="-gid=469519483 -from=$FROM -to=$TO"

# 3. ETL 三层
make sync-dim ARGS="-from=$FROM -to=$TO"
make sync-dwd ARGS="-from=$FROM -to=$TO"
make sync-dws ARGS="-from=$FROM -to=$TO"

# 4. Analytics 单段验证
make analytics-query ARGS="-preset=product-type-compare -from=$DATE -to=$DATE"
make analytics-query ARGS="-preset=team-performance -from=$(date -v-6d +%Y-%m-%d 2>/dev/null || date -d '6 days ago' +%Y-%m-%d) -to=$DATE"
make analytics-query ARGS="-preset=product-health"
```

> **说明**：`-from`/`-to` 目前用于 **ETL 分区范围** 与 **DQ 检查区间**；ODS 拉取为 Sheet 全量 CSV，若 Sheet 行数很大可缩短同步窗口后依赖 CH 分区过滤。

---

## 四、DQ 报告判读

```
[dq] ===== DQ Report [2026-05-18 ~ 2026-05-18] =====
[dq] [PASS] duplicate (date, product_code, source)  no duplicates
[dq] [PASS] negative DAU  ok
[dq] [PASS] negative recharge_total  ok
[dq] [PASS] outlier DAU chain-ratio ...  (warn only)
[dq] Overall: ALL PASS
```

| 结果 | 处理 |
|------|------|
| `Overall: ALL PASS` | 继续出日报 |
| `duplicate` / `negative_*` FAIL | 停止发日报，检查 Sheet 源数据或重跑 `sync-ods` |
| `outlier` 有计数 | 仅警告，可备注进日报「异常波动产品」 |

---

## 五、验收清单（端到端）

- [ ] `make daily-report ARGS="-date=YYYY-MM-DD"` 无 fatal 退出
- [ ] DQ `Overall: ALL PASS`（或已知离群警告已记录）
- [ ] ① 输出 2 行 `product_type`（paid / free）
- [ ] ② 有 `team` + `date` 多行
- [ ] ③ 有 `health_score` 降序 Top 产品
- [ ] 可选：`make analytics-query ARGS="-preset=product-detail -from=DATE -to=DATE"` 抽查 Top 充值产品

---

## 六、常见问题

| 现象 | 处理 |
|------|------|
| `relation "meta_metric_dict" does not exist` | `make migrate-seed` |
| ClickHouse `GROUP BY` 报错 | 更新代码后 `go build -o datapilot ./cmd` |
| Docker 未启动 | 见 [cli.md §1](../cli.md#1-环境初始化) |
| Sheet 拉取失败 | 确认公开可读或配置 Google 凭证；`make test-gsheet-public url="..."` |

---

## 七、日报交付模板（复制到 IM / 文档）

```markdown
## 运营日报 YYYY-MM-DD

**数据区间**：{from} ~ {to}  
**DQ**：{PASS / FAIL 摘要}

### 付费 vs 免费
（粘贴 ① product-type-compare 表格）

### 小组表现（近 7 日）
（粘贴 ② team-performance 表格）

### 健康度关注
（粘贴 ③ product-health Top 5~10）

### 备注
- 离群/异常产品：
- 待跟进：
```

---

*文档版本：2026-05-19 · 命令入口：`make daily-report` / `./datapilot --report:daily`*
