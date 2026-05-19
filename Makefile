# Import .env file
ifneq (,$(wildcard ./.env))
		include .env
		export $(shell sed 's/=.*//' .env)
endif

# Variables
CONTAINER_NAME=${APP_NAME}-app
POSTGRES_CONTAINER_NAME=${APP_NAME}-db

# Commands
dep: 
	go mod tidy

run: 
	go run cmd/main.go

build: 
	go build -o main cmd/main.go

run-build: build
	./main

test:
	go test -v ./tests

test-auth:
	go test -v ./modules/auth/tests/...

test-user:
	go test -v ./modules/user/tests/...

test-all:
	go test -v ./modules/.../tests/...

test-coverage:
	go test -v -coverprofile=coverage.out ./modules/.../tests/...
	go tool cover -html=coverage.out

module:
	@if [ -z "$(name)" ]; then echo "Usage: make module name=<module_name>"; exit 1; fi
	@./create_module.sh $(name)

# Commands (without docker)
migrate:
	go run cmd/main.go --migrate:run

migrate-rollback:
	go run cmd/main.go --migrate:rollback

migrate-rollback-batch:
	@if [ -z "$(batch)" ]; then echo "Usage: make migrate-rollback-batch batch=<batch_number>"; exit 1; fi
	go run cmd/main.go --migrate:rollback $(batch)

migrate-rollback-all:
	go run cmd/main.go --migrate:rollback:all

migrate-status:
	go run cmd/main.go --migrate:status

migrate-create:
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-create name=<migration_name>"; exit 1; fi
	go run cmd/main.go --migrate:create:$(name)

seed: 
	go run cmd/main.go --seed

# 测试入口：通过 Google API 读取 Sheet（需要 service account 凭证）
# 用法：make test-gsheet [url=<spreadsheet-url-or-id>]
# 注意：必须用 `go run ./cmd` 而不是 `go run cmd/main.go`，否则同包下的其他 .go 文件不会被编译
test-gsheet:
	@if [ -z "$(url)" ]; then \
		go run ./cmd --test:gsheet; \
	else \
		go run ./cmd --test:gsheet $(url); \
	fi

# 测试入口：直接读公开 Sheet 的 CSV 导出（零凭证，只适用于「知道链接的任何人」可查看的文档）
# 用法：make test-gsheet-public [url=<spreadsheet-url-or-id>]
test-gsheet-public:
	@if [ -z "$(url)" ]; then \
		go run ./cmd --test:gsheet:public; \
	else \
		go run ./cmd --test:gsheet:public $(url); \
	fi

# ClickHouse migrations（建表/视图）
ch-migrate:
	go run ./cmd --ch:migrate

# GSheet 同步命令
# 可选参数（通过 ARGS 传入，例如 ARGS="-id=1sJZB... -from=2026-03-01 -to=2026-05-19 -gid=0"）
#   -id    spreadsheet ID 或完整 URL（默认使用代码中的 defaultSpreadsheetID）
#   -from  起始日期 YYYY-MM-DD（默认 90 天前）
#   -to    结束日期 YYYY-MM-DD（默认今天）
#   -gid   sheet tab gid（默认 0）
sync-ods:
	go run ./cmd --sync:ods $(ARGS)

sync-dim:
	go run ./cmd --sync:dim $(ARGS)

sync-dwd:
	go run ./cmd --sync:dwd $(ARGS)

sync-dws:
	go run ./cmd --sync:dws $(ARGS)

sync-all:
	go run ./cmd --sync:all $(ARGS)

migrate-seed: 
	go run cmd/main.go --migrate:run --seed

# Postgres commands
container-postgres:
	docker exec -it ${POSTGRES_CONTAINER_NAME} /bin/sh

create-db:
	docker exec -it ${POSTGRES_CONTAINER_NAME} /bin/sh -c "createdb --username=${DB_USER} --owner=${DB_USER} ${DB_NAME}"

init-uuid:
	docker exec -it ${POSTGRES_CONTAINER_NAME} /bin/sh -c "psql -U ${DB_USER} -d ${DB_NAME} -c 'CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";'"

# Docker commands
init-docker:
	docker compose up -d --build

up: 
	docker-compose up -d

down:
	docker-compose down

logs:
	docker-compose logs -f

container-go:
	docker exec -it ${CONTAINER_NAME} /bin/sh

migrate-docker:
	docker exec -it ${CONTAINER_NAME} /bin/sh -c "go run cmd/main.go --migrate:run"

migrate-rollback-docker:
	docker exec -it ${CONTAINER_NAME} /bin/sh -c "go run cmd/main.go --migrate:rollback"

migrate-rollback-batch-docker:
	@if [ -z "$(batch)" ]; then echo "Usage: make migrate-rollback-batch-docker batch=<batch_number>"; exit 1; fi
	docker exec -it ${CONTAINER_NAME} /bin/sh -c "go run cmd/main.go --migrate:rollback $(batch)"

migrate-rollback-all-docker:
	docker exec -it ${CONTAINER_NAME} /bin/sh -c "go run cmd/main.go --migrate:rollback:all"

migrate-status-docker:
	docker exec -it ${CONTAINER_NAME} /bin/sh -c "go run cmd/main.go --migrate:status"

migrate-create-docker:
	@if [ -z "$(name)" ]; then echo "Usage: make migrate-create-docker name=<migration_name>"; exit 1; fi
	docker exec -it ${CONTAINER_NAME} /bin/sh -c "go run cmd/main.go --migrate:create:$(name)"

seed-docker: 
	docker exec -it ${CONTAINER_NAME} /bin/sh -c "go run cmd/main.go --seed"

migrate-seed-docker: 
	docker exec -it ${CONTAINER_NAME} /bin/sh -c "go run cmd/main.go --migrate:run --seed"

go-tidy-docker:
	docker exec -it ${CONTAINER_NAME} /bin/sh -c "go mod tidy"