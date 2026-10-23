# biodays

> 实验室科学史上的今天。每天一张卡片，一则发生在该月该日的实验室故事，外加一句极客金句。

biodays 是一个前后端分离的双工程仓库：Go 单体后端提供版本化 REST API，Next.js 前端以**复古 Windows 98 桌面**风格呈现沉浸式阅读与档案浏览体验。当前数据集 408 篇（季度故事 208 篇 + 生物学主题故事 200 篇），原始 JSON 位于仓库 `data/`；数据缺口日期由 `/api/v1/today` 的随机兜底策略平滑处理。

```
data/stories_*.json ──▶ 构建时 biodays seed --strict ──▶ 镜像内 SQLite
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
├── docker-compose.yml     单个 app 服务，无需数据挂载
├── data/                  Git 管理的季度与主题故事 JSON（408 篇）
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

本地开发直接使用仓库中的 JSON；生产容器已包含构建时生成的 SQLite：

```bash
cd server
export BIODAYS_DATA_DIR=../data

go run ./cmd/biodays seed --strict  # imported=408 skipped=0 files=8 failed_files=0
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

Next.js 通过 `web/proxy.ts` 将 `/api/*`、`/swagger/*`、`/healthz` 和 `/readyz` 原路径代理到 `BIODAYS_API_URL`（默认 `http://127.0.0.1:8080`），开发与 standalone 生产环境均生效。代理目标在运行时读取，镜像无需为不同 API 地址重新构建；浏览器请求保持同源。

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
| `BIODAYS_API_URL` | `http://127.0.0.1:8080` | SSR / ISR 数据抓取与 `/api` 代理目标 |
| `BIODAYS_SITE_URL` | `http://localhost:3000` | canonical / OpenGraph / sitemap 站点根地址 |
| `NEXT_PUBLIC_BIODAYS_DOCS_URL` | `/swagger/index.html` | 前端关于页与任务栏直达的 API 文档链接 |

---

## API 速查

所有业务端点位于 `/api/v1`，成功时直接返回对应资源 JSON；列表结果统一为 `{ data: [...], meta: { total, page, page_size } }` 格式。

| 方法 | 路径 | 说明 | 查询参数 |
|---|---|---|---|
| GET | `/healthz` | 存活检查（不计入限流） | 无 |
| GET | `/readyz` | 就绪检查，含数据库连通性（不计入限流） | 无 |
| GET | `/api/v1/today` | 当日全部候选中优先随机选未读故事 | `tz`（默认 `+08:00`）、`fallback`（`random` 或 `none`） |
| GET | `/api/v1/stories` | 分页列表 | `month`、`day`、`tag`、`q`、`page`、`page_size`（上限 100）、`view=full|summary`（默认 full） |
| GET | `/api/v1/stories/{id}` | 单篇详情（含正文与引用） | 路径参数 `id`（如 `Q1-01`） |
| GET | `/api/v1/stories/random` | 优先随机未读故事，更新已读 Cookie | `exclude`（逗号分隔排除列表，如 `Q1-01,Q4-02`） |
| GET | `/api/v1/tags` | 全部标签及关联篇数 | 无 |
| POST | `/api/v1/history` | 记录浏览，仅更新 Cookie，成功返回 204 | JSON：`{"id":"Q1-01"}` |
| GET | `/api/v1/meta` | 数据集指标（总篇数、月份计数、覆盖、数据版本） | 无 |

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
- 缓存策略：普通 GET 成功响应携带 `Cache-Control: public, max-age=300`；随机故事、今日推荐、浏览记录、健康检查和错误响应携带 `no-store`。`view=summary` 省略正文、来源与 geek_quote，供前端列表使用；详情与默认 full 响应不变。

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

#### 单容器镜像（前端 + Go API + SQLite）

根目录 `Dockerfile` 使用 `gcr.io/distroless/nodejs24-debian13:nonroot`。Next.js 监听 3000，Go API 在容器内部监听 8080。`data/stories_*.json` 由 Git 管理，在构建阶段通过 `biodays seed --strict` 导入 SQLite；空数据、无效 JSON、重复 ID 或被跳过的记录均令构建失败。运行镜像只携带生成的数据库，不需要 JSON 目录或数据库卷。

```bash
docker build --platform linux/amd64 -t biodays .
docker run -d --name biodays -p 127.0.0.1:8080:3000 \
  -e BIODAYS_SITE_URL=https://biodays.example.com \
  biodays
```

打开同一个入口即可访问：`/`（前端）、`/api/v1/*`（API）、`/swagger/index.html`（文档）、`/readyz`（健康检查）。启动会校验数据库含有故事；空库会明确报错并退出，避免健康检查通过但今日接口持续 404。

更新故事时修改 `data/stories_*.json` 并刷新 `data/manifest.json`，提交后重建并重新部署镜像即可。数据库随镜像版本更新，回滚镜像也会回滚数据。API 不修改故事数据，浏览记录存储在浏览器 Cookie 中，因此默认无需持久化数据库；不要把旧的空卷继续挂到 `/data`，它会遮住镜像内数据库。

#### 持续更新故事

`data/stories_*.json` 是唯一发布源；`data/manifest.json` 记录各文件数量、总数和 SHA-256 数据版本。不要提交 SQLite。构建时严格校验清单，再生成镜像内数据库；清单过期、数量不符、重复 ID、坏 JSON 或无效日期都会阻止发布。

导出目录可能同时包含整季度、拆分文件和 `raw_sources/`。同步工具递归读取导出 JSON，按 ID 合并完全相同的副本；同 ID 内容不同会报出冲突文件，需选择权威文件单独同步。

```bash
# 仓库根目录：先查看新增、更新与去重数量
python3 scripts/stories.py audit /path/to/export/data
# 更新现有 ID；新 ID 进入 data/stories_imported.json；保留导出中未出现的旧故事
python3 scripts/stories.py sync /path/to/export/data
# 如果是直接编辑仓库 JSON，用此命令刷新清单
python3 scripts/stories.py manifest
python3 scripts/stories.py check
python3 -m unittest discover -s scripts -p 'test_*.py'
(cd server && go test ./internal/seed)
git diff -- data
```

确认后提交 `data/` 并推送到 `main`，CNB 会构建和发布新镜像。保留稳定且唯一的故事 ID（不超过 16 字节）；修订同一故事沿用 ID。需要删除故事时，明确从仓库 JSON 删除并刷新清单；同步不会因某次导出不完整而自动删除旧故事。

服务器更新无需挂载目录、手动导入或改数据库权限：

```bash
docker compose pull app
docker compose up -d --force-recreate app
curl -fsS https://your-domain.example/api/v1/meta
```

将响应的 `data.total` 与 `data.dataset_version` 对照本次提交的清单。两者一致即可确认运行的是该批数据；不一致时检查所用镜像 tag/digest，以及是否仍有旧卷遮住 `/data`。回滚镜像会同时回滚故事数据。

#### 页面资源与缓存

图标使用独立模块入口，避免整包图标进入客户端；任务栏复用本地窗口状态，归档标签支持搜索并每页显示 24 个。归档月份统计由 `/api/v1/meta` 返回，列表只请求 summary 视图；sitemap 分页读取全部故事，没有 1000 篇上限。

字体原始文件及许可证保存在仓库，`bun run dev/build` 自动生成 UI 与故事两个 WOFF2 子集，新增文字会随构建纳入。React95 的经典重置与滚动条保留，去掉未使用的内嵌 TTF。不要把生成的 `web/app/fonts/` 提交到 Git。

Caddy 保持 `encode zstd gzip`。Next 的静态资源继续使用带内容哈希的文件名和长期 immutable 缓存；保留框架生成的 `Vary`，不要用全站 header 规则覆盖 HTML/API 缓存或 `Vary`。随机接口不缓存；今日数据每个请求读取一次，并在该次页面渲染与 metadata 之间复用。

#### Distroless 选型

| 镜像 | 适用性 |
| --- | --- |
| `nodejs24-debian13:nonroot` | 默认选择：包含 Node 24、glibc 和 C/C++ 运行库，可运行 Next standalone 与 CGO SQLite 后端 |
| `static-debian13` | 不含 Node，也不能直接运行当前动态链接的 CGO 二进制 |
| `base-debian13` / `cc-debian13` | 可用于单独的 Go 后端，组合容器仍缺 Node |

Go 构建阶段使用 `golang:1.27.3-trixie`，与运行阶段的 Debian 13/glibc 对齐；不再复制 Alpine/musl 二进制。Next 构建使用 glibc 版 Bun 镜像。运行用户为 UID/GID `65532:65532`，数据库与 Next 缓存目录均归该用户所有。健康检查直接使用 Node，无 shell、`wget` 或 `curl` 依赖。

依据：[Distroless 官方镜像列表](https://github.com/GoogleContainerTools/distroless#what-images-are-available)、[Node 镜像内容](https://github.com/GoogleContainerTools/distroless/blob/main/nodejs/README.md)。

#### CNB 自动构建与 Docker Compose

推送 `main` 或 `v*.*.*` 标签时，`.cnb.yml` 运行 Go 检查（含真实 JSON 数据导入测试）和前端检查，然后构建 `linux/amd64` 镜像并发布到 `docker.cnb.cool/ciallo.cv/biodays`。PR 只检查构建，不推送。

```bash
cp .env.example .env
# 设置 BIODAYS_SITE_URL；生产环境可将 BIODAYS_IMAGE 固定到版本或 digest。
docker compose pull
docker compose up -d
docker compose logs -f app
curl http://127.0.0.1:8080/api/v1/meta  # total / dataset_version 应与 data/manifest.json 一致
curl http://127.0.0.1:8080/api/v1/today
```

Compose 只有一个 `app` 服务，没有数据卷或种子目录挂载。默认监听宿主机 `127.0.0.1:8080`，由 Caddy 对外提供 HTTPS；跨机器访问时可调整 `BIODAYS_BIND_IP`。已有部署替换 compose 后运行 `docker compose up -d` 即可移除旧挂载，旧命名卷不会被删除。

#### Caddy 与客户端 IP

宿主机上的 Caddy 可使用 [deploy/Caddyfile.example](deploy/Caddyfile.example)：

```caddyfile
biodays.example.com {
    encode zstd gzip
    reverse_proxy 127.0.0.1:8080
}
```

Caddy 默认生成 `X-Forwarded-For`、`X-Forwarded-Proto` 和 `X-Forwarded-Host`，Next 保留这些转发信息，Gin 从可信代理请求中自动解析 `X-Forwarded-For` / `X-Real-IP`，日志与限流使用同一个客户端 IP。组合容器的 Go 直接上游为 Next，默认信任 `127.0.0.1,::1`；独立部署 Go 时，将 Caddy 的实际 IP/CIDR 加入 `BIODAYS_TRUSTED_PROXIES`。无需把所有来源设为可信。Caddy 自身处于 CDN 后面时，在 Caddy 配置可信 CDN 网段。

SSR/ISR 在服务器内发起的数据请求没有浏览器转发头时，`client_ip=127.0.0.1` 属于正常现象。日志额外包含 `remote_addr` 与 `forwarded_for`，便于与经过 Caddy 的浏览器 API 请求区分。依据：[Caddy 转发头默认行为](https://caddyserver.com/docs/caddyfile/directives/reverse_proxy#defaults)。

#### 404 排查

后端日志已经出现 `/api/v1/today`，说明请求到达 Go。默认随机兜底仍返回 `STORY_NOT_FOUND` 时，检查 `/api/v1/meta` 的 `data.total` 是否为 0，以及启动日志的 `story data ready`。如果只有 `fallback=none` 返回 404，可能只是当天未收录。Next 返回 HTML 404、Go 返回 JSON 业务错误，两者需要分别判断。

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
   - `id` 仅用于稳定链接与浏览记录，不参与季度或日期判断；`Q1-01`、`BIO-C1-01` 或其他唯一 ID（最多 16 字节）同样进入归档、随机与日期推荐，无需按 Q 编码重命名。
   - 当服务端判定某月某日存在多条故事记录时，按独立的 `month` / `day` 元数据筛选，从所有未读候选中等概率抽取；当日候选全读过后可重读，不清空其他日期的浏览记录。
   - 数据未覆盖的日期由 `/api/v1/today` 随机推荐一篇，并在元数据中标记 `meta.is_fallback = true`，前端状态栏会向用户清晰呈现提示；
   - 首页服务端渲染只向内部 API 转发已读 Cookie，并在单次渲染内复用结果；`today` 返回 `no-store` / `Vary: Cookie`，避免其他用户或旧缓存固定住推荐结果。
2. **幂等 Seed 机制**：
   - `biodays seed` 会扫描指定目录下的全部 `stories_*.json` 文件，根据主键 `id` 幂等 upsert；
   - 生产镜像在构建时生成 SQLite，启动时只迁移并校验已有数据；本地开发仍可通过 `BIODAYS_DATA_DIR` 自动导入。
   - 自动建立和更新故事与标签的多对多关联，并主动清理已无任何故事引用的孤儿标签；遇到单条损坏数据记录 warning 日志并跳过，确保整体导入流程稳健不中断；
3. **单容器运行期渲染**：
   - 镜像构建从仓库 JSON 生成数据库；首页动态读取今日故事；归档与最近归档复用 5 分钟服务端数据缓存；详情页首次访问生成，随后按 5 分钟 ISR 更新；
   - Next 反向代理 `/api`、`/swagger`、`/healthz`、`/readyz`，浏览器只访问一个端口；`sitemap.xml`/`robots.txt` 也按运行期 `BIODAYS_SITE_URL` 生成；
4. **CGO SQLite 驱动选型**：
   - 采用 GORM 官方 `gorm.io/driver/sqlite`（底层 `mattn/go-sqlite3`，内置 SQLite amalgamation），取代此前因本机 CGO 工具链故障而使用的纯 Go `glebarez/sqlite`；
   - 优势是官方维护、与 GORM 版本同步、贴近上游 SQLite 行为，并支持 FTS5/JSON1 等扩展能力；代价是构建需要 C 工具链，跨平台交叉编译需准备对应的 C 交叉编译器；
5. **随机与浏览记录**：
   - 通过 Go `crypto/rand.Reader` 获取操作系统支持的密码学随机数，使用 `rand.Int` 无偏抽取候选 ID；不使用固定 seed、时间戳、轮播次序或 SQLite `RANDOM()` 排序。
   - 当前浏览器的已读 ID 存入 `biodays_read_v1` Cookie，压缩后可完整保存目前 408 篇；HTTPOnly、SameSite=Lax、HTTPS 下 Secure，90 天有效且随阅读续期，不存服务器数据库。
   - 首页卡片与实际打开的详情页在客户端调用 `POST /api/v1/history` 记录展示的故事；预加载和 ISR 生成不会标记为已读，故事 HTML 可继续共享缓存。
   - 随机抽取先排除已读及显式 `exclude`，全部读完后清空本轮历史重新抽取，仍尽量避开当前篇；只有一篇或显式排除全部时保持原来的兜底行为。新故事自动作为未读候选。
   - Cookie 与账号、其他浏览器或设备不互通，清除 Cookie 会清空记录。为遵守浏览器容量限制，最多保存 2048 个 ID，压缩值不超过 3500 字节；未来数据超过容量时保留较新的记录。
   - 随机响应为 `no-store`，并保留 `Vary: Cookie`；浏览器随机请求也使用 `cache: no-store`。这是密码学随机抽样并结合已读去重，不宣称物理硬件真随机。
6. **内容来源**：
   - 条目内容均基于科学史专利、期刊论文、诺贝尔奖官方新闻稿及科研机构历史档案整理改写；单篇条目随 API 携带 citation 引用与原始链接。

---

## 技术栈清单

- **后端**：Go 1.27 · Gin v1.12 · GORM · `gorm.io/driver/sqlite` (CGO SQLite) · `caarlos0/env/v11` · `log/slog` · `swaggo/swag` · `stretchr/testify`
- **前端**：Next.js 16 (App Router) · React 19 · TypeScript (strict) · Tailwind CSS v4 · `@react95/core` · `@react95/icons` · `@neodrag/react` · Fusion Pixel Font · TanStack Query · Bun · Biome
