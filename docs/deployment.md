# Vercel + Supabase + Resend 部署

## 环境与发布

1. 在 Supabase 创建生产项目，以及独立的预览/测试项目。先在测试项目按文件名顺序执行 `supabase/migrations/*.sql`；已有项目只应用尚未执行的迁移。迁移仅管理 `biodays_` 前缀对象。
2. Supabase Auth 开启 Email 密码登录，关闭全局新用户注册。不要关闭 Email provider，否则预先创建的管理员也无法登录。本地 `config.toml` 的 `auth.enable_signup=false` 与 `auth.email.enable_signup=true` 正是这一区别。
3. 在 Vercel 连接 GitHub 仓库，Root Directory 设置为 `web`，Node.js 24，使用 `web/vercel.json`。本地固定 24.22.1，CI / Vercel 跟随 24.x 补丁，构建日志输出实际版本。文章修改合并后自动构建，全部故事详情和归档预生成，首页与互动接口动态运行。
4. 分别设置 Production 和 Preview 环境变量。`BIODAYS_SITE_URL` 必须是当前部署的访问源（协议和域名）；预览应使用自己的 URL，也可以不设置此变量，由 VERCEL_URL 自动解析，以便表单同源校验与邮件链接正确。不要让预览连接生产 Supabase。

| 变量 | 用途 |
|---|---|
| `BIODAYS_SITE_URL` | canonical、邮件链接与同源校验，生产使用 HTTPS |
| `SUPABASE_URL` | 项目 URL |
| `SUPABASE_PUBLISHABLE_KEY` | publishable key 或 anon key，用于服务端会话客户端 |
| `SUPABASE_SECRET_KEY` | secret key 或 service_role key，仅服务端，禁止 NEXT_PUBLIC 前缀 |
| `BIODAYS_SETTINGS_ENC_KEY` | 32 随机字节的 base64 或 64 位 hex，加密后台保存的密钥 |
| `BIODAYS_CRON_SECRET` | 队列接口的独立共享密钥，与 Vault 中相同 |
| `RESEND_WEBHOOK_SECRET` | Resend webhook 签名密钥 |
| `RESEND_ENABLED` | 首次配置默认开关，建议先 false |
| `RESEND_API_KEY` | 首次配置引用的 Resend API key，可随后通过后台加密保存替换 |
| `RESEND_FROM` | 已验证域名的发件地址，如 `Biodays <hello@example.com>` |
| `BIODAYS_NOTIFY_TO` | 管理员新反馈与测试邮件收件地址 |
| `BIODAYS_MAIL_ALLOWLIST` | 测试收件地址白名单，逗号分隔；预览未设置时拒绝发送 |
| `AI_GATEWAY_API_KEY` | Jev 服务端调用的备用密钥，后台未保存密钥时使用；不会自动启用 AI |

环境变量中的邮件参数只用于**首次创建**运行设置。已有设置以数据库为准，后续在 `/admin/settings` 修改，下一次相关请求生效。环境引用 `RESEND_API_KEY` 在服务端读取；后台写入新密钥需要有效的加密主密钥。清除密钥填写 `__clear__`，空值表示保留。加密主密钥丢失会使后台保存的密钥不可读取，需要重新配置；备份时单独保管。

在本地可信终端配置生产项目的服务端变量，通过 `BIODAYS_ADMIN_EMAIL`、`BIODAYS_ADMIN_PASSWORD` 执行 `cd web && bun run admin:create`。脚本不会输出密码。创建后移除密码变量。已有 Supabase 用户的 UUID 可由 SQL editor 加入 `biodays_admins`；用户 metadata 不授予权限。

## Jev 与可疑反馈复核

官方 `ai@7.0.154` 和 `zod@4.6.7` 已固定版本，使用 `experimental_decide` + `createGateway().decisionModel()`；实验接口可能在补丁版本变化，升级需重跑 SDK 合约测试。默认模型固定 `typesafe-ai/jev-1.14.0`，后台也允许 `typesafe-ai/jev` 或明确的 Jev 版本。固定版本可避免别名升级带来的判断变化。

先应用 `202610270002_jev.sql`，再部署新应用。它新增可空的 `spam_probability`，保留原置信度及其他数据。旧应用可以忽略新列；新应用的 `/readyz` 会检查该列是否存在。旧 Cloudflare AI 设置在新应用读取时兼容转换为关闭的 Jev 设置，保留合法的限额及超时；读取不写库、不递增版本，下一次管理员保存通过原有 CAS 和审计写入新格式。旧账号、模式与 token 不会回显或传给 Gateway。

在 Vercel AI Gateway 创建密钥并配置 `AI_GATEWAY_API_KEY`，或在后台加密保存密钥。后台密钥优先，清除后使用环境变量；已保存但不可读取的密钥不会静默切换账号。后台显示实际来源。后台设置下次请求生效，Vercel 环境变量变更需要重新部署。即使环境有 OIDC token，也只有显式配置 Gateway 密钥且打开后台开关后才会调用模型。

在 `/admin/settings` 启用规则判断与 Jev 复核。先测试再启用，默认仍关闭。模型只接收脱敏并截断的标题与正文，不接收访客 IP、昵称、邮箱字段或站点密钥。SDK 遥测未启用。发送到外部模型的内容仍可能包含无法自动识别的个人信息，因此只提交必要文字。

规则先判断：正常内容和明确垃圾不调用模型，只有可疑内容交给 Jev。默认每自然日（UTC）最多 500 次，最多 4000 个字符，超时 5 秒。应用侧关闭 SDK 重试；调用失败也计入限额。缺失密钥、超时、供应商错误、拒答、非法响应或预算耗尽时，保留留言进入人工审核。模型失败不影响公开故事阅读，也不导致有效留言丢失。

Boolean 答案是 P(垃圾)，与历史置信度独立保存。初始概率 ≥0.8 标垃圾，≤0.2 判正常，中间值待复核；阈值必须大于 0.5 且不大于 1。后台同时显示模型与垃圾概率；旧数据只显示历史置信度，不自动转换。上线前用带人工标签的中文反馈测试误判与漏判，再调整阈值。SDK 合约和本地业务测试使用模拟 Gateway 网络响应，不能代替真实模型准确率评估。

回滚到 Jev 接入前的应用时，先关闭 AI，并将 `ai_spam` 恢复成旧应用所需的字段形态，再回滚代码。可通过 Supabase SQL editor 执行以下带版本和审计的修改（保留其他设置；旧 AI 保持关闭）：

```sql
select public.biodays_save_settings(
  revision,
  jsonb_set(document, '{ai_spam}', '{"enabled":false,"account_id":"","model":"@cf/cloudflare/clef-flash","style":"decision","timeout_ms":5000,"threshold":0.8,"daily_limit":500,"max_chars":4000,"api_token":""}'::jsonb),
  'jev-rollback',
  '{"ai_spam":"disabled for pre-Jev application rollback"}'
) from public.biodays_settings where id;
```

新增概率列可以保留，避免删除已记录的判断。若回滚时有并发保存导致版本冲突，刷新当前设置后重试。

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

代码回滚使用 Vercel 的前一个部署；本轮数据库只有新增对象，没有旧运营数据迁移。回滚到旧 Go 架构时，先关闭新 Cron 和 Resend 发送，恢复基线提交 `f1f1cb4` 对应的部署，避免两个系统同时发信。迁移上线后产生的反馈不会自动复制到旧架构，回滚期间保留 Supabase 项目和备份以便恢复。

本地回归使用隔离 Supabase；邮件发送请求使用网络模拟，webhook 使用真实 SDK 验签。它不能替代正式 Resend 域名、收件箱、Vercel Preview 和云端 Cron 验收。
