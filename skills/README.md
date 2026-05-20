# DataPilot Skills

项目内 **Agent Skill** 的 canonical 目录。Go Agent（Gemini）通过 `AGENT_SKILL_PATH` 加载并注入 system prompt。

| Skill | 路径 | 说明 |
|-------|------|------|
| 每日运营报表 | `datapilot-daily-ops-report/SKILL.md` | HTML 日报、异常解读、CLI 工作流（含站点产品查询） |

LLM 数据获取速查（不注入 Skill 时也可读）：`docs/agent/ai-cli-reference.md`。

## 环境变量

```bash
AGENT_SKILL_ENABLED=true   # false 关闭注入
AGENT_SKILL_PATH=skills/datapilot-daily-ops-report/SKILL.md
```

## 验证 Gemini + Skill

```bash
./datapilot --agent:llm-ping -format=json
./datapilot --report:daily-html-llm -skip-sync -date=2026-05-18
# 打开 reports/daily-*-llm.html，检查 Tab 切换与底栏「AI 生成 · Model: gemini-...」
```

Cursor IDE 可通过 `.cursor/skills/` 下的符号链接发现同一 Skill。
