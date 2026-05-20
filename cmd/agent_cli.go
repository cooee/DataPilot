package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	agentdto "github.com/Caknoooo/go-gin-clean-starter/modules/agent/dto"
	agentSvc "github.com/Caknoooo/go-gin-clean-starter/modules/agent/service"
	"github.com/samber/do"
)

func runAgentAsk(injector *do.Injector) {
	fs := flag.NewFlagSet("--agent:ask", flag.ExitOnError)
	question := fs.String("q", "", "自然语言问题")
	date := fs.String("date", "", "报告日 YYYY-MM-DD")
	preset := fs.String("preset", "", "直接指定 preset")
	_ = fs.Parse(argsAfterCommand("--agent:ask"))

	svc := do.MustInvoke[agentSvc.AgentService](injector)
	resp, err := svc.AskMetrics(context.Background(), &agentdto.AskRequest{
		Question: *question,
		Date:     *date,
		Preset:   *preset,
	})
	if err != nil {
		log.Fatalf("[agent:ask] %v", err)
	}
	printAgentResponse(resp)
}

func runAgentAnomalies(injector *do.Injector) {
	fs := flag.NewFlagSet("--agent:anomalies", flag.ExitOnError)
	question := fs.String("q", "", "自然语言问题（可选）")
	date := fs.String("date", "", "报告日 YYYY-MM-DD")
	_ = fs.Parse(argsAfterCommand("--agent:anomalies"))

	svc := do.MustInvoke[agentSvc.AgentService](injector)
	resp, err := svc.ListAnomalies(context.Background(), &agentdto.AnomalyListRequest{
		Question: *question,
		Date:     *date,
	})
	if err != nil {
		log.Fatalf("[agent:anomalies] %v", err)
	}
	printAgentResponse(resp)
}

func printAgentResponse(resp *agentdto.AgentResponse) {
	fmt.Println(resp.Answer)
	fmt.Println("---")
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp)
}
