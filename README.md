# Biodays

以 Windows 98 桌面介绍生物学知识与实验室科学史。现有 408 篇故事已经无损迁移为 Markdown；文章阅读不依赖数据库或邮件服务。

- Next.js 16 / React 19，保留 Win98 界面、原文章 ID、链接、随机阅读与已读 Cookie。
- 内容保存在 `web/content/stories/`，Git 提交触发 Vercel 发布。
- Supabase PostgreSQL 保存反馈、订阅、邮件任务、运行配置和审计；Supabase Auth 管理邮箱登录，关闭公开注册。
- Resend 官方 SDK 发送邮件，Supabase Cron 调度队列，webhook 验签更新投递情况。

## 本地运行

需要 Node.js 24、Bun 1.4.4。仅阅读故事不需要 Supabase：

```sh
cd web
bun install --frozen-lockfile
cp .env.example .env.local
bun run dev
```

访问 `http://localhost:3000`。文章详情和归档在构建时生成，首页按 +08:00 日期和当前浏览器的已读记录选择故事。

完整反馈开发需要 Docker 和 Supabase CLI：

```sh
supabase start -x realtime,storage-api,imgproxy,postgres-meta,studio,edge-runtime,logflare,vector,supavisor,mailpit
```

将本地 Supabase 的 URL、anon/publishable key、service_role/secret key 配置到 `web/.env.local`。不要提交密钥；`SUPABASE_SECRET_KEY` 仅在服务端使用。

通过环境变量设置 `BIODAYS_ADMIN_EMAIL` 和至少 12 位的 `BIODAYS_ADMIN_PASSWORD`，在 `web/` 运行 `bun run admin:create`。脚本预先确认邮箱并加入管理员成员表；普通 Supabase 用户没有管理权限。首次创建后删除密码环境变量。已有账号可在 SQL editor 中把用户 UUID 加入 `biodays_admins`。

## 编辑故事

每篇文章一个 `.md` 文件，YAML frontmatter 保存 `id/month/day/year/title/lab_item/tags/summary/geek_quote/source`，正文使用 Markdown，支持现有数学公式。不要更改已发布故事的 ID 和文件名。

```sh
cd web
bun run content:check
bun run build
```

生成的内容索引和字体文件不提交；构建会重新生成。页面布局、主题和文章更改通过 Git 发布；运营配置在后台保存后下次请求即生效。后台故事管理页提供仓库入口，旧文章写入 API 返回 410。

`web/tests/fixtures/legacy-stories.json` 是迁移前的只读对照快照，不作为生产内容源。迁移完成时逐字段及正文完全一致；今后有意修订旧故事时，应评审并更新对应的兼容性断言，不能重新运行转换脚本覆盖新内容。

## 测试

```sh
make install
make test
make test-db       # 独立临时 PostgreSQL，自动销毁，不需要真实项目
make build
cd web && bunx playwright install chromium && bun run test:e2e
```

未配置本地 Supabase 时，完整互动和队列集成测试会明确跳过。运行全部检查：启动上述本地 Supabase，执行 `python3 scripts/local-test-env.py --write`；然后在 `web/` 用 `bun --env-file=.env.test.local run admin:create`、`bun --env-file=.env.test.local run test`、`bun --env-file=.env.test.local run test:e2e`。该文件仅含本地项目测试凭据并被 Git 忽略。集成测试会清理专用本地项目的数据，禁止用于生产数据库。

CI 自动创建隔离的 Supabase，执行真实认证和数据库测试、Resend SDK 模拟网络测试、生产构建，以及桌面/移动端浏览器回归。数据库凭据不参与构建。生产 Resend 邮件与线上 Cron 仍需部署后按文档验收。

## 部署

详见 [Vercel、Supabase 与 Resend 部署和回滚](docs/deployment.md)。Vercel 项目 Root Directory 设置为 `web`，Node.js 24，使用仓库中的安装与构建配置。生产与预览使用不同 Supabase 项目；预览邮件仅允许测试收件人。

原 Go、SQLite/自管 PostgreSQL、SMTP、容器发布流程已由该架构替换。原实现和迁移前工作完整保存在基线提交 `f1f1cb4`；`docs/plans/` 中早期方案仅供历史参考。
