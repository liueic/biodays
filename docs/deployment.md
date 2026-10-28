# Vercel + Supabase + Resend 部署

## 环境与发布

1. 在 Supabase 创建生产项目，以及独立的预览/测试项目。先在测试项目按文件名顺序执行 `supabase/migrations/*.sql`；已有项目只应用尚未执行的迁移。迁移仅管理 `biodays_` 前缀对象。
2. Supabase Auth 开启 GitHub Provider，按下文配置两级回调和固定 GitHub ID 白名单。生产可关闭 Email Provider；本站不提供密码登录。本地配置仅用于隔离的身份与数据库测试。
3. 在 Vercel 连接 GitHub 仓库，Root Directory 设置为 `web`，Node.js 24，使用 `web/vercel.json`。本地固定 24.22.1，CI / Vercel 跟随 24.x 补丁，构建日志输出实际版本。文章修改合并后自动构建，全部故事详情和归档预生成，首页与互动接口动态运行。
4. 分别设置 Production 和 Preview 环境变量。`BIODAYS_SITE_URL` 必须是当前部署的访问源（协议和域名）；预览应使用自己的 URL，也可以不设置此变量，由 VERCEL_URL 自动解析，以便表单同源校验与邮件链接正确。不要让预览连接生产 Supabase。

首次迁移按“推送迁移分支 → GitHub Actions 通过 → Vercel Preview → 验收后合并”执行。`.github/workflows/ci.yml` 在 push、PR 和手动触发时运行内容校验、类型检查、lint、数据库升级及并发回归、业务规则、无数据库密钥构建、数据库故障隔离和桌面/手机浏览器测试。CI 使用临时本地 Supabase，不需要生产密钥，不发送真实邮件或调用模型。

首次 Preview 应部署 CI 已通过的同一个提交。Vercel Git 自动部署与 GitHub Actions 默认可并行运行，连接仓库本身不构成 CI 门禁；需要严格顺序时，在 CI 成功后再触发部署。后续启用自动发布前，建议将 `verify` 设为主分支必需检查。

| 变量 | 用途 |
|---|---|
| `BIODAYS_SITE_URL` | canonical、邮件链接与同源校验，生产为 `https://www.oneleaf.me` |
| `BIODAYS_ADMIN_GITHUB_IDS` | GitHub 固定用户 ID 白名单；生产仅为 `69223323`（liueic），为空或非法时拒绝后台访问 |
| `SUPABASE_URL` | 项目 URL |
| `SUPABASE_PUBLISHABLE_KEY` | publishable key 或 anon key，用于服务端会话客户端 |
| `SUPABASE_SECRET_KEY` | secret key 或 service_role key，仅服务端，禁止 NEXT_PUBLIC 前缀 |
| `BIODAYS_CRON_SECRET` | 队列接口的独立共享密钥，与 Vault 中相同 |
| `RESEND_WEBHOOK_SECRET` | Resend webhook 签名密钥 |
| `RESEND_API_KEY` | Resend API key，唯一来源为环境变量 |
| `RESEND_FROM` | 已验证域名的发件地址，如 `Biodays <hello@example.com>` |
| `BIODAYS_NOTIFY_TO` | 管理员新反馈与测试邮件收件地址 |
| `BIODAYS_MAIL_ALLOWLIST` | 测试收件地址白名单，逗号分隔；预览未设置时拒绝发送 |
| `AI_GATEWAY_API_KEY` | Jev 调用的唯一密钥来源；配置后还需在后台开启 AI 复核 |
| `TURNSTILE_SITE_KEY` | 可选，人机验证的公开 site key |
| `TURNSTILE_SECRET_KEY` | 可选，服务端验证密钥 |
| `TURNSTILE_HOSTNAMES` | 可选，允许的域名，逗号分隔，不含协议、路径或端口 |
| `BIODAYS_FEEDBACK_ENABLED` | 反馈服务总开关，默认 true，可设 false 暂停互动接口 |
| `NEXT_PUBLIC_BIODAYS_DOCS_URL` | 公开 API 文档链接，构建时写入浏览器代码 |
| `BIODAYS_DISABLE_ANALYTICS` | 本地建议 true；生产可设置 false |

完整示例见仓库根目录和 `web/.env.example`（内容一致）；本地复制到 `web/.env.local`。只有环境变量管理密钥、站点地址、发件地址、管理员收件邮箱、Turnstile site key 与域名。后台 `/admin/settings` 只读展示这些配置的状态，密钥只显示是否存在，不回显内容。检查只验证存在性与格式，不会调用模型或发送邮件，不能代替连通性验证。线上修改 Vercel 环境变量后重新部署；本地修改后重启服务。应用每次相关请求解析当前部署的环境变量，数据库不会覆盖它们。

邮件发送、AI 复核、Turnstile 的功能开关，以及订阅、模型、阈值、限流、调用额度和超时保存在 Supabase，通过后台修改后下一次请求生效。新环境的邮件、AI 和 Turnstile 默认关闭，填好环境变量后在后台启用。`RESEND_ENABLED` 和 `BIODAYS_SETTINGS_ENC_KEY` 已废弃，可从 Vercel 移除。

从旧版升级时，先把仍在使用的密钥、发件地址、管理员邮箱和 Turnstile 参数配置到对应环境；如果旧密钥只有数据库中的密文且无法找回，应到服务商重新生成密钥并更新环境变量。应用 `202610270003_environment_settings.sql` 会清理数据库中的历史部署字段，通过原有版本控制与审计记录清理，保留正常业务配置与反馈，旧 Cloudflare AI 保持关闭。即使尚未执行清理迁移，新应用也会忽略这些旧字段。不要依赖旧数据库值继续发信或通过人机验证。

该迁移清理后，旧应用的配置读取方式已不兼容，不能直接回滚到依赖后台密钥的版本。若需要回滚，应先部署兼容环境变量的旧版补丁，或使用受控备份恢复旧配置；不要将密文或密钥写入 Git。

## GitHub 管理员登录

后台只提供 GitHub 登录。每次后台请求通过 Supabase Auth 的 `getUser()` 验证会话，再检查返回的 GitHub identity 固定 ID。生产 `BIODAYS_ADMIN_GITHUB_IDS=69223323`，只允许 `liueic`。姓名、邮箱、可编辑的 user metadata 和旧 `biodays_admins` 成员记录都不授予权限。白名单改变后重新部署，现有会话的下一次请求也会按新名单校验；非白名单账号在回调中退出当前会话。

1. 在 [GitHub → OAuth Apps → New OAuth App](https://github.com/settings/applications/new) 创建 OAuth App。Application name 填 `Biodays Admin`，Homepage URL 填 `https://www.oneleaf.me`，**Authorization callback URL** 填 Supabase Provider 页面显示的地址：`https://uqhlfskucglmembrqfef.supabase.co/auth/v1/callback`。不用开启 Device Flow。此地址是 GitHub 回到 Supabase 的地址。
2. 在 Supabase **Authentication → Sign In / Providers → GitHub** 开启 Provider，填写 GitHub Client ID 和 Client Secret。这两个值只配置在 Supabase，不放进 Vercel。应用使用默认的 `user:email` 身份权限，不申请仓库或组织权限。首次 GitHub 登录需要允许 Supabase 创建身份；可关闭不需要的 Email 和其他登录 Provider。后台仍只允许名单内的 GitHub ID，不需要预创建邮箱或设置网站密码。
3. 在 Supabase **Authentication → URL Configuration** 把 **Site URL** 设为 `https://www.oneleaf.me`，**Redirect URLs** 加入准确的 `https://www.oneleaf.me/auth/callback`。这是 Supabase 回到本站的地址，和第 1 步的 GitHub 回调不同。`redirectTo` 未被允许时，Supabase 会回退到 Site URL；若那里仍是默认的 localhost，就会在登录后跳回本地。生产配置不要保留 localhost。独立本地/预览项目分别添加自己的 `/auth/callback` 地址。
4. Vercel **Production** 设置 `BIODAYS_SITE_URL=https://www.oneleaf.me`、`BIODAYS_ADMIN_GITHUB_IDS=69223323`，以及该环境的三个 Supabase 变量，然后重新部署。Preview 不写入生产站点地址，使用自己的 URL 和独立 Supabase 项目。白名单变量只使用数字 ID，不填用户名；可通过 `gh api users/liueic --jq .id` 查询固定 ID。
5. 访问 `https://www.oneleaf.me/admin/login`，点击「使用 GitHub 账号登录」。已有 GitHub 会话通常直接继续；需要时由 GitHub 处理登录和两步验证。本站不收集账号密码，旧的密码登录接口返回 410。回调通过 HttpOnly Cookie 保存 PKCE 校验信息，完成后只跳转到本站固定后台地址。

配置依据见 [Supabase GitHub 登录](https://supabase.com/docs/guides/auth/social-sign-in/auth-github)和[Redirect URLs](https://supabase.com/docs/guides/auth/redirect-urls)。本地测试验证白名单允许与拒绝、元数据伪造拒绝、旧密码接口关闭、回调错误和来源校验；真实 GitHub 授权仍需在线验收。`test:admin:create` 只为隔离的本地 Supabase 创建测试身份，拒绝连接线上项目。

## Jev 与可疑反馈复核

官方 `ai@7.0.154` 和 `zod@4.6.7` 已固定版本，使用 `experimental_decide` + `createGateway().decisionModel()`；实验接口可能在补丁版本变化，升级需重跑 SDK 合约测试。默认模型固定 `typesafe-ai/jev-1.14.0`，后台也允许 `typesafe-ai/jev` 或明确的 Jev 版本。固定版本可避免别名升级带来的判断变化。

先应用 `202610270002_jev.sql`，再部署新应用。它新增可空的 `spam_probability`，保留原置信度及其他数据。旧应用可以忽略新列；新应用的 `/readyz` 会检查该列是否存在。旧 Cloudflare AI 设置在新应用读取时兼容转换为关闭的 Jev 设置，保留合法的限额及超时；读取不写库、不递增版本，下一次管理员保存通过原有 CAS 和审计写入新格式。旧账号、模式与 token 不会回显或传给 Gateway。

在 Vercel AI Gateway 创建密钥并配置 `AI_GATEWAY_API_KEY`，后台只显示密钥是否配置。旧数据库密钥被忽略；后台接口拒绝密钥与其他部署字段写入，返回 422 READ_ONLY_SETTING。Vercel 环境变量变更需要重新部署。即使环境有 OIDC token，也只有显式配置 Gateway 密钥且打开后台开关后才会调用模型。

在 `/admin/settings` 启用规则判断与 Jev 复核。先测试再启用，默认仍关闭。模型只接收脱敏并截断的标题与正文，不接收访客 IP、昵称、邮箱字段或站点密钥。SDK 遥测未启用。发送到外部模型的内容仍可能包含无法自动识别的个人信息，因此只提交必要文字。

规则先判断：正常内容和明确垃圾不调用模型，只有可疑内容交给 Jev。默认每自然日（UTC）最多 500 次，最多 4000 个字符，超时 5 秒。应用侧关闭 SDK 重试；调用失败也计入限额。缺失密钥、超时、供应商错误、拒答、非法响应或预算耗尽时，保留留言进入人工审核。模型失败不影响公开故事阅读，也不导致有效留言丢失。

Boolean 答案是 P(垃圾)，与历史置信度独立保存。初始概率 ≥0.8 标垃圾，≤0.2 判正常，中间值待复核；阈值必须大于 0.5 且不大于 1。后台同时显示模型与垃圾概率；旧数据只显示历史置信度，不自动转换。上线前用带人工标签的中文反馈测试误判与漏判，再调整阈值。SDK 合约和本地业务测试使用模拟 Gateway 网络响应，不能代替真实模型准确率评估。

回滚到旧 AI 实现前，先关闭 AI，并确认目标应用使用环境变量读取密钥。执行环境配置清理迁移后，不能直接回滚到依赖后台密钥的应用。新增的概率列可以保留，避免删除已记录的判断。

## Resend 与队列

在 Resend 验证发信域名及 DNS。使用官方 `resend@6.34.1`；`@supabase/supabase-js@2.118.0`、`@supabase/ssr@0.12.7` 也已锁定在 Bun 锁文件中。

创建 webhook 指向 `https://站点/api/webhooks/resend`，订阅 `email.sent`、`email.delivered`、`email.failed`、`email.bounced`、`email.complained`、`email.suppressed`。将签名密钥写入 Vercel。请求体必须保持原始字节；SDK 验签后才会写库。数据库按 webhook ID 去重，投诉/退信优先级高于较早的成功事件。退信、投诉与被抑制的地址进入抑制表，后续邮件不会继续发送。

在 Supabase Vault 创建两个秘密：

```sql
select vault.create_secret('https://你的正式域名', 'biodays_site_url');
select vault.create_secret('与 Vercel BIODAYS_CRON_SECRET 相同的值', 'biodays_cron_secret');
```

然后执行 `supabase/cron.sql`，每分钟调用队列接口，每小时执行清理。此脚本重新执行会替换同名任务。预览不启用定时发送，可手动调用受保护的队列接口测试。

队列按事务领取并加租约，发送前固定收件人、发件人和正文；后续配置变更不会改变已尝试发送的载荷。单次请求逐项领取、最多处理 5 项，避免在一个函数中启动常驻 worker。可重试失败按 1、5、30、120 分钟间隔，最多 5 次。关闭发送不会消耗尝试次数。

Resend 的幂等键有效期为 24 小时，本系统在 23 小时后保守停止重试。网络结果不明、租约超时或幂等冲突均进入 `unknown`。**不能通过直接改回 pending 来重发**：先在 Resend 核对 message ID、发送记录、时间和收件人；已接受则补记 provider ID/完成状态并清空敏感载荷，无法核实时保持暂停或取消。只有已知失败且仍在窗口内的任务可以在后台重试。更换 API key 后，已尝试的任务也需先核查，避免跨账号幂等失效。发出后的邮件不能召回；退订会取消尚未发送的订阅邮件。

成功邮件立即清空正文和收件人；成功/取消记录保留 30 天。超过 30 天的失败载荷清空，关闭超过 90 天的反馈删除并留下不含正文与联系方式的删除记录。确认令牌保存哈希，确认链接 24 小时有效；退订链接 90 天有效。链接打开只显示按钮，不会直接确认或退订。

## 验收与回滚

正式切换前，在预览环境验证：

- 408 篇迁移内容与旧快照一致，原链接、404、站点地图、公式和移动端正常。
- 登录、匿名反馈、双重确认、状态通知、退订、配置冲突与越权拒绝均正常。
- 后台发送测试邮件到白名单邮箱，确认真实到达；Resend webhook 验签并显示投递状态。
- Vault/Cron 正常调用，关闭发送可暂停队列，Supabase 或 Resend 故障不影响公开故事阅读。

`/healthz` 检查应用存活；`/readyz` 分开报告内容与互动数据服务。后台邮件页显示积压及错误；同时检查 Supabase Cron 执行记录和 Vercel Functions 错误。`sent` 只表示 Resend 接受请求，最终接收情况通过 `delivery_status` 查看。

代码回滚使用 Vercel 的前一个部署；数据库保留反馈与邮件记录；环境配置迁移会清理历史部署字段，回滚该版本前需按上面的兼容要求处理。回滚到旧 Go 架构时，先关闭新 Cron 和 Resend 发送，恢复基线提交 `f1f1cb4` 对应的部署，避免两个系统同时发信。迁移上线后产生的反馈不会自动复制到旧架构，回滚期间保留 Supabase 项目和备份以便恢复。

本地回归使用隔离 Supabase；邮件发送请求使用网络模拟，webhook 使用真实 SDK 验签。它不能替代正式 Resend 域名、收件箱、Vercel Preview 和云端 Cron 验收。
