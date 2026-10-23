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

## 页面

| 路由                          | 渲染                | 说明                                                            |
| ----------------------------- | ------------------- | --------------------------------------------------------------- |
| `/`                           | 动态渲染            | 时区 +08:00；从当日候选中优先随机选未读；无当日故事则从全库兜底 |
| `/story/[id]`                 | 按需 ISR 300s       | 首次访问生成；完整正文、来源文献与再来一篇                      |
| `/archive`                    | 动态渲染 + 数据缓存 | 5 分钟共享数据缓存；月份、标签、关键词、分页（TanStack Query）  |
| `/about`                      | ISR 300s            | 每日机制、数据覆盖、API 速查与来源声明                          |
| `/sitemap.xml`, `/robots.txt` | 动态 metadata route | 自动包含全部故事 URL 与搜索引擎规则                             |

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
