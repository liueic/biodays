# biodays 本地开发与调试
#
# 快速上手:
#   make dev               一键启动: Go API(:8080) + Next.js(:3000)，Ctrl-C 同时退出
#   make admin-password    创建/重置管理员账号（默认用户名 admin，写入 server/feedback.db）
#
# 管理后台入口: http://localhost:3000/admin/login （dev 已默认开启 BIODAYS_FEEDBACK_ENABLED）
#   /admin/feedback 匿名反馈收件箱 · /admin/mail 邮件状态 · /admin/stories 故事编辑
#   /admin/settings 站点设置（SMTP/频控/反滥用，保存即热生效）
#
# 提示: make dev 会把两个进程的日志交错打印；想分开看就在两个终端分别跑
#   make backend 与 make frontend。若退出后端口仍被占用: lsof -ti:8080 | xargs kill
#
# 其它目标: make help 查看。

# ---- 可按需覆盖，例如: make dev BIODAYS_FEEDBACK_ENABLED=false ----
# 注意: 这些赋值行不要写行尾注释，make 会把注释前的空格一并算进变量值。
# BIODAYS_DATA_DIR 相对 server/，serve 启动时按指纹对 data/ 语料做增量导入
# GIN_MODE=debug 打印 Gin 路由与请求日志（生产镜像内固定为 release）
export BIODAYS_PORT ?= 8080
export BIODAYS_DATA_DIR ?= ../data
export BIODAYS_SITE_URL ?= http://localhost:3000
export BIODAYS_FEEDBACK_ENABLED ?= true
export BIODAYS_LOG_LEVEL ?= debug
export GIN_MODE ?= debug

ADMIN_USERNAME ?= admin

SERVER_DIR := server
WEB_DIR    := web

.DEFAULT_GOAL := dev
.PHONY: dev backend frontend seed admin-password ready test build help

dev:
	$(MAKE) --no-print-directory -j2 backend frontend

backend:
	cd $(SERVER_DIR) && go run ./cmd/biodays serve

frontend:
	@[ -d "$(WEB_DIR)/node_modules" ] || (cd $(WEB_DIR) && bun install)
	cd $(WEB_DIR) && BIODAYS_API_URL=http://127.0.0.1:$(BIODAYS_PORT) bun run dev

seed:
	cd $(SERVER_DIR) && go run ./cmd/biodays seed --strict

admin-password:
	cd $(SERVER_DIR) && go run ./cmd/biodays admin set-password --username $(ADMIN_USERNAME)

ready:
	curl -fsS http://127.0.0.1:$(BIODAYS_PORT)/healthz && echo
	curl -fsS http://127.0.0.1:$(BIODAYS_PORT)/readyz && echo

test:
	cd $(SERVER_DIR) && go test ./...
	cd $(WEB_DIR) && bun run typecheck && bun run lint

build:
	cd $(SERVER_DIR) && CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o biodays ./cmd/biodays

help:
	@echo "make dev              一键启动 Go API(:8080) + Next.js(:3000)，Ctrl-C 同时退出"
	@echo "make backend          只启动 Go API"
	@echo "make frontend         只启动 Next.js dev"
	@echo "make seed             严格模式导入 data/ 语料（serve 启动时也会自动增量导入）"
	@echo "make admin-password   创建/重置管理员账号（用户名 \$$ADMIN_USERNAME，存 server/feedback.db）"
	@echo "make ready            健康检查 healthz/readyz"
	@echo "make test             后端 go test + 前端 typecheck/lint"
	@echo "make build            编译后端单二进制 server/biodays"
