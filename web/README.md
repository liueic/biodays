# biodays · web

biodays 的 Next.js 前端。基于 App Router、React 19、TypeScript strict、Tailwind CSS v4、React95、Neodrag 与 Fusion Pixel 像素字体构建，呈现经典的 Windows 98 复古桌面交互，包管理器为 Bun。

## 视觉与交互

- **Windows 98 桌面视觉**：经典青色桌面（`#008080`）、Win98 渐变标题栏（`#000080` → `#1084d0`）、开始菜单任务栏与 16x16 / 32x32 经典系统图标；
- **窗口拖拽与位置记忆**：桌面端支持通过标题栏自由拖拽视窗（`@neodrag/react`），并在切页时自动保持上次拖拽坐标；移动端自动适配最大化视窗；
- **双排版字体栈**：
  - 界面与控件：`Fusion Pixel 12px Proportional Simplified Chinese` 点阵像素字体；
  - 故事正文与长篇阅读：系统衬线宋体（`Songti SC` / `SimSun` / `Noto Serif CJK SC`），开启平滑抗锯齿与 15px / 1.9 行高；
- **资源管理器风格归档库**：按月份树形/下拉筛选、标签过滤与防抖即时检索，配合 TanStack Query 缓存与流畅分页；
- **黄金分割居中**：垂直空间以 `0.382 : 0.618` 黄金分割自适应，短视窗优雅居中偏上，长内容平滑滚动。
- **移动端阅读**：仅在宽屏且使用精细指针时启用拖拽；触屏保留内容区原生滚动。长标题截断于标题栏、正文自动换行，超宽公式和代码局部横向滚动；手机归档列表跟随页面滚动，任务栏预留底部安全区域。

## 路由与布局

- `app/(site)/layout.tsx`：阅读区根布局，保留 Umami 统计与录制脚本；包含首页、归档、故事详情、关于。
- `app/(private)/layout.tsx`：反馈与后台根布局，共享 Win98 桌面外壳，但**不加载**统计或录制。两个根布局之间切换会整页重载，从而结束阅读页已加载的录制脚本。
- `app/(private)/feedback`：灵感信箱（匿名投稿、回执、重发确认邮件）。
- `app/(private)/feedback/confirm`、`unsubscribe`：从邮件 fragment `#token=…` 读取令牌，读后立即用 `history.replaceState` 清除地址栏；只有点击按钮才 POST。
- `app/(private)/admin/login`、`admin/feedback`、`admin/feedback/[id]`、`admin/mail`：单管理员登录、收件箱、详情处理与 SMTP/队列面板。
- `app/robots.ts`、`app/sitemap.ts`：后台、确认与退订页不索引；`/feedback` 可索引并加入 sitemap；`next.config.ts` 额外设置 `X-Robots-Tag`、`Referrer-Policy: no-referrer` 与 `no-store`。

反馈与管理请求通过 `lib/feedback-api.ts`、`lib/admin-api.ts` 走同源 `/api/v1`；管理 CSRF 令牌只保存在内存（不写 localStorage），页面加载后由 `/admin/me` 重新获取。后台 Cookie 由 Go 设置为 HttpOnly + SameSite=Strict，HTTPS 站点带 Secure。

## 开发

工具链版本由仓库根目录 `.prototools` 固定（Bun 1.4.4）；使用 proto 时先在仓库根目录执行 `proto install`。

```bash
bun install
cp .env.example .env.local   # 按需修改后端地址
bun run dev                  # http://localhost:3000
```

后端需先运行在 `BIODAYS_API_URL`（默认 `http://localhost:8080`）。开发期 `/api/*` 由 `next.config.ts` 的 rewrites 代理到后端，浏览器同源，无 CORS 问题。

## 命令

```bash
bun run dev        # 开发服务器
bun run build      # 生产构建
bun run start      # 启动生产服务
bun run typecheck  # tsc --noEmit
bun run lint       # Biome check
bun run lint:fix   # Biome check --write
bun run format     # Biome format --write
```

## 环境变量

| 变量                           | 默认                                       | 说明                              |
| ------------------------------ | ------------------------------------------ | --------------------------------- |
| `BIODAYS_API_URL`              | `http://localhost:8080`                    | 服务端 fetch 与 `/api` 代理目标   |
| `BIODAYS_SITE_URL`             | `http://localhost:3000`                    | canonical / OG / sitemap          |
| `NEXT_PUBLIC_BIODAYS_DOCS_URL` | `http://localhost:8080/swagger/index.html` | 关于页与任务栏的 Swagger 文档链接 |

反馈页的人机验证配置（PoW / Turnstile site key）由 Go 的 `GET /api/v1/feedback/options` 在运行时下发，**没有新增 `NEXT_PUBLIC_*` 变量**。Turnstile 脚本只在 `FeedbackForm` 内按需加载，不进入 `(private)` 根布局，也不会出现在 `/admin/*`；PoW 使用 `web/lib/feedback/pow-worker.ts`，Worker 不可用时回退主线程分片计算。

## 页面

| 路由                          | 渲染                | 说明                                                             |
| ----------------------------- | ------------------- | ---------------------------------------------------------------- |
| `/`                           | 动态渲染            | 时区 +08:00；从当日候选中优先随机选未读；无当日故事则从全库兜底  |
| `/story/[id]`                 | 按需 ISR 300s       | 首次访问生成；完整正文、来源文献与再来一篇                       |
| `/archive`                    | 动态渲染 + 数据缓存 | 5 分钟共享数据缓存；月份、标签、关键词、分页（TanStack Query）   |
| `/about`                      | 动态渲染 + 数据缓存 | 运行时读取数据覆盖统计，成功响应缓存 5 分钟；每日机制与 API 速查 |
| `/sitemap.xml`, `/robots.txt` | 动态 metadata route | 自动包含全部故事 URL 与搜索引擎规则                              |

## 使用统计

生产环境通过根布局接入 `https://ana.juniortree.com` 的 Umami，网站 ID 为 `c3b4bed2-5ad6-4265-9c23-f9e462a829c4`。访问统计脚本在页面初始化后加载，录制脚本在页面加载完成后的浏览器空闲时间加载；全站共用，页面切换不会重复插入脚本。本地 `bun run dev` 不加载统计脚本。

录制和热力图是否启用、采样率及遮罩规则由 Umami 后台的该网站设置控制。

## 部署

Vercel 或任意 Node / Bun 托管：

```bash
bun install
BIODAYS_API_URL=https://api.example.com \
BIODAYS_SITE_URL=https://biodays.example.com \
NEXT_PUBLIC_BIODAYS_DOCS_URL=https://api.example.com/swagger/index.html \
bun run build
bun run start
```

同时把前端正式域名加入后端的 `BIODAYS_CORS_ORIGINS`。

构建不依赖运行中的 API。首页请求读取当前浏览器的已读 Cookie 并转发给内部 API；详情 HTML 继续共享 ISR 缓存，实际打开后才记录已读。随机按钮优先抽取未读故事，记录由后端写入 90 天有效的 HttpOnly Cookie，无需登录。
