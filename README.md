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
├── docker-compose.yml     单个 app 服务 + 反馈库持久化卷
├── data/                  Git 管理的季度与主题故事 JSON（408 篇）
├── deploy/launcher.js     容器内进程 supervisor：启动 Go API，就绪后启动 Next
├── server/                Go module（module biodays/server）
│   ├── cmd/biodays/       单二进制入口：serve / seed / migrate / hash-password / admin set-password / cleanup / backup / restore
│   ├── internal/          config · model · store · service · mailer · handler · middleware · router · seed
│   └── docs/              swag 生成的 Swagger 文档（挂载于 /swagger/index.html）
└── web/                   Next.js 16 + React 19 + Tailwind v4（Bun 管理）
    ├── app/(site)/        今日 (/) / 档案 (/archive) / 详情 (/story/[id]) / 关于 (/about)
    ├── app/(private)/     灵感信箱 / 确认订阅 / 退订 / 管理员登录 / 收件箱 / 邮件状态
    ├── components/        React95 桌面视窗、反馈表单、管理员收件箱、归档资源管理器
    └── lib/               API client、反馈/管理请求封装、日期与文本工具
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
| `BIODAYS_DB_DSN` | 空 | 设为 `postgres://...` 时故事与反馈全部存入该 PostgreSQL 库；为空使用下方 SQLite 路径 |
| `BIODAYS_DB_PATH` | `./biodays.db` | SQLite 数据文件路径（DSN 为空时生效） |
| `BIODAYS_DATA_DIR` | 必填（执行 seed 时） | 原始 `stories_*.json` 所在目录；serve 启动时按数据集指纹**增量导入**（详见下方「数据集更新策略」） |
| `BIODAYS_ADMIN_API_TOKEN` | 空 | 管理端故事发布的静态 Bearer Token（≥32 字节）；设置后可脚本/CI 发布，仅作用于故事接口 |
| `BIODAYS_CORS_ORIGINS` | `http://localhost:3000` | 跨域放行源，多个以逗号分隔，支持 `*`；`BIODAYS_SITE_URL` 的 origin 会自动加入 |
| `BIODAYS_TRUSTED_PROXIES` | 空 | 可信反向代理 IP/CIDR，多个以逗号分隔；仅对这些来源信任 `X-Forwarded-For` |
| `BIODAYS_RATE_LIMIT` | `60` | 单 IP 每分钟限流请求数 |
| `BIODAYS_LOG_LEVEL` | `info` | 日志级别：`debug` / `info` / `warn` / `error` |
| `BIODAYS_SITE_URL` | `http://localhost:3000` | 确认/退订链接使用的可信站点 origin，也用于校验管理写请求的 Origin |
| `BIODAYS_FEEDBACK_ENABLED` | `false` | 是否启用匿名反馈与管理员后台；关闭时故事服务不受影响 |
| `BIODAYS_FEEDBACK_DB_PATH` | `./feedback.db` | 独立反馈库；容器内为 `/var/lib/biodays/feedback.db` |
| `BIODAYS_ADMIN_USERNAME` | 空（可选） | 旧部署首次启动导入用；账号实际存在数据库，由 `biodays admin set-password` 管理 |
| `BIODAYS_ADMIN_PASSWORD_HASH` | 空（可选） | 同上；提供时必须是合法 bcrypt 哈希，且与 `BIODAYS_ADMIN_USERNAME` 同时提供 |
| `BIODAYS_ADMIN_SESSION_TTL` | `8h` | 管理员会话绝对有效期 |
| `BIODAYS_ABUSE_KEY_SECRET` | 空（自动生成） | IP/邮箱 HMAC 限流键与令牌指纹密钥；留空时首次启动生成随机密钥并持久化到数据库；显式提供则需 ≥32 字节 |
| `BIODAYS_SETTINGS_ENC_KEY` | 空（推荐设置） | 后台保存密钥类设置（SMTP 密码 / Turnstile Secret / CF Token）的 AES-256-GCM 密钥，`openssl rand -base64 32`；密文存数据库、密钥只在这里 |

**运行时设置（数据库优先，后台热生效）**：SMTP 全套（`BIODAYS_SMTP_*`、`BIODAYS_FEEDBACK_NOTIFY_TO`）、频控限额（`BIODAYS_FEEDBACK_SUBMIT/RESEND/CONFIRM/ADMIN_MAIL_*`）、订阅开关（`BIODAYS_FEEDBACK_SUBSCRIPTIONS_ENABLED`）、Turnstile（`BIODAYS_TURNSTILE_*`）、垃圾评分（`BIODAYS_FEEDBACK_SPAM_*`）与 Workers AI 复核（`BIODAYS_AI_SPAM_*`、`BIODAYS_CF_*`）这些环境变量只在**首次启动**时导入数据库 `runtime_settings` 文档；此后数据库值优先，请在 `/admin/settings`（站点设置）中修改，保存后立即生效、带 revision 乐观并发与审计记录，密钥加密存储、永不回显。修改 `.env` 中的同名变量不再生效。

PoW 工作量证明（`BIODAYS_FEEDBACK_POW_*`）已于 2026-10 移除；大陆访客的人机防线由每 IP 频控 + 规则垃圾评分承担，均可在后台热调。
| `BIODAYS_AI_SPAM_MODEL` | `@cf/cloudflare/clef-flash` | 模型 ID；切换模型只改此值 |
| `BIODAYS_AI_SPAM_STYLE` | `decision` | `decision`（Clef 系列）或 `chat`（Qwen/GLM/Llama） |
| `BIODAYS_AI_SPAM_TIMEOUT` | `2s` | AI 复核超时，1–10s；失败按 uncertain 放行 |
| `BIODAYS_AI_SPAM_THRESHOLD` | `0.8` | spam/legit 置信度阈值 |
| `BIODAYS_AI_SPAM_DAILY_LIMIT` | `500` | UTC 日窗口 AI 调用额度 |
| `BIODAYS_AI_SPAM_MAX_CHARS` | `4000` | 发给 AI 的去标识文本上限 |
| `GIN_MODE` | `release`（镜像内固定） | Gin 运行模式；未设置时 Gin 会退回 `debug` 并打印 `[GIN-debug]` 警告。镜像已固定为 `release`，本地开发想开调试路由日志时显式设为 `debug` |

> 部署在 Nginx/Caddy 等反向代理后时，把代理地址加入 `BIODAYS_TRUSTED_PROXIES`（例如 `127.0.0.1/32,172.18.0.0/16`），限流才会按真实客户端 IP 生效。不要配置 `0.0.0.0/0`，否则 `X-Forwarded-For` 可被伪造。直连部署时保持为空即可。

#### 前端环境变量（`web/.env.local`，参考 `web/.env.example`）

| 变量 | 默认值 | 说明 |
|---|---|---|
| `BIODAYS_API_URL` | `http://127.0.0.1:8080` | SSR / ISR 数据抓取与 `/api` 代理目标 |
| `BIODAYS_SITE_URL` | `http://localhost:3000` | canonical / OpenGraph / sitemap 站点根地址 |
| `NEXT_PUBLIC_BIODAYS_DOCS_URL` | `/swagger/index.html` | 前端关于页与任务栏直达的 API 文档链接 |

### 4. 匿名反馈与管理员后台（可选，默认关闭）

访客无需注册即可提交反馈。昵称与邮箱只是这一条投稿的联系方式，不是账号；管理员使用独立后台登录。启用步骤：

1. 在 `.env` / 运行环境中设置（完整列表见 `.env.example` 与上方环境变量表）：

   ```bash
   BIODAYS_FEEDBACK_ENABLED=true
   # 本地 Next dev 用 3000；Docker Compose 用映射端口（默认 8080）或公网 origin
   BIODAYS_SITE_URL=http://localhost:3000
   ```

   管理员账号存在数据库里（不再需要环境变量里的 bcrypt 哈希），限流密钥留空时会自动生成并持久化。

2. 创建/修改管理员账号（交互式输入两次密码，或用 `--stdin` 管道）：

   ```bash
   cd server
   go run ./cmd/biodays admin set-password
   # 容器部署：
   docker compose exec app /usr/local/bin/biodays admin set-password
   # 脚本化：
   printf 'your-long-password\n' | go run ./cmd/biodays admin set-password --stdin
   ```

   密码长度 8–72 字节；修改后旧会话立即失效。账号未初始化时登录接口返回 `ADMIN_NOT_INITIALIZED`（503），前端会提示初始化命令。老部署升级时，`BIODAYS_ADMIN_USERNAME` + `BIODAYS_ADMIN_PASSWORD_HASH` 会在首次启动导入数据库，之后即可删除这两个变量。

   > **容易踩的坑**：`BIODAYS_SITE_URL` 必须是浏览器实际访问的 origin（`scheme://host[:port]`，不要带路径或结尾 `/`）。确认/退订链接、canonical、sitemap 和管理端 Origin 校验都依赖它。`BIODAYS_CORS_ORIGINS` 的 `*` 只作用于公开 API 跨域，管理端写请求始终只认 `BIODAYS_SITE_URL`（及 CORS 列表中显式列出的 origin）。

4. **运行时设置与热生效**：SMTP、频控限额、Turnstile、垃圾评分、Workers AI 复核与订阅开关保存在数据库 `runtime_settings` 文档中，在 `/admin/settings` 修改后立即生效，无需重启容器。首次启动会把 `.env` 中同名变量的值导入数据库作为初始值，此后以数据库为准。

   - **密钥加密**：设置 `BIODAYS_SETTINGS_ENC_KEY`（`openssl rand -base64 32`）后，SMTP 密码 / Turnstile Secret / CF Token 以 AES-256-GCM 密文落库，API 与页面永不回传，只显示「已配置」；未设置该密钥时这些字段无法保存（其余设置不受影响）。密钥丢失只需在后台重新录入密钥字段。
   - **并发保护**：保存携带 `base_revision`，与他人修改冲突时返回 409，前端提示刷新后重试。
   - **审计**：每次保存记录变更摘要（密钥显示 `••••`），可在设置页底部查看。
   - **反滥用现状**：PoW 已移除；大陆访客依靠每 IP 频控 + 规则垃圾评分（含可选 Workers AI 复核），全部参数可在后台热调，被灌水时直接调低限额即可即时止损。

3. 启动后端与前端，访问：

   - `/feedback`：匿名投稿、订阅状态回执与重发确认邮件；
   - `/feedback/confirm#token=…`：邮件中的确认链接（打开页面不会修改状态，点击按钮才 POST）；
   - `/feedback/unsubscribe#token=…`：退订；
   - `/admin/login`：管理员登录；
   - `/admin/feedback`：Outlook Express 风格收件箱，列表与详情联动；
   - `/admin/mail`：邮件队列计数、最近错误、测试邮件与人工重试；
   - `/admin/settings`：站点设置——SMTP、频控限额、Turnstile、垃圾评分、Workers AI 复核与订阅开关，保存后**热生效**（无需重启），带 revision 并发保护与修改审计；密钥字段加密存储、永不回显。

5. 需要用户订阅处理结果时，再启用 SMTP：

   ```bash
   BIODAYS_SMTP_ENABLED=true
   BIODAYS_SMTP_HOST=smtp.example.com
   BIODAYS_SMTP_PORT=587
   BIODAYS_SMTP_TLS_MODE=starttls   # 或 implicit_tls（465）
   BIODAYS_SMTP_USERNAME=...
   BIODAYS_SMTP_PASSWORD='...'   # 含 $ 的授权码必须用单引号
   BIODAYS_SMTP_FROM=notify@example.com
   BIODAYS_FEEDBACK_NOTIFY_TO=admin@example.com
   ```

   部署后先在 `/admin/mail` 发送测试邮件。测试邮件和新反馈通知只发往 `BIODAYS_FEEDBACK_NOTIFY_TO`，调用方不能指定任意收件人。SMTP 关闭时，反馈仍会保存，管理员通知与确认任务保留在队列中，订阅选项不会展示，服务端也会拒绝新的 `subscribe_updates=true` 请求。

行为约定：

- 投稿使用 `request_id` 去重：相同 ID 与相同规范化内容返回原回执，不重复建记录或邮件；相同 ID 不同内容返回 409。函数内部在**同一事务**写入反馈、事件、令牌与 outbox，SMTP 不参与请求事务。
- 订阅是双重确认：确认前不发送结果邮件。确认令牌 24 小时有效、单次使用；重发会作废旧令牌与旧任务。确认时若已有可通知状态，只补排队一次最新结果。
- 邮件只在管理员明确勾选“保存并通知订阅者”且状态真正变化、订阅已确认时排队；内部备注、处理中状态、垃圾信息不会触发用户邮件。退订会立即取消尚未发送的用户邮件。
- 邮件任务状态为 `pending` / `sending` / `sent` / `failed` / `unknown` / `cancelled`。临时错误最多尝试 5 次（1、5、30、120 分钟）；永久拒收直接失败；发送阶段断链标为 `unknown`，需管理员核对后重试，避免重复邮件。`sent` 仅表示 SMTP 已接受。
- 保留策略：已关闭反馈的邮箱与昵称 90 天后清除并撤销订阅；邮件发送成功后立即清除敏感 payload；已投递/cancelled 任务元数据保留 30 天；过期会话、确认令牌、限流桶与一次性挑战定期清理。可用 `biodays cleanup` 手动执行。
- 敏感页面：`/admin/*`、确认与退订页不加载 Umami 统计或录制，设置 `X-Robots-Tag: noindex` 与 `Referrer-Policy: no-referrer`，并写入 `robots.txt` 排除规则。`/feedback` 公开可索引并加入 sitemap。
- 请求日志不记录正文、完整邮箱、会话或订阅令牌；业务表不保存原始 IP。

#### 反垃圾与人工验证

公开投稿按“幂等检查 → 验证 → 频控 → 规则评分 → 可选 AI 复核 → 单事务写入”执行；同一 `request_id` 与同内容的重试直接返回原回执，不会重复消耗挑战或验证码。

- **PoW 主路径**：`POST /api/v1/feedback/challenge` 签发一次性挑战，前端在 Web Worker 中计算 `SHA-256(salt + ":" + nonce)` 的前导零；挑战绑定客户端 IP、默认 10 分钟有效、至少等待 3 秒，并在写入事务内消费。大陆访客默认使用此路径。
- **Turnstile 可选**：默认关闭；前端从 `/api/v1/feedback/options` 读取 site key，8 秒加载/就绪超时后回退 PoW。Cloudflare 官方明确不支持中国大陆，不要把 Turnstile 作为唯一入口。
- **规则评分**：链接数量、短链、零宽/控制字符、重复正文哈希、一次性邮箱、邮箱域名短时爆发、关键词等信号累加为 0–100 分。默认 0–29 正常、30–69 待复核、70–100 自动隔离；自动隔离复用现有 `spam` 状态，不发管理员/订阅邮件，管理员可在详情页“恢复为待处理”。
- **AI 复核**：只有“待复核”投稿才会调用 Workers AI；文本先去标识（邮箱、电话、URL 参数、零宽字符），日志只记录判定、模型、延迟与分数，不记录正文。默认模型 `@cf/cloudflare/clef-flash`（decision 请求形态）；切到 Qwen/GLM/Llama 等聊天模型时把 `BIODAYS_AI_SPAM_STYLE` 改为 `chat`。免费额度为 10,000 Neurons/天，建议先用真实样本离线评测再开启。
- **邮件安全**：首次确认邮件按收件邮箱 1/小时、3/天，并受全局 100/天约束；管理员逐条通知 10/小时封顶。确认邮件被节流时仍在回执中明确返回 `confirmation_email=deferred`，不会假装已发送。
- **隐私**：Turnstile 与 Workers AI 都会接触第三方；启用 AI 时页面会提示“去标识文本可能由境外 AI 服务辅助判断”，不发送邮箱、昵称、IP 或链接参数。详情见 `.env.example` 与 `docs/plans/2026-10-25-feedback-antispam-plan.md`。

新增错误码：`CHALLENGE_REQUIRED`、`CHALLENGE_INVALID`、`CHALLENGE_EXPIRED`、`CHALLENGE_CONSUMED`、`CHALLENGE_FAILED`、`CHALLENGE_TOO_FAST`、`TURNSTILE_REQUIRED`、`TURNSTILE_FAILED`、`VERIFICATION_UNAVAILABLE`、`UNSUPPORTED_MEDIA_TYPE`。

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
go test ./...              # 故事端点、反馈事务/去重/频控、订阅确认/退订、管理会话/CSRF、worker 重试
go test -race ./...        # CI 使用的竞态检测
go vet ./...

# 前端：类型检查与 Biome 检查
cd ../web
bun run typecheck          # tsc --noEmit
bun run lint               # Biome check
bun run format             # Biome format
```

---

## 部署

### 后端（CGO 单二进制，SQLite 或 PostgreSQL）

本项目默认使用 GORM 官方 SQLite 驱动（`gorm.io/driver/sqlite`，底层为 `mattn/go-sqlite3`），设置 `BIODAYS_DB_DSN` 后切换到 PostgreSQL（`gorm.io/driver/postgres`），开发与测试无需数据库服务。编译需要本机 C 工具链：macOS 安装 Xcode Command Line Tools，Linux 安装 `gcc`。Docker 镜像内已配置好工具链，推荐用容器构建以保证可复现：

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

根目录 `Dockerfile` 使用 `gcr.io/distroless/nodejs24-debian13:nonroot`。Next.js 监听 3000，Go API 在容器内部监听 8080。`data/stories_*.json` 由 Git 管理，在构建阶段通过 `biodays seed --strict` 导入 SQLite；空数据、无效 JSON、重复 ID 或被跳过的记录均令构建失败。运行镜像同时携带生成的 SQLite（供无 PostgreSQL 的部署直接使用）和 JSON 语料目录 `/stories-data`（供启动时增量导入：新语料随镜像发布，管理员编辑与删除不会被覆盖）。

存储支持双模式：设置 `BIODAYS_DB_DSN`（`postgres://...`）时故事与反馈全部存入同一个 PostgreSQL 数据库，`docker-compose.yml` 默认如此（附带 `postgres:17-alpine` 的 `db` 服务）；不设置 DSN 则保持单文件 SQLite。PostgreSQL 模式下 `biodays backup/restore` 文件快照不可用，改用 `pg_dump`。

```bash
docker build --platform linux/amd64 -t biodays .
docker run -d --name biodays -p 127.0.0.1:8080:3000 \
  -v biodays-feedback:/var/lib/biodays \
  -e BIODAYS_SITE_URL=https://biodays.example.com \
  -e BIODAYS_FEEDBACK_ENABLED=true \
  -e BIODAYS_ADMIN_USERNAME=admin \
  -e BIODAYS_ADMIN_PASSWORD_HASH='$2a$12$...' \
  -e BIODAYS_ABUSE_KEY_SECRET='...' \
  biodays
```

打开同一个入口即可访问：`/`（前端）、`/api/v1/*`（API）、`/swagger/index.html`（文档）、`/readyz`（健康检查）。启动会校验数据库含有故事；空库会明确报错并退出，避免健康检查通过但今日接口持续 404。

**PostgreSQL 模式的首播与更新**：空库第一次启动时自动导入镜像内 `/stories-data` 的 JSON；之后重启不再重复导入，管理员通过 API/网页做的修改会一直保留。日常发布新故事不再需要重建镜像——见下文「持续更新故事」。

#### 反馈库持久化、备份与恢复

**PostgreSQL 模式（compose 默认）**：反馈、管理员会话、订阅与邮件任务都在 `db` 服务的 `biodays-pg` 命名卷里，与应用库同库。备份与恢复使用 PostgreSQL 自身工具：

```bash
docker compose exec db pg_dump -U biodays biodays > backup.sql
# 恢复：先停应用，再导入
docker compose stop app
cat backup.sql | docker compose exec -T db psql -U biodays biodays
docker compose start app
```

**SQLite 回退模式**（未设 `BIODAYS_DB_DSN`）：反馈等数据保存在独立的 `/var/lib/biodays/feedback.db`，`biodays-feedback` 命名卷仍在 compose 中保留。此时可使用应用内文件快照：

```bash
# 一致性备份（VACUUM INTO，包含 WAL 中已提交的数据），默认保留 7 天
biodays backup --dest /backups --keep 7

# 恢复：先停服务，再替换目标库。restore 会撤销全部管理员会话，并把未完成邮件任务标为 unknown
biodays restore --source /backups/20261024-120000-feedback.db \
  --target /var/lib/biodays/feedback.db --kind feedback --force
```

恢复后请到 `/admin/mail` 人工核对被暂停的任务，确认没有重复投递后再重试。不要直接复制正在写入的主 DB 文件而忽略 `-wal` / `-shm`。`/readyz` 在启用反馈后会同时检查故事库与反馈库，SMTP 可达性不参与 readiness，避免邮件故障导致站点反复重启。

#### 持续更新故事

故事有两条发布路径，都不需要重建镜像：

**网页编辑**：管理员登录后打开 `/admin/stories`，搜索、编辑、新建或删除故事，保存即时入库。公开页面缓存 300 秒，修改最多 5 分钟后全站生效（`/today` 与随机接口即时生效）。

**命令行发布**：设置 `BIODAYS_ADMIN_API_TOKEN`（`openssl rand -hex 32`）后，用仓库脚本把 `data/` 语料或任意导出推到线上：

```bash
# 发布整个 data/ 语料（默认）
python3 scripts/stories.py publish --api https://your-domain.example --token "$BIODAYS_ADMIN_API_TOKEN"
# 只发布一个导出文件，或按 ID 挑选
python3 scripts/stories.py publish --api ... --token ... --file data/stories_q1.json
python3 scripts/stories.py publish --api ... --token ... --id Q1-03 --id NEW-07
# 命令行查看线上列表
python3 scripts/stories.py list --api https://your-domain.example --token "$BIODAYS_ADMIN_API_TOKEN"
```

`publish` 按 id 幂等 upsert（存在即更新，不存在即创建），单次上限 200 篇、自动分批，服务端校验与 JSON 导入完全一致（id ≤16 字符、合法日历日期、title/content/geek_quote 非空）。

`data/stories_*.json` 的角色是「首播语料 + 增量底本 + 备份」。需要删除线上故事时，用管理页删除或 `curl -X DELETE -H "Authorization: Bearer $TOKEN" .../api/v1/admin/stories/<id>`。

**随镜像/重启增量导入**：`serve` 启动时会把 `BIODAYS_DATA_DIR` 指向的语料指纹与库内 `dataset_version` 比对，不一致才导入，且导入是**增量的**：

- 新增/上游修改的故事 → 正常 upsert；
- 通过管理页编辑过的故事 → 保留管理员版本（不会被语料覆盖）；
- 通过管理页删除的语料故事 → 保持删除（墓碑机制），不会复活；
- 管理员自建的故事 → 不受影响。

因此「改 JSON → 跑 `python3 scripts/stories.py manifest` → 重新部署」即可把新语料增量上线下发，无需清库，也不会回滚线上修改。指纹一致时启动零导入。要强制重置为语料 canon（覆盖编辑、恢复删除），显式运行 `biodays seed [--strict]`。

管理端故事 API 的认证是双通道：脚本用 `Authorization: Bearer <BIODAYS_ADMIN_API_TOKEN>`（可免 Origin/CSRF）；网页沿用登录会话 Cookie + `X-CSRF-Token`。Token 等同发布权限，只放服务端或 CI secret。完整接口见 `/swagger/index.html` 的 admin-stories 分组。

```bash
# 仓库语料整理流程（不变）
python3 scripts/stories.py audit /path/to/export/data
python3 scripts/stories.py sync /path/to/export/data
python3 scripts/stories.py manifest
python3 scripts/stories.py check
python3 -m unittest discover -s scripts -p 'test_*.py'
(cd server && go test ./internal/seed)
```

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
# 必填：BIODAYS_SITE_URL 与 BIODAYS_POSTGRES_PASSWORD（openssl rand -hex 24）
# 生产环境可将 BIODAYS_IMAGE 固定到版本或 digest。
docker compose pull
docker compose up -d
docker compose logs -f app
curl http://127.0.0.1:8080/api/v1/today
```

Compose 包含两个服务：`db`（PostgreSQL 17，命名卷 `biodays-pg`）与 `app`；app 通过 `env_file` 读取 `.env` 的全部 `BIODAYS_*` 变量，`environment` 只保留镜像固定路径和 DSN 拼装。默认监听宿主机 `127.0.0.1:8080`，由 Caddy 对外提供 HTTPS；跨机器访问时可调整 `BIODAYS_BIND_IP`。首次启动空库会自动导入镜像内 `/stories-data` 的 408 篇语料。要回退单容器 SQLite 模式，注释掉 `BIODAYS_DB_DSN` 与 `db` 服务/`depends_on` 即可。

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
6. **反馈与邮件架构**：
   - 故事库与反馈库完全分离：`store.Open` 只迁移 Story/Tag/Dataset；新增 `FeedbackStore` 使用 `feedback.db`（WAL、busy timeout、4 连接），故事删除不级联删除用户反馈，跨库只用字符串 ID 关联；
   - 投稿在单事务内写 `feedbacks`、`feedback_events`、`subscription_tokens`、`mail_outbox`；outbox 使用唯一业务事件键去重，worker 与 API 同进程、单实例领取并以租约防重复；
   - 管理会话只存令牌哈希，CSRF 由服务端密钥从会话哈希派生，Cookie 为 HttpOnly + SameSite=Strict + Path=/，HTTPS 下 Secure；每个管理 API 由 Go 强制执行认证，写接口额外校验 Origin 与 CSRF；
   - 前端使用 `(site)` 与 `(private)` 两个根布局：只有阅读区加载 Umami 统计与录制，反馈/后台页面切换时整页重载，结束已加载的录制脚本；所有反馈/管理响应为 `no-store`，不继承故事路由的公开缓存。
7. **内容来源**：
   - 条目内容均基于科学史专利、期刊论文、诺贝尔奖官方新闻稿及科研机构历史档案整理改写；单篇条目随 API 携带 citation 引用与原始链接。

---

## 技术栈清单

- **后端**：Go 1.27 · Gin v1.12 · GORM · `gorm.io/driver/sqlite` (CGO SQLite) · `caarlos0/env/v11` · `log/slog` · `swaggo/swag` · `stretchr/testify`
- **邮件**：`github.com/wneessen/go-mail` v0.8.1 · 持久化 outbox · 租约 worker
- **前端**：Next.js 16 (App Router) · React 19 · TypeScript (strict) · Tailwind CSS v4 · `@react95/core` · `@react95/icons` · `@neodrag/react` · Fusion Pixel Font · TanStack Query · Bun · Biome
