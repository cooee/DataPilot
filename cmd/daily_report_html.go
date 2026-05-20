package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/provider"
	"github.com/Caknoooo/go-gin-clean-starter/modules/agent/skill"
	analyticsSvc "github.com/Caknoooo/go-gin-clean-starter/modules/analytics/service"
	gsheetSvc "github.com/Caknoooo/go-gin-clean-starter/modules/gsheet/service"
	"github.com/Caknoooo/go-gin-clean-starter/modules/report"
	"github.com/Caknoooo/go-gin-clean-starter/pkg/constants"
	"github.com/samber/do"
)

type dailyHTMLArgs struct {
	dateFrom      time.Time
	dateTo        time.Time
	reportOn      string
	outPath       string
	thinkingPath  string
	skipSync      bool
	useLLM        bool
	verbose       bool
	showThinking  bool
}

func runDailyReportHTML(injector *do.Injector) {
	runDailyReportHTMLMode(injector, false)
}

func runDailyReportHTMLLLM(injector *do.Injector) {
	runDailyReportHTMLMode(injector, true)
}

func runDailyReportHTMLMode(injector *do.Injector, defaultLLM bool) {
	cmd := "--report:daily-html"
	if defaultLLM {
		cmd = "--report:daily-html-llm"
	}
	args := parseDailyHTMLArgs(cmd, defaultLLM)

	loader := do.MustInvokeNamed[*gsheetSvc.ODSLoader](injector, constants.ODSLoader)
	etlRunner := do.MustInvokeNamed[*gsheetSvc.ETLRunner](injector, constants.ETLRunner)
	dqChecker := do.MustInvokeNamed[*gsheetSvc.DQChecker](injector, constants.DQChecker)
	analytics := do.MustInvoke[analyticsSvc.AnalyticsService](injector)

	ctx := context.Background()
	tag := "[daily-report-html]"
	if args.useLLM {
		tag = "[daily-report-html-llm]"
	}

	log.Printf("%s sync_range=%s~%s report_date=%s out=%s llm=%v",
		tag, args.dateFrom.Format("2006-01-02"), args.dateTo.Format("2006-01-02"), args.reportOn, args.outPath, args.useLLM)

	dqOK, dqSummary := true, "DQ OK（跳过同步未复检）"
	if !args.skipSync {
		syncODS(ctx, loader, dqChecker, syncArgs{
			spreadsheetID: defaultSpreadsheetID, gid: 0, productType: "paid",
			dateFrom: args.dateFrom, dateTo: args.dateTo,
		})
		syncODS(ctx, loader, dqChecker, syncArgs{
			spreadsheetID: defaultSpreadsheetID, gid: 469519483, productType: "free",
			dateFrom: args.dateFrom, dateTo: args.dateTo,
		})
		if err := etlRunner.RunAll(ctx, gsheetSvc.ETLParams{DateFrom: args.dateFrom, DateTo: args.dateTo}); err != nil {
			log.Fatalf("%s ETL failed: %v", tag, err)
		}
		dqReport, err := dqChecker.Check(ctx, args.dateFrom, args.dateTo)
		if err != nil {
			log.Fatalf("%s DQ check failed: %v", tag, err)
		}
		dqOK, dqSummary = report.DQSummaryFromReport(dqReport)
		if !dqOK {
			log.Printf("%s WARNING: %s", tag, dqSummary)
		}
	}

	payload, err := report.Collect(ctx, analytics, report.CollectInput{
		ReportDate: args.reportOn,
		SyncFrom:   args.dateFrom,
		SyncTo:     args.dateTo,
		DQOK:       dqOK,
		DQSummary:  dqSummary,
	})
	if err != nil {
		log.Fatalf("%s collect: %v", tag, err)
	}

	if args.useLLM {
		skillDoc, err := skill.LoadFromEnv()
		if err != nil {
			log.Fatalf("%s skill load: %v", tag, err)
		}
		if skillDoc == nil {
			log.Fatalf("%s Skill 未找到，请配置 AGENT_SKILL_PATH=%s", tag, skill.DefaultPath())
		}
		llm, err := provider.NewLLMProviderFromEnv(ctx)
		if err != nil {
			log.Fatalf("%s LLM: %v", tag, err)
		}
		if _, ok := llm.(provider.StubLLMProvider); ok {
			log.Fatalf("%s 需要配置 AGENT_LLM_PROVIDER=gemini 与 GEMINI_API_KEY", tag)
		}
		log.Printf("%s skill=%s sha=%s provider=%s model=%s",
			tag, skillDoc.ID, skillDoc.SHA256, llm.Name(), provider.ModelName(llm))

		result, err := report.GenerateHTMLViaLLM(ctx, llm, skillDoc, payload, report.LLMHTMLOptions{
			Verbose:      args.verbose,
			ShowThinking: args.showThinking,
		})
		if err != nil {
			log.Fatalf("%s generate: %v", tag, err)
		}
		if err := os.WriteFile(args.outPath, []byte(result.HTML), 0o644); err != nil {
			log.Fatalf("%s write html: %v", tag, err)
		}
		if args.thinkingPath != "" && result.Thinking != "" {
			if err := os.WriteFile(args.thinkingPath, []byte(result.Thinking), 0o644); err != nil {
				log.Fatalf("%s write thinking: %v", tag, err)
			}
		}
		abs, _ := filepath.Abs(args.outPath)
		log.Printf("%s DONE (LLM HTML) → %s", tag, abs)
		log.Printf("%s badge: provider=%s model=%s skill=%s", tag, result.Meta.Provider, result.Meta.Model, result.Meta.SkillID)
		if args.thinkingPath != "" && result.Thinking != "" {
			log.Printf("%s thinking saved → %s", tag, args.thinkingPath)
		}
		return
	}

	if err := report.WriteDailyHTML(args.outPath, payload); err != nil {
		log.Fatalf("%s write template: %v", tag, err)
	}
	abs, _ := filepath.Abs(args.outPath)
	log.Printf("%s DONE (template) → %s", tag, abs)
}

func parseDailyHTMLArgs(command string, useLLM bool) dailyHTMLArgs {
	fs := flag.NewFlagSet(command, flag.ExitOnError)
	from := fs.String("from", "", "同步起始日期 YYYY-MM-DD（默认：报告日）")
	to := fs.String("to", "", "同步结束日期 YYYY-MM-DD（默认：报告日）")
	reportDate := fs.String("date", "", "报告日 YYYY-MM-DD（默认昨天）")
	out := fs.String("out", "", "输出 HTML 路径")
	skipSync := fs.Bool("skip-sync", false, "跳过 Sheet 同步与 ETL，仅拉数生成 HTML")
	llmFlag := fs.Bool("llm", useLLM, "使用大模型直接生成 HTML（需 Gemini + Skill）")
	verbose := fs.Bool("verbose", useLLM || report.LLMVerboseFromEnv(), "流水线日志（stderr）")
	showThinking := fs.Bool("thinking", useLLM, "先流式输出数据分析思考过程（stdout）")
	noThinking := fs.Bool("no-thinking", false, "关闭思考过程，直接生成 HTML")
	thinkingOut := fs.String("thinking-out", "", "思考过程 Markdown 保存路径（可选）")

	rest := argsAfterCommand(command)
	_ = fs.Parse(rest)

	useLLMMode := useLLM || *llmFlag

	now := time.Now().Truncate(24 * time.Hour)
	reportOn := now.AddDate(0, 0, -1).Format("2006-01-02")
	if *reportDate != "" {
		reportOn = *reportDate
	}
	if _, err := time.Parse("2006-01-02", reportOn); err != nil {
		log.Fatalf("invalid -date: %v", err)
	}

	dateFrom, dateTo := reportOn, reportOn
	if *from != "" {
		dateFrom = *from
	}
	if *to != "" {
		dateTo = *to
	}
	if *from == "" && *to != "" {
		dateFrom = dateTo
	}
	if *to == "" && *from != "" {
		dateTo = dateFrom
	}

	df, err := time.Parse("2006-01-02", dateFrom)
	if err != nil {
		log.Fatalf("invalid -from: %v", err)
	}
	dt, err := time.Parse("2006-01-02", dateTo)
	if err != nil {
		log.Fatalf("invalid -to: %v", err)
	}
	if df.After(dt) {
		log.Fatal("-from must be <= -to")
	}

	outPath := *out
	if outPath == "" {
		if useLLMMode {
			outPath = filepath.Join("reports", "daily-"+reportOn+"-llm.html")
		} else {
			outPath = report.DefaultOutputPath(reportOn)
		}
	}
	if dir := filepath.Dir(outPath); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	thinkPath := *thinkingOut
	if thinkPath == "" && useLLMMode && *showThinking && !*noThinking {
		thinkPath = filepath.Join("reports", fmt.Sprintf("daily-%s-thinking.md", reportOn))
	}

	return dailyHTMLArgs{
		dateFrom:     df,
		dateTo:       dt,
		reportOn:     reportOn,
		outPath:      outPath,
		thinkingPath: thinkPath,
		skipSync:     *skipSync,
		useLLM:       useLLMMode,
		verbose:      *verbose,
		showThinking: *showThinking && !*noThinking,
	}
}
