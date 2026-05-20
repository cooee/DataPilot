package report

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

const dailyHTMLTmpl = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>DataPilot 运营分析日报 · {{.ReportDate}}</title>
<style>
:root{--bg:#0b0f14;--card:#151c27;--border:#2a3548;--text:#e8edf4;--muted:#8b9cb3;--accent:#3b82f6;--paid:#10b981;--free:#8b5cf6;--warn:#f59e0b;--danger:#ef4444;--ok:#22c55e}
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:"PingFang SC","SF Pro Display",system-ui,sans-serif;background:var(--bg);color:var(--text);line-height:1.55;padding:1.5rem}
.wrap{max-width:1280px;margin:0 auto}
header{border-bottom:1px solid var(--border);padding-bottom:1.25rem;margin-bottom:1.25rem;display:flex;flex-wrap:wrap;justify-content:space-between;gap:1rem}
h1{font-size:1.6rem;font-weight:700}
.sub{color:var(--muted);font-size:.88rem;margin-top:.35rem}
.badge{display:inline-block;padding:.3rem .7rem;border-radius:999px;font-size:.75rem;font-weight:600}
.badge-ok{background:rgba(34,197,94,.15);color:var(--ok)}
.badge-warn{background:rgba(245,158,11,.15);color:var(--warn)}
.tabs{display:flex;gap:.5rem;margin-bottom:1.25rem}
.tab{flex:1;max-width:220px;padding:.85rem 1rem;border:1px solid var(--border);border-radius:10px;background:var(--card);color:var(--muted);cursor:pointer;font-weight:600;font-size:.95rem;transition:.15s}
.tab.active-paid{border-color:var(--paid);color:var(--paid);box-shadow:0 0 0 1px var(--paid)}
.tab.active-free{border-color:var(--free);color:var(--free);box-shadow:0 0 0 1px var(--free)}
.panel{display:none}
.panel.active{display:block}
.card{background:var(--card);border:1px solid var(--border);border-radius:12px;padding:1.15rem 1.25rem;margin-bottom:1rem}
.card h2{font-size:.78rem;text-transform:uppercase;letter-spacing:.06em;color:var(--muted);margin-bottom:.85rem;font-weight:600}
.focus{font-size:.92rem;color:var(--muted);margin-bottom:1rem;padding:.75rem 1rem;background:rgba(59,130,246,.06);border-radius:8px;border-left:3px solid var(--accent)}
.risk{display:inline-block;font-size:.72rem;padding:.2rem .55rem;border-radius:6px;margin-left:.5rem}
.risk-low{background:rgba(34,197,94,.15);color:var(--ok)}
.risk-medium{background:rgba(245,158,11,.15);color:var(--warn)}
.risk-high{background:rgba(239,68,68,.15);color:var(--danger)}
.kpi-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:.75rem}
.kpi{background:rgba(0,0,0,.2);border-radius:8px;padding:.75rem}
.kpi .v{font-size:1.35rem;font-weight:700}
.kpi .l{font-size:.72rem;color:var(--muted);margin-top:.2rem}
.kpi .d{font-size:.7rem;margin-top:.25rem}
.up{color:var(--ok)}.down{color:var(--danger)}
.notes li{margin:.5rem 0 .5rem 1rem;font-size:.88rem}
table{width:100%;border-collapse:collapse;font-size:.82rem}
th,td{padding:.5rem .4rem;border-bottom:1px solid var(--border);text-align:left}
th{color:var(--muted);font-weight:500;font-size:.72rem}
tr.anomaly td{background:rgba(239,68,68,.06)}
.forecast-group{margin-bottom:1rem}
.forecast-group h3{font-size:.8rem;color:var(--muted);margin-bottom:.4rem}
.global-insights li{padding:.5rem .75rem;margin-bottom:.4rem;background:rgba(59,130,246,.08);border-radius:6px;font-size:.85rem;list-style:none}
footer{margin-top:2rem;text-align:center;color:var(--muted);font-size:.72rem}
</style>
</head>
<body>
<div class="wrap">
<header>
  <div>
    <h1>DataPilot 运营分析日报</h1>
    <p class="sub">报告日 {{.ReportDate}} · 同步 {{.SyncFrom}} ~ {{.SyncTo}} · {{.GeneratedAt}}</p>
    <p class="sub">数据分析师视图 · 付费/免费分轨预测</p>
  </div>
  <div>
    {{if .DQOK}}<span class="badge badge-ok">DQ 通过</span>{{else}}<span class="badge badge-warn">DQ 需关注</span>{{end}}
    <p class="sub" style="margin-top:.4rem">{{.DQSummary}}</p>
  </div>
</header>

{{if .Insights}}
<ul class="global-insights card">{{range .Insights}}<li>{{.}}</li>{{end}}</ul>
{{end}}

<div class="tabs">
  <button type="button" class="tab active-paid" id="tab-paid" onclick="switchTab('paid')">💰 付费产品</button>
  <button type="button" class="tab" id="tab-free" onclick="switchTab('free')">📱 免费产品</button>
</div>

{{range $seg := .Segments}}
<div id="panel-{{$seg.Key}}" class="panel {{if eq $seg.Key "paid"}}active{{end}}">
  <p class="focus">{{.Focus}}</p>

  <div class="card">
    <h2>{{.Label}} · 报告日快照
      <span class="risk risk-{{.RiskLevel}}">{{riskLabel .RiskLevel}}</span>
    </h2>
    <div class="kpi-grid">
      {{if eq .Key "paid"}}
      <div class="kpi"><div class="v">{{fmt0 .Snapshot.NewUsers}}</div><div class="l">新增用户</div></div>
      <div class="kpi"><div class="v">{{fmt0 .Snapshot.NewPaying}}</div><div class="l">新增付费用户</div></div>
      <div class="kpi"><div class="v">{{fmt2 .Snapshot.Recharge}}</div><div class="l">充值金额</div><div class="l {{wowClass .Snapshot.RechargeWoW}}">环比 {{fmtPct .Snapshot.RechargeWoW}}</div></div>
      <div class="kpi"><div class="v">{{fmt2 .Snapshot.ARPPU}}</div><div class="l">ARPPU</div></div>
      <div class="kpi"><div class="v">{{fmt0 .Snapshot.DAU}}</div><div class="l">日活 DAU</div></div>
      {{else}}
      <div class="kpi"><div class="v">{{fmt0 .Snapshot.DAU}}</div><div class="l">日活 DAU</div><div class="l {{wowClass .Snapshot.DAUWoW}}">环比 {{fmtPct .Snapshot.DAUWoW}}</div></div>
      <div class="kpi"><div class="v">{{fmtPctRatio .Snapshot.RetentionD7}}</div><div class="l">7日留存</div><div class="l {{wowClass .Snapshot.RetainWoW}}">环比 {{fmtPct .Snapshot.RetainWoW}}</div></div>
      <div class="kpi"><div class="v">{{fmt0 .Snapshot.NewUsers}}</div><div class="l">新增用户</div></div>
      <div class="kpi"><div class="v">{{fmt2 .Snapshot.Recharge}}</div><div class="l">充值（如有）</div></div>
      {{end}}
    </div>
  </div>

  <div class="card">
    <h2>分析师结论</h2>
    <ul class="notes">{{range .AnalystNotes}}<li>{{.}}</li>{{end}}</ul>
  </div>

  <div class="card">
    <h2>未来 7 日预测模型</h2>
    {{range forecastGroups .Forecasts}}
    <div class="forecast-group">
      <h3>{{.Label}} · {{.Method}}</h3>
      <table>
        <thead><tr><th>日期</th><th>预测值</th><th>方法</th></tr></thead>
        <tbody>{{range .Rows}}<tr><td>{{.Date}}</td><td>{{.ValueStr}}</td><td>{{.MethodDetail}}</td></tr>{{end}}</tbody>
      </table>
    </div>
    {{end}}
    <p class="sub" style="margin-top:.5rem;font-size:.75rem">模型：近14日线性回归；样本&lt;2天为 carry_forward。付费侧重变现指标，免费侧重规模与留存。</p>
  </div>

  <div class="card">
    <h2>近 7 日趋势</h2>
    <table>
      <thead><tr>
        <th>日期</th>
        {{if eq .Key "paid"}}<th>新增</th><th>新增付费</th><th>充值</th><th>ARPPU</th>
        {{else}}<th>DAU</th><th>7日留存</th><th>新增</th>{{end}}
      </tr></thead>
      <tbody>
      {{range .TrendRecent}}
      <tr>
        <td>{{.Date}}</td>
        {{if eq $seg.Key "paid"}}
        <td>{{fmt0 .NewUsers}}</td><td>{{fmt0 .NewPaying}}</td><td>{{fmt2 .Recharge}}</td><td>{{fmt2 .ARPPU}}</td>
        {{else}}
        <td>{{fmt0 .DAU}}</td><td>{{fmtPctRatio .RetentionD7}}</td><td>{{fmt0 .NewUsers}}</td>
        {{end}}
      </tr>
      {{end}}
      </tbody>
    </table>
  </div>

  <div class="card">
    <h2>指标异常</h2>
    {{if .Anomalies}}
    <table>
      <thead><tr><th>产品</th><th>指标</th><th>值</th><th>Z</th><th>方向</th></tr></thead>
      <tbody>{{range .Anomalies}}
      <tr class="anomaly">
        <td>{{.ProductName}}</td><td>{{.Metric}}</td>
        <td>{{fmt2 .Value}}</td><td>{{fmt2 .ZScore}}</td><td>{{.Direction}}</td>
      </tr>{{end}}</tbody>
    </table>
    {{else}}<p class="sub">无异常记录</p>{{end}}
  </div>

  <div class="card">
    <h2>Top 产品（{{if eq .Key "paid"}}按充值{{else}}按日活{{end}}）</h2>
    <table>
      <thead><tr><th>产品</th><th>小组</th>
        {{if eq .Key "paid"}}<th>充值</th><th>新增付费</th><th>ARPPU</th>
        {{else}}<th>DAU</th><th>7日留存</th><th>新增</th>{{end}}
      </tr></thead>
      <tbody>{{range .TopProducts}}
      <tr>
        <td>{{.ProductName}}</td><td>{{.Team}}</td>
        {{if eq $seg.Key "paid"}}
        <td>{{fmt2 .Recharge}}</td><td>{{fmt0 .NewPaying}}</td><td>{{fmt2 .ARPPU}}</td>
        {{else}}
        <td>{{fmt0 .DAU}}</td><td>{{fmtPctRatio .RetentionD7}}</td><td>{{fmt0 .NewUsers}}</td>
        {{end}}
      </tr>{{end}}</tbody>
    </table>
  </div>
</div>
{{end}}

<footer>DataPilot · datapilot --report:daily-html · 付费/免费分轨分析</footer>
</div>
<script>
function switchTab(key){
  document.querySelectorAll('.panel').forEach(p=>p.classList.remove('active'));
  document.getElementById('panel-'+key).classList.add('active');
  const paid=document.getElementById('tab-paid'), free=document.getElementById('tab-free');
  paid.classList.remove('active-paid','active-free');
  free.classList.remove('active-paid','active-free');
  if(key==='paid'){paid.classList.add('active-paid');}else{free.classList.add('active-free');}
}
</script>
</body>
</html>`

type forecastGroupView struct {
	Label  string
	Method string
	Rows   []forecastRowView
}

type forecastRowView struct {
	Date         string
	ValueStr     string
	MethodDetail string
}

// WriteDailyHTML 将报表写入 path。
func WriteDailyHTML(path string, p *DailyReportPayload) error {
	tmpl, err := template.New("daily").Funcs(template.FuncMap{
		"eq":            func(a, b string) bool { return a == b },
		"fmt0":          func(v float64) string { return fmt.Sprintf("%.0f", v) },
		"fmt2":          func(v float64) string { return fmt.Sprintf("%.2f", v) },
		"fmtPct":        func(v float64) string { return fmt.Sprintf("%+.1f%%", v) },
		"fmtPctRatio":   func(v float64) string { return fmt.Sprintf("%.1f%%", v*100) },
		"wowClass":      wowClass,
		"riskLabel":     riskLabel,
		"forecastGroups": forecastGroupsForTemplate,
	}).Parse(dailyHTMLTmpl)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, p); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

func DefaultOutputPath(reportDate string) string {
	return filepath.Join("reports", fmt.Sprintf("daily-%s.html", reportDate))
}

func wowClass(pct float64) string {
	if pct > 0 {
		return "up"
	}
	if pct < 0 {
		return "down"
	}
	return ""
}

func riskLabel(level string) string {
	switch level {
	case "high":
		return "风险·高"
	case "medium":
		return "风险·中"
	default:
		return "风险·低"
	}
}

func forecastGroupsForTemplate(fc []MetricForecast) []forecastGroupView {
	byMetric := map[string][]MetricForecast{}
	order := []string{}
	for _, row := range fc {
		if _, ok := byMetric[row.MetricKey]; !ok {
			order = append(order, row.MetricKey)
		}
		byMetric[row.MetricKey] = append(byMetric[row.MetricKey], row)
	}
	var groups []forecastGroupView
	for _, key := range order {
		rows := byMetric[key]
		if len(rows) == 0 {
			continue
		}
		g := forecastGroupView{Label: rows[0].MetricLabel, Method: rows[0].MethodDetail}
		for _, r := range rows {
			valStr := fmt.Sprintf("%.2f", r.Value)
			if r.MetricKey == "dau" || r.MetricKey == "new_user_total_cnt" || r.MetricKey == "new_paying_user_cnt" {
				valStr = fmt.Sprintf("%.0f", r.Value)
			}
			if r.MetricKey == "retention_d7_ratio" {
				valStr = fmt.Sprintf("%.1f%%", r.Value*100)
			}
			if r.MetricKey == "recharge_total_amt" {
				valStr = "¥" + valStr
			}
			g.Rows = append(g.Rows, forecastRowView{Date: r.Date, ValueStr: valStr, MethodDetail: r.MethodDetail})
		}
		groups = append(groups, g)
	}
	return groups
}
