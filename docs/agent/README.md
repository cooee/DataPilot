# AI Agent v1.1 / v1.2

基于 **ADS 视图** + **meta_metric_dict** 的 NL→语义层查询编排。

## LLM 抽象（多 Provider）

```go
type LLMProvider interface {
    Name() string
    Chat(ctx context.Context, messages []ChatMessage) (*ChatResponse, error)
}
```

| 实现 | 说明 |
|------|------|
| `StubLLMProvider` | 未配置 LLM，走规则引擎 |
| `GeminiProvider` | `google.golang.org/genai`，`AGENT_LLM_PROVIDER=gemini` |

编排层 `Orchestrator` 通过 `Chat` 要求模型输出 JSON 查询计划，再交给 `AnalyticsService` 执行。

## 环境变量

```bash
# 规则引擎（默认）
AGENT_LLM_PROVIDER=stub

# Gemini
AGENT_LLM_PROVIDER=gemini
GEMINI_API_KEY=your_key
GEMINI_MODEL=gemini-2.0-flash   # 可选，默认 gemini-2.0-flash
```

LLM 失败时自动 **回退规则引擎**（`RulePlanner`）。

## API

### 指标问答

`POST /api/agent/metrics/ask`

```json
{
  "question": "2026-05-18 付费和免费日活充值对比",
  "date": "2026-05-18"
}
```

### 异常产品列举

`POST /api/agent/anomalies`

```json
{
  "question": "昨天有哪些异常产品",
  "date": "2026-05-18"
}
```

响应字段：`intent`、`preset`、`source`（`rule`|`llm`）、`skill_loaded`、`skill_id`、`skill_sha256`、`interpretation`、`answer`、`query`、`data`、`meta`。

## Skill 注入（Gemini + 项目 `skills/`）

Skill 文件位于仓库根目录 **`skills/`**（非 `.cursor/skills` 副本）。`Compiler` 启动时读取 `AGENT_SKILL_PATH`（默认 `skills/datapilot-daily-ops-report/SKILL.md`），在调用 Gemini 编排时追加到 **system prompt**。无需 LangChain 等第三方库。

```bash
AGENT_SKILL_ENABLED=true
AGENT_SKILL_PATH=skills/datapilot-daily-ops-report/SKILL.md
```

**验收 Gemini + Skill 同时生效：**

```bash
./datapilot --agent:llm-ping -format=json
# skill_loaded: true, skill_id: datapilot-daily-ops-report, skill_sha256: <64 hex>

./datapilot --agent:ask -q="付费免费对比" -date=2026-05-18
# JSON: "source": "llm", "skill_loaded": true
```

`source=rule` 表示 LLM 未参与本次编排（失败或未配置）；`skill_loaded=true` 仅表示 Skill 文件已加载。

## CLI 快速验证 LLM（推荐第一步）

```bash
go build -o datapilot ./cmd

# 基础连通性（发一条 Chat，期望 reply 含 pong）
make agent-llm-ping
./datapilot --agent:llm-ping

# JSON 输出
./datapilot --agent:llm-ping -format=json

# 额外测试 NL→Plan 编排（与 agent:ask 同路径）
./datapilot --agent:llm-ping -orch

# 编排失败时打印原因
AGENT_LLM_DEBUG=true ./datapilot --agent:ask -q="付费免费对比" -date=2026-05-18
```

成功示例：

```
[agent:llm-ping] OK  provider=gemini  model=gemini-2.0-flash  latency=800ms
  reply: pong
```

失败时 exit code=1，并给出 `hint`（如模型名 404、Key 无效）。

## CLI 验证 Gemini 问答

1. 在 `.env` 中配置（**每行一个变量，不要在行尾写 `#` 注释**）：

```bash
AGENT_LLM_PROVIDER=gemini
GEMINI_API_KEY=你的密钥
GEMINI_MODEL=gemini-2.0-flash
```

2. 编译并执行（会自动 `LoadEnv`，无需 `export $(grep ...)`）：

```bash
go build -o datapilot ./cmd

./datapilot --agent:ask -q="2026-05-18 付费和免费产品的日活和充值对比" -date=2026-05-18

./datapilot --agent:anomalies -date=2026-05-18
```

3. 成功时 JSON 里 `"source": "llm"`；失败会回退 `"source": "rule"` 并在日志体现。

```bash
# 规则引擎（不调用 Gemini）
AGENT_LLM_PROVIDER=stub ./datapilot --agent:ask -q="付费免费对比" -date=2026-05-18
```

## 目录结构

```
modules/agent/
  provider/
    types.go          # LLMProvider / ChatMessage
    gemini.go         # GeminiProvider
    stub.go
    factory.go        # NewLLMProviderFromEnv
    orchestration.go  # NL→PlanResult（调用 Chat）
    mock.go
  orchestrator/
    rule_planner.go   # 规则回退
    compiler.go       # LLM 优先 + 回退
  service/            # AgentService + AnswerBuilder
```

## 扩展新 Provider

1. 在 `provider/` 新建文件实现 `LLMProvider.Chat`
2. 在 `factory.go` 的 `switch` 中注册
3. 设置 `AGENT_LLM_PROVIDER=<name>`
