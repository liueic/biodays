# biodays

> 实验室科学史上的今天。每天一张卡片，一则发生在该月该日的实验室故事，外加一句极客金句。

biodays 是一个前后端分离的双工程仓库：Go 单体后端提供版本化 REST API，Next.js 前端以**复古 Windows 98 桌面**风格呈现沉浸式阅读与档案浏览体验。当前数据集 104 篇（Q1 + Q4），Q2/Q3 尚在整理；数据缺口日期由 `/api/v1/today` 的随机兜底策略平滑处理。

```
stories_q*.json ──▶ biodays seed ──▶ SQLite (biodays.db)
                                      │
                                      ▼
                       Go + Gin  /api/v1  ──▶  Next.js App Router
                       Swagger · 限流 · CORS      React95 · 像素风 · Win98 桌面
```

---

## 视觉与交互特色

- **Windows 98 复古桌面体验**：经典青色桌面（`#008080`）、Win98 经典渐变活动标题栏（`#000080` → `#1084d0`）、凹凸立体边框、软盘与经典图标系统，配合底部常驻开始菜单任务栏。
- **窗口自由拖拽（Draggable Window）**：桌面端支持按住标题栏在视口内自由拖拽（基于 `@neodrag/react`），并记忆拖拽落点，切页或刷新保持窗口位置；移动端自动适配全屏最大化体验。
- **黄金分割垂直居中**：页面剩余垂直空间采用 `0.382 : 0.618` 黄金分割布局，短页面自然落于视觉上方黄金位置，长页面平滑滚动。
- **双排版字体系统**：
  - **界面与控件**：采用 **Fusion Pixel 12px** 中文比例点阵像素字体，纯正复古软体质感；
  - **正文阅读**：长篇故事与引用采用系统衬线宋体栈（macOS Songti SC、Windows 宋体、Linux Noto Serif CJK）配合平滑抗锯齿与舒适行高（15px / 1.9），兼具像素美学与长时间阅读易读性。
- **“资源管理器”式档案库**：归档页支持按月份下拉/树形筛选、标签过滤与防抖即时搜索，以 Win98 文件列表视图罗列条目，配合状态栏与进度指示条。
- **全键盘与无障碍支持**：提供“跳到正文”Skip-link，各主功能带助记快捷键（`今日(T)`、`归档(A)`、`关于(B)`、`阅读全文(R)` 等）。

---

## 仓库结构

```
biodays/
├── .cnb.yml               CNB 流水线：检查、测试与容器镜像构建
├── .prototools            固定 Go (1.27.3) / Bun (1.4.4) 工具链版本
├── Dockerfile             单容器镜像（Next.js + Go API）
├── docker-compose.yml     app + 可选 seed，只暴露一个端口
├── deploy/launcher.js     容器内进程 supervisor：启动 Go API，就绪后启动 Next
├── server/                Go module（module biodays/server）
│   ├── cmd/biodays/       单二进制入口：serve（默认）/ seed / migrate 子命令
│   ├── internal/          config · model · store · service · handler · middleware · router · seed
│   └── docs/              swag 生成的 Swagger 文档（挂载于 /swagger/index.html）
└── web/                   Next.js 16 + React 19 + Tailwind v4（Bun 管理）
    ├── app/               今日 (/) / 档案 (/archive) / 详情 (/story/[id]) / 关于 (/about) / sitemap / robots
    ├── components/        React95 桌面视窗组件、归档资源管理器、状态栏等
    └── lib/               API client、日期与文本格式化工具
```

---

## 快速开始

### 0. 前置条件

- **Go 1.27.3**
- **Bun 1.4.4**

> 这两个版本同时是安全基线：Go 1.27.3 修复 eMudhra 信任链（CVE-2026-111943）与 x509 问题；Bun 1.4.4 起拒绝 eMudhra 泄露证书链。

两个版本固定于根目录 `.prototools`。推荐使用 [proto](https://moonrepo.dev/proto) 一键就绪环境：

```bash
proto install      # 自动安装 .prototools 中声明的 Go 与 Bun 版本
# 或
proto use
```

未安装 proto 的环境，手动安装对应版本即可。

### 1. 后端：导入数据并启动

数据源目录通过环境变量传入，仓库自身不冗余复制原始数据：

```bash
cd server
export BIODAYS_DATA_DIR=/path/to/data   # 包含 stories_q1.json、stories_q4.json 等的目录

go run ./cmd/biodays seed    # 输出 imported=104 skipped=0 files=2
go run ./cmd/biodays serve   # 启动于 http://localhost:8080（或省略 serve）
```

服务就绪验证：

```bash
curl http://localhost:8080/healthz   # 存活
curl http://localhost:8080/readyz    # 就绪（含数据库连通性）
curl http://localhost:8080/api/v1/today
open http://localhost:8080/swagger/index.html
```

### 2. 前端：启动 Next.js

```bash
cd web
bun install
bun run dev                # 启动于 http://localhost:3000
```

开发模式下 Next.js 通过 `next.config.ts` 的 `rewrites` 将 `/api/*` 请求透明代理至 `BIODAYS_API_URL`（默认 `http://localhost:8080`），浏览器同源无跨域困扰。生产环境则由后端 CORS 白名单放行正式域名。

### 3. 环境变量

#### 后端环境变量

| 变量 | 默认值 | 说明 |
|---|---|---|
| `BIODAYS_PORT` | `8080` | 服务监听端口 |
| `BIODAYS_DB_PATH` | `./biodays.db` | SQLite 数据文件路径 |
| `BIODAYS_DATA_DIR` | 必填（执行 seed 时） | 原始 `stories_*.json` 所在目录 |
| `BIODAYS_CORS_ORIGINS` | `http://localhost:3000` | 跨域放行源，多个以逗号分隔，支持 `*` |
| `BIODAYS_TRUSTED_PROXIES` | 空 | 可信反向代理 IP/CIDR，多个以逗号分隔；仅对这些来源信任 `X-Forwarded-For` |
| `BIODAYS_RATE_LIMIT` | `60` | 单 IP 每分钟限流请求数 |
| `BIODAYS_LOG_LEVEL` | `info` | 日志级别：`debug` / `info` / `warn` / `error` |
| `GIN_MODE` | `release`（镜像内固定） | Gin 运行模式；未设置时 Gin 会退回 `debug` 并打印 `[GIN-debug]` 警告。镜像已固定为 `release`，本地开发想开调试路由日志时显式设为 `debug` |

> 部署在 Nginx/Caddy 等反向代理后时，把代理地址加入 `BIODAYS_TRUSTED_PROXIES`（例如 `127.0.0.1/32,172.18.0.0/16`），限流才会按真实客户端 IP 生效。不要配置 `0.0.0.0/0`，否则 `X-Forwarded-For` 可被伪造。直连部署时保持为空即可。

#### 前端环境变量（`web/.env.local`，参考 `web/.env.example`）

| 变量 | 默认值 | 说明 |
|---|---|---|
| `BIODAYS_API_URL` | `http://localhost:8080` | SSR / ISR 数据抓取与 `/api` 代理目标 |
| `BIODAYS_SITE_URL` | `http://localhost:3000` | canonical / OpenGraph / sitemap 站点根地址 |
| `NEXT_PUBLIC_BIODAYS_DOCS_URL` | `/swagger/index.html` | 前端关于页与任务栏直达的 API 文档链接 |

---

## API 速查

所有业务端点位于 `/api/v1`，成功时直接返回对应资源 JSON；列表结果统一为 `{ data: [...], meta: { total, page, page_size } }` 格式。

| 方法 | 路径 | 说明 | 查询参数 |
|---|---|---|---|
| GET | `/healthz` | 存活检查（不计入限流） | 无 |
| GET | `/readyz` | 就绪检查，含数据库连通性（不计入限流） | 无 |
| GET | `/api/v1/today` | 今日故事 | `tz`（默认 `+08:00`）、`fallback`（`random` 或 `none`） |
| GET | `/api/v1/stories` | 分页列表 | `month`、`day`、`tag`、`q`、`page`、`page_size`（上限 100） |
| GET | `/api/v1/stories/{id}` | 单篇详情（含正文与引用） | 路径参数 `id`（如 `Q1-01`） |
| GET | `/api/v1/stories/random` | 随机一篇 | `exclude`（逗号分隔排除列表，如 `Q1-01,Q4-02`） |
| GET | `/api/v1/tags` | 全部标签及关联篇数 | 无 |
| GET | `/api/v1/meta` | 数据集指标（总篇数、季度/月份覆盖） | 无 |

调用示例：

```bash
# 获取今日故事（默认 Asia/Shanghai）
curl http://localhost:8080/api/v1/today

# 筛选 1 月份故事列表
curl 'http://localhost:8080/api/v1/stories?month=1&page_size=5'

# 排除当前篇目获取随机故事
curl 'http://localhost:8080/api/v1/stories/random?exclude=Q1-01'

# 严格模式获取今日故事（无数据时返回 404 而不随机）
curl 'http://localhost:8080/api/v1/today?fallback=none'
```

统一错误响应：

```json
{
  "error": {
    "code": "STORY_NOT_FOUND",
    "message": "story Q2-99 not found"
  }
}
```

- 常见错误码：`INVALID_PARAM` (400)、`STORY_NOT_FOUND` (404)、`RATE_LIMITED` (429)、`INTERNAL_ERROR` (500)。
- 缓存策略：所有 GET 成功响应默认携带 `Cache-Control: public, max-age=300`；错误响应携带 `no-store`。

---

## 测试与代码质量

```bash
# 后端：集成与单元测试、代码审查
cd server
go test ./...              # 覆盖全部端点、限流、CORS、400/404 分支（基于内存 SQLite）
go vet ./...

# 前端：类型检查与 Biome 检查
cd ../web
bun run typecheck          # tsc --noEmit
bun run lint               # Biome check
bun run format             # Biome format
```

---

## 部署

### 后端（CGO 单二进制 + 单文件 SQLite）

本项目使用 GORM 官方 SQLite 驱动（`gorm.io/driver/sqlite`，底层为 `mattn/go-sqlite3`）。编译需要本机 C 工具链：macOS 安装 Xcode Command Line Tools，Linux 安装 `gcc`。Docker 镜像内已配置好工具链，推荐用容器构建以保证可复现：

```bash
cd server
CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o biodays ./cmd/biodays

# 数据导入并启动（单二进制子命令）
BIODAYS_DATA_DIR=/path/to/data ./biodays seed
BIODAYS_DB_PATH=./biodays.db ./biodays serve

# 或者一步到位：serve 启动时会自动迁移 schema 并在配置了
# BIODAYS_DATA_DIR 时执行幂等 seed，无需先手动 seed
BIODAYS_DATA_DIR=/path/to/data BIODAYS_DB_PATH=./biodays.db ./biodays serve
```

#### 单容器镜像（前端 + Go API）

根目录 `Dockerfile` 会把 Next.js 与 Go API 打进同一个镜像：Next 作为唯一入口监听 3000，内部启动 Go API（`127.0.0.1:8080`），对外只需暴露一个端口。

```bash
docker build -t biodays .
docker volume create biodays-data

docker run -d --name biodays -p 8080:3000 \
  -v biodays-data:/data \
  -v /path/to/data:/seed:ro \
  -e BIODAYS_DATA_DIR=/seed \
  -e BIODAYS_SITE_URL=http://localhost:8080 \
  biodays
```

容器启动时 Go API 会先完成 schema 迁移，并在检测到 `BIODAYS_DATA_DIR` 中的 `stories_*.json` 后执行一次幂等 seed（按 id upsert），随后才开始监听；就绪后 launcher 再启动 Next。打开 `http://localhost:8080` 即可使用。同一个端口下：

- `/`：前端
- `/api/v1/*`：API
- `/swagger/index.html`：Swagger
- `/readyz`：健康检查

#### CNB 自动构建（CNB Docker 制品库）

推送 `main` 或推送 `v*.*.*` 标签时，`.cnb.yml` 会并行跑两条检查流水线，全部通过后再构建 `linux/amd64` 与 `linux/arm64` 双架构镜像推送到 CNB Docker 制品库：

- `go`：`go vet ./...`、`go test -race -count=1 ./...`、`govulncheck`
- `web`：`bun install --frozen-lockfile`、`typecheck`、`lint`

镜像地址为 `docker.cnb.cool/ciallo.cv/biodays`（即 `${CNB_DOCKER_REGISTRY}/${CNB_REPO_SLUG_LOWERCASE}`）：

- `latest`：默认分支最新构建
- `main` / `sha-<短哈希>`：分支构建
- `1.2.3` / `1.2`：Git tag（如 `v1.2.3`）
- PR 只构建校验，不推送

拉取运行（制品库私有化时需先 `docker login docker.cnb.cool -u cnb -p <访问令牌>`，公开则直接拉）：

```bash
docker pull docker.cnb.cool/ciallo.cv/biodays:latest
docker run -d --name biodays -p 8080:3000 \
  -v biodays-data:/data \
  -v /path/to/data:/seed:ro \
  -e BIODAYS_DATA_DIR=/seed \
  -e BIODAYS_SITE_URL=http://localhost:8080 \
  docker.cnb.cool/ciallo.cv/biodays:latest
```

#### Docker Compose（推荐）

仓库根目录的 `docker-compose.yml` 只有一个 `app` 服务，启动时自动导入数据：

```bash
cp .env.example .env
# 编辑 .env：设置 BIODAYS_SITE_URL 与 BIODAYS_DATA_DIR
# 制品库私有化时需要先登录：docker login docker.cnb.cool -u cnb -p <访问令牌>

docker compose pull
docker compose up -d
docker compose logs -f app
curl http://127.0.0.1:8080/readyz
```

compose 内置了 `pull_policy`（默认 `missing`）与容器 `init`，健康检查直接复用镜像自带的 `/readyz`；`BIODAYS_DB_PATH` 在 compose 里固定为 `/data/biodays.db`，与命名卷对齐。

打开 `http://<服务器>:8080/` 即可使用，不需要单独部署前端，也不需要手动执行 seed。数据更新后重启容器即可重新导入：

```bash
docker compose restart app
```

部署要点：

- 对外只有一个端口；`BIODAYS_HTTP_PORT` 可以改成 80 等。
- `BIODAYS_SITE_URL` 应设置为用户实际访问的地址，用于 canonical、sitemap 和 robots。
- `BIODAYS_DATA_DIR` 必须指向包含 `stories_*.json` 的目录（建议绝对路径），并以只读方式挂载；该目录需对容器内 `biodays`（非 root）可读，否则启动时 seed 会跳过。
- SQLite 数据保存在命名卷 `biodays-data`，会自动创建；`/data` 与 `/seed` 是两个独立挂载点。
- 生产环境建议把 `BIODAYS_IMAGE` 固定到版本 tag 或 digest，而不是长期使用 `latest`。
- 如果需要对公网提供 HTTPS，在 8080（或 80）前加 Caddy/Nginx/Cloudflare 即可；镜像内部的两个进程无需再拆开。
- `server/Dockerfile` 仅保留给“前后端分开部署”的场景，默认部署不需要它。

> 为什么不在镜像 build 时导入？故事 JSON 在仓库外，CI 构建上下文拿不到；把 SQLite 写进镜像会变成可变数据层，而且首次挂载 `/data` 卷时镜像内数据库会被卷遮住。启动时 seed 是幂等的，既保留外部数据源，也能在重启时自动更新。

### 前端（可选拆分部署）

默认单容器已经包含 Next.js，不需要单独部署前端。仅当你确实想把前端放到 Vercel 时，才需要另外暴露后端 API：

```bash
cd web
bun install
BIODAYS_API_URL=https://api.example.com \
BIODAYS_SITE_URL=https://biodays.example.com \
NEXT_PUBLIC_BIODAYS_DOCS_URL=/swagger/index.html \
bun run build
bun run start
```

这种情况下使用 `server/Dockerfile` 或另外反代后端 8080 端口，并在部署平台配置上述环境变量。

---

## 数据与架构实现说明

1. **同日多篇规则与缺口兜底**：
   - 当服务端判定某月某日存在多条故事记录时，按 `year ASC` 选取年份最早的一条，确保同一日期的“今日卡片”展示绝对稳定；
   - 当前 Q2/Q3 暂缺，未覆盖日期由 `/api/v1/today` 随机推荐一篇，并在元数据中标记 `meta.is_fallback = true`，前端状态栏会向用户清晰呈现提示；
2. **幂等 Seed 机制**：
   - `biodays seed` 会扫描指定目录下的全部 `stories_*.json` 文件，根据主键 `id` 幂等 upsert；
   - `biodays serve` 启动时自动执行 schema 迁移，并在 `BIODAYS_DATA_DIR` 指向的目录存在 `stories_*.json` 时自动执行同一套幂等 seed，容器无需单独的初始化步骤；
   - 自动建立和更新故事与标签的多对多关联，并主动清理已无任何故事引用的孤儿标签；遇到单条损坏数据记录 warning 日志并跳过，确保整体导入流程稳健不中断；
3. **单容器运行期渲染**：
   - 镜像构建不依赖故事数据；`/`、`/archive`、`/story/[id]` 按请求从同容器内的 Go API 读取 SQLite；
   - Next 反向代理 `/api`、`/swagger`、`/healthz`、`/readyz`，浏览器只访问一个端口；`sitemap.xml`/`robots.txt` 也按运行期 `BIODAYS_SITE_URL` 生成；
4. **CGO SQLite 驱动选型**：
   - 采用 GORM 官方 `gorm.io/driver/sqlite`（底层 `mattn/go-sqlite3`，内置 SQLite amalgamation），取代此前因本机 CGO 工具链故障而使用的纯 Go `glebarez/sqlite`；
   - 优势是官方维护、与 GORM 版本同步、贴近上游 SQLite 行为，并支持 FTS5/JSON1 等扩展能力；代价是构建需要 C 工具链，跨平台交叉编译需准备对应的 C 交叉编译器；
5. **内容来源**：
   - 条目内容均基于科学史专利、期刊论文、诺贝尔奖官方新闻稿及科研机构历史档案整理改写；单篇条目随 API 携带 citation 引用与原始链接。

---

## 技术栈清单

- **后端**：Go 1.27 · Gin v1.12 · GORM · `gorm.io/driver/sqlite` (CGO SQLite) · `caarlos0/env/v11` · `log/slog` · `swaggo/swag` · `stretchr/testify`
- **前端**：Next.js 16 (App Router) · React 19 · TypeScript (strict) · Tailwind CSS v4 · `@react95/core` · `@react95/icons` · `@neodrag/react` · Fusion Pixel Font · TanStack Query · Bun · Biome
