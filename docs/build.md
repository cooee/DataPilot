1. 编译
推荐（包含 analytics、sync、ch-migrate 等所有 CLI）：

cd .../DataPilot
go build -o datapilot ./cmd
会得到可执行文件 ./datapilot。

现有 make build 用的是 go build -o main cmd/main.go，只会编译单个文件，analytics/sync 等子命令会缺失。请用上面的 ./cmd 方式。

2. 用二进制调用 CLI
在项目根目录执行（.env 在这里）：

# 查看语义层
./datapilot --analytics:schema
# 查询
./datapilot --analytics:query -preset=product-type-compare -from=2026-05-18 -to=2026-05-18
# JSON 输出
./datapilot --analytics:query -preset=product-health -format=json
# 其他已有 CLI
./datapilot --ch:migrate
./datapilot --sync:ods -gid=0 -from=2026-03-01 -to=2026-05-19
3. 放到 PATH（可选）
# 安装到 GOPATH/bin
go install ./cmd
# 之后任意目录（仍需能读到 .env，或在项目根执行）
datapilot --analytics:schema
go install 后若要在别的目录跑，需把 .env 拷过去，或导出 CLICKHOUSE_*、DB_* 等环境变量。

4. 交叉编译（可选，例如 Linux 服务器）
GOOS=linux GOARCH=amd64 go build -o datapilot-linux ./cmd
依赖：ClickHouse / PostgreSQL 已启动，.env 配置正确；需要指标中文语义时先 make migrate-seed（或用二进制：./datapilot --migrate:run --seed）。

