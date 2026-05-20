package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/provider"
)

func runAgentLLMPing() {
	fs := flag.NewFlagSet("--agent:llm-ping", flag.ExitOnError)
	orch := fs.Bool("orch", false, "额外测试 NL→Plan 编排")
	question := fs.String("q", "", "编排测试用问题")
	format := fs.String("format", "text", "输出: text | json")
	timeout := fs.Int("timeout", 30, "超时秒数")
	_ = fs.Parse(argsAfterCommand("--agent:llm-ping"))

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeout)*time.Second)
	defer cancel()

	res := provider.Ping(ctx, provider.PingOptions{
		TestOrchestration: *orch,
		SampleQuestion:    *question,
	})

	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(res); err != nil {
			log.Fatal(err)
		}
	default:
		printPingText(res)
	}

	if !res.OK {
		os.Exit(1)
	}
}

func printPingText(res provider.PingResult) {
	status := "FAIL"
	if res.OK {
		status = "OK"
	}
	fmt.Printf("[agent:llm-ping] %s  provider=%s", status, res.Provider)
	if res.Model != "" {
		fmt.Printf("  model=%s", res.Model)
	}
	fmt.Printf("  latency=%dms\n", res.LatencyMs)
	if res.ReplyPreview != "" {
		fmt.Printf("  reply: %s\n", res.ReplyPreview)
	}
	if res.Usage.TotalTokens > 0 {
		fmt.Printf("  tokens: prompt=%d completion=%d total=%d\n",
			res.Usage.PromptTokens, res.Usage.CompletionTokens, res.Usage.TotalTokens)
	}
	if res.Error != "" {
		fmt.Printf("  error: %s\n", res.Error)
	}
	if res.Hint != "" {
		fmt.Printf("  hint: %s\n", res.Hint)
	}
	if res.SkillLoaded {
		fmt.Printf("  skill: id=%s sha=%s\n", res.SkillID, res.SkillSHA256)
	} else {
		fmt.Println("  skill: not loaded (check AGENT_SKILL_PATH / skills/)")
	}
	if res.OK {
		fmt.Println("  下一步: ./datapilot --agent:ask -q=\"你的问题\" -date=2026-05-18")
		fmt.Println("  成功时 JSON: source=llm, skill_loaded=true")
	}
}
