# Biodays 匿名反馈、管理员收件箱与 SMTP 实施计划

日期：2026-10-24（Asia/Shanghai）。状态：已实施并完成本机验收；真实 SMTP 往返待部署方提供邮件服务商凭据。实施记录见第 15 节。

用户已确定：访客无需注册；提供管理员界面；接入 SMTP；第一版同时支持用户自愿订阅处理结果。

## 1. 产品约定

访客直接提交反馈，不创建用户账号，不设置密码。昵称是每条投稿的署名，邮箱是联系方式；两者都不作为用户身份或权限凭据。管理员使用独立的后台登录。

第一版交付匿名反馈页、单管理员收件箱、管理员新反馈邮件、用户订阅确认邮件、处理结果邮件与退订。投稿默认仅管理员可见；采纳后由维护者整理成正式故事或开发任务。正式故事仍通过现有仓库 JSON 和构建发布流程更新。

公开留言墙、评论、投票、附件上传、多人管理权限和在线编辑故事，留待后续根据实际使用情况评估。

## 2. 用户提交哪些信息

| 展示字段 | API 字段 | 必填 | 规则与用途 |
|---|---|---|---|
| 反馈类型 | `kind` | 是 | `topic` 想看话题、`resource` 趣味资源、`feature` 功能建议、`issue` 问题纠错 |
| 标题 | `title` | 是 | 去除首尾空白后 1–80 个 Unicode 字符 |
| 具体内容 | `content` | 是 | 去除首尾空白后 10–5000 个 Unicode 字符，按纯文本处理 |
| 资源链接 | `source_url` | 否 | 最长 2048 字节，仅接受合法 HTTP/HTTPS URL；仅保存，不由服务器抓取 |
| 怎么称呼你 | `nickname` | 否 | 最长 40 个 Unicode 字符；为空显示“匿名访客”，无唯一性要求 |
| 联系邮箱 | `email` | 条件必填 | 通常选填；勾选订阅时必填。填写时必须通过服务端格式校验 |
| 通过邮件接收处理结果 | `subscribe_updates` | 否 | 默认不勾选；需邮箱验证后才生效 |
| 关联故事 | `story_id` | 否 | 从故事纠错入口带入，显示关联故事；服务端校验，按字符串关联两个数据库 |

邮箱处理分三种情况：

1. 不留邮箱：完整支持匿名提交，成功后提供反馈编号。
2. 留邮箱、不订阅：仅供管理员就这条反馈联系；后台标记“邮箱未验证”，应用不自动发送邮件。
3. 留邮箱并订阅：投稿立即保存，发送确认邮件；用户确认后订阅这条反馈的处理结果。

邮箱去除首尾空白，使用标准解析器校验单一地址，拒绝控制字符和换行。保留本地部分，不擅自删除 `+tag` 或点；域名按约定规范化。格式有效和拥有邮箱是两个独立判断，订阅必须完成所有权确认。参考：[OWASP 邮箱验证说明](https://cheatsheetseries.owasp.org/cheatsheets/Email_Validation_and_Verification_Cheat_Sheet.html)。

系统自动生成反馈 ID、提交时间和处理状态。客户端生成随机 `request_id` 用于重复提交去重；访客无需填写。请求体上限 32 KiB。表单设置不会被正常用户或辅助技术误触的蜜罐字段，服务器对其校验。

页面说明文案：

> 无需注册，也可以匿名提交。昵称和邮箱选填。勾选邮件通知后，请到邮箱确认订阅；邮箱仅用于本条反馈的联系与通知。

## 3. 页面与入口

| 路由 | 页面职责 |
|---|---|
| `/feedback` | Win98“灵感信箱”窗口，提交表单、成功回执、发送确认邮件提示和重发入口 |
| `/feedback/confirm` | 从邮件进入，用户点击按钮确认订阅；处理有效、已确认、过期与无效状态 |
| `/feedback/unsubscribe` | 用户点击按钮退订；显示成功或已退订状态 |
| `/admin/login` | 管理员登录 |
| `/admin/feedback` | Outlook Express 风格收件箱，列表与详情联动 |
| `/admin/feedback/[id]` | 可直接打开的反馈详情，在手机上也能完整操作 |
| `/admin/mail` | SMTP 配置状态、邮件任务、失败原因、测试邮件与重试 |

公共入口放在桌面图标、开始菜单、窗口主菜单和关于页。故事末尾增加“纠错 / 补充线索”，指向 `/feedback?kind=issue&story_id=...`。只接受已定义的查询参数，不从 URL 填入邮箱或正文。

表单复用现有 Window、React95 控件及字体。昵称、邮箱放在“选填信息”区，订阅复选框始终清晰可见。具备字段级错误、字数提示、提交中状态和失败重试。失败时保留内存中的输入；默认不向 localStorage 保存邮箱和正文。

提交成功时显示反馈编号和订阅状态。未订阅用户不承诺主动通知；待确认用户提示“反馈已收到，请到邮箱确认订阅”。反馈编号不是访问凭证，第一版不提供按编号公开查询反馈详情的接口。

## 4. 管理员收件箱

列表默认按新到旧排列，每页 20 条，支持类型、状态、关键词筛选和分页。列表只显示摘要与联系方式存在标识；完整邮箱仅在受保护详情中显示。

详情包含原始投稿、关联故事、资源链接、邮箱验证/订阅状态、处理记录和邮件状态。管理员可修改状态、保存内部备注、填写对用户的处理说明、标记垃圾信息及删除记录。

反馈状态：`pending` 待处理、`reviewing` 处理中、`accepted` 已采纳、`completed` 已完成、`rejected` 未采纳、`spam` 垃圾信息。允许管理员重新打开记录至 `pending`；每次状态修改均记录事件。编辑接口携带 `version`，并发覆盖返回 409。

`internal_note` 只在后台展示；`public_reply` 专供用户通知。用户邮件只能引用 `public_reply` 和面向用户的状态描述。后台保存按钮显示“保存并通知订阅者”的实际结果；无已确认订阅时说明不会发信。同一状态的重复保存与只修改内部备注不会自动发信。需要再次通知时由管理员明确操作，并生成独立事件。

删除需二次确认，清理反馈关联的联系方式、订阅令牌、内容事件与未发送邮件。保留不含原文和邮箱的最小删除审计信息；已发出的邮件不在应用控制范围内。

## 5. 管理员认证

第一版为单管理员，账号名和密码哈希由部署配置提供；无公众注册入口。提供本地 `biodays admin hash-password` 工具，交互或从标准输入读取密码，生成哈希，避免把明文密码放进命令参数。

密码使用 bcrypt；在实现时明确处理其 72 字节输入上限。参考：[Go bcrypt 文档](https://pkg.go.dev/golang.org/x/crypto/bcrypt)。

登录成功后生成高熵随机会话令牌，数据库只存令牌哈希；Cookie 使用 HttpOnly、SameSite=Strict、Path=/，正式 HTTPS 环境使用 Secure，默认 8 小时绝对有效期。更换账号凭据后使已有会话失效，退出时立即撤销会话。参考：[OWASP 会话管理](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)。

认证由 Go 对每个管理 API 强制执行；前端重定向仅改善体验。管理写接口校验 Origin 和 CSRF 令牌；登录要求同源 JSON 请求，并单独限制失败尝试。令牌不进入 localStorage。管理界面和 API 返回 `Cache-Control: no-store`。

继续通过 Next 的同源代理访问 Go；无需为后台开启任意来源的携带 Cookie 跨域访问。现有公开 API 的 CORS 行为需保留；相关方法和自定义请求头按新增接口补齐。

## 6. 用户订阅与退订

一次订阅仅关联一条反馈，不是账号登录，也不订阅全站营销内容。`subscribe_updates` 默认 false；记录勾选时间和说明文案版本。

确认流程：

1. 创建反馈时，同事务保存反馈、管理员通知任务和订阅确认任务。
2. 确认邮件只包含站点说明及确认/忽略指引，不嵌入未验证投稿的自由文本。
3. 链接形如 `/feedback/confirm#token=...`。使用 URL fragment 避免令牌进入 HTTP URL 日志；页面读取后清除地址中的 fragment，并仅在内存保存。
4. GET 访问只展示页面；用户点击确认后 POST 令牌，避免邮件扫描器预访问触发状态修改。
5. 服务端校验用途、哈希、有效期及关联邮箱后，将订阅从 `pending_confirmation` 改为 `confirmed`。确认令牌默认 24 小时有效，单次使用，重复确认返回稳定结果。
6. 确认前反馈已变更的，确认后发送一次当前可通知状态；已经退订或已删除的记录不会重新激活。

确认令牌仅以哈希保存在令牌表；为了支持邮件重试，原始链接仅短期存在待发送任务的敏感 payload 中。任务发送成功后清除敏感 payload。重发确认会使旧确认令牌失效，接口统一响应且限制频率，不泄露邮箱与反馈的对应关系。

用户在已确认订阅后，于“已采纳、已完成、未采纳”这些状态变化收到邮件；处理中和内部备注变化不自动发信。标记垃圾信息不向投稿邮箱发送通知。结果邮件包含反馈编号、标题、状态、对用户的处理说明和退订入口。

退订链接使用单独用途的随机令牌，服务端只存哈希；保持有效至退订或关联记录被清理。页面确认按钮执行 POST。退订幂等，立即取消尚未开始发送的用户通知；worker 在发送前再次检查订阅状态。已经交给 SMTP 的邮件无法撤回。

## 7. SMTP 与持久化邮件任务

Go 侧新增 Mailer 接口和 SMTP 实现，采用 `github.com/wneessen/go-mail`，固定实施时验证的依赖版本。支持 587 + 强制 STARTTLS 或 465 + 隐式 TLS，校验证书，配置连接和协议读写超时。能力依据：[go-mail 官方项目](https://github.com/wneessen/go-mail)。

发件人固定为配置的站点地址。管理员新反馈通知仅发往配置的接收地址；后台测试邮件也只发往该地址。已确认用户才可成为结果通知的收件人。HTML 模板转义用户内容，同时提供纯文本版本；不使用用户邮箱作为 From。

反馈与 outbox 任务使用同一个反馈数据库事务保存；不在事务中连接 SMTP，也不持有数据库连接等待网络请求。所有事务内操作使用同一 `tx`。依据：[GORM 事务文档](https://gorm.io/docs/transactions.html)。

任务类型为 `admin_new_feedback`、`subscription_confirm`、`user_status_update`、`smtp_test`。worker 与 Go 服务同进程，单 worker 即可；定时扫描待发送任务，使用租约防止重复领取。服务退出时停止领取任务并限时等待，重启后检查过期租约。

任务状态为 `pending`、`sending`、`sent`、`failed`、`unknown`、`cancelled`。SMTP 明确拒收的永久错误直接失败；明确未被接受的暂时错误最多尝试 5 次，重试间隔 1、5、30、120 分钟。发送阶段断链或崩溃造成是否接受不确定时标记 `unknown`，后台显示原因并允许管理员明确重试。

使用唯一业务事件键避免应用重复排队；SMTP 不能承诺端到端恰好一次，重试不确定任务仍可能产生重复邮件。`sent` 表示 SMTP 已接受，不表示邮件已进入收件箱。仅收到 QUIT/关闭连接错误时，不能直接把已接受邮件当作未发送后无条件重试。

重试前检查反馈是否已删除、订阅是否仍有效、确认令牌是否过期及任务是否已经成功。过期确认邮件应重新生成确认任务，不能原样重发失效链接。SMTP 故障不会改变反馈提交成功的结果。

## 8. 存储与数据生命周期

| 数据 | 位置 | 生命周期 |
|---|---|---|
| 正式故事 | 现有 `/data/biodays.db` | 由 JSON 构建并随镜像发布 |
| 反馈、管理员会话、订阅、邮件任务 | 新增 `/var/lib/biodays/feedback.db` | 挂独立持久化卷，独立备份 |

新增独立 FeedbackStore，复用 SQLite 连接配置能力，按自身模型迁移。现有 `store.Open` 自动迁移 Story/Tag/Dataset，不能直接用于反馈库。使用 WAL、合理 busy timeout 和适合当前写入量的连接数；模型跨库用字符串 ID 关联，历史故事删除不级联删除用户反馈。

建议表结构：

| 表 | 核心字段 |
|---|---|
| `feedbacks` | `id`、唯一 `request_id`、`request_body_hash`、kind/title/content/source_url/story_id/nickname/email、status、internal_note/public_reply、version、subscription_status、consented_at/consent_version、confirmed_at/unsubscribed_at、created_at/updated_at/closed_at |
| `feedback_events` | feedback_id、事件序号、原状态/新状态、内部备注或公开处理说明快照、操作者、时间 |
| `admin_sessions` | token_hash、凭据版本标识、CSRF 令牌、创建/过期时间 |
| `subscription_tokens` | token_hash、feedback_id、用途、邮箱关联指纹、expires_at/consumed_at |
| `mail_outbox` | id、feedback_id/event_id、类型、唯一业务事件键、收件人、模板与敏感 payload、状态、attempts、next_attempt_at、lease_until、last_error、smtp_accepted_at、created_at |
| `abuse_buckets` | IP/邮箱的 HMAC 限流键、窗口、计数、过期时间；定期清理 |

索引至少覆盖反馈状态/类型/创建时间、邮件状态/下次尝试时间、令牌哈希和事件去重键。确认、退订、删除与邮件排队的相关修改同事务完成。

默认清理策略：已关闭反馈的邮箱和昵称保留 90 天后清除，并撤销订阅及令牌；邮件任务完成后立即清除敏感 payload，普通投递元数据保留 30 天。过期会话、确认令牌和限流桶定期清理。后台支持主动删除。请求日志不记录正文、完整邮箱、会话或订阅令牌；新增业务表不保存原始 IP。现有访问日志的 IP 与转发头也需按部署方日志保留规则管理。

持久化卷目录由镜像构建时创建并赋予 UID/GID 65532 写权限，首次空卷初始化需验证。只挂载新增反馈目录；现有 `/data` 随镜像更新。

备份采用 SQLite 一致性备份方式，例如 Online Backup API 或 `VACUUM INTO`，保留最近 7 天，并演练恢复。不能仅复制正在写入的主 DB 文件而忽略 WAL。方法依据：[SQLite 备份文档](https://sqlite.org/backup.html)。恢复备份后撤销管理员会话，对已发送状态可能回退的 outbox 暂停自动投递并人工核对，防止重复通知。

## 9. API 合同

统一前缀 `/api/v1`，新接口使用 `{data: ...}`，列表附 `{meta: {total, page, page_size}}`。错误延续 `{error: {code, message}}`；字段错误可选附 `fields`。所有反馈、订阅和管理响应使用 `no-store`，避免继承现有故事路由的公开缓存中间件。

| 方法 | 路径 | 权限 / 行为 |
|---|---|---|
| GET | `/feedback/options` | 公开；返回功能启用状态、订阅能力、类型和字段限制，不含 SMTP 凭据 |
| POST | `/feedback` | 匿名投稿；校验及写入成功首次返回 201 |
| POST | `/feedback/subscriptions/confirm` | 令牌确认订阅 |
| POST | `/feedback/subscriptions/resend` | feedback_id + email；统一返回 202，实际发送需匹配且通过频控 |
| POST | `/feedback/subscriptions/unsubscribe` | 专用令牌退订 |
| POST | `/admin/login` | 账号密码登录，设置 Cookie |
| POST | `/admin/logout` | 撤销会话并清 Cookie |
| GET | `/admin/me` | 查询管理员会话与 CSRF 令牌 |
| GET | `/admin/feedback` | 已认证；kind/status/q/page/page_size，page_size 上限 100 |
| GET | `/admin/feedback/:id` | 已认证；完整详情、处理记录和通知状态 |
| PATCH | `/admin/feedback/:id` | 已认证 + CSRF；status/internal_note/public_reply/version/notify_subscriber |
| DELETE | `/admin/feedback/:id` | 已认证 + CSRF；确认后清理关联数据 |
| GET | `/admin/mail/status` | 已认证；配置是否齐全、队列计数、最近错误，不回传密码 |
| GET | `/admin/mail/outbox` | 已认证；任务分页、筛选、脱敏摘要 |
| POST | `/admin/mail/test` | 已认证 + CSRF；仅向配置的管理员邮箱发送测试任务，返回 202 |
| POST | `/admin/mail/outbox/:id/retry` | 已认证 + CSRF；符合条件才重试，返回 202 |

投稿示例：

```json
{
  "request_id": "c0c32287-d305-49ea-a75d-f41ea6cb6140",
  "kind": "topic",
  "title": "想了解 GFP 的发现故事",
  "content": "希望讲讲荧光蛋白怎样从水母研究走向实验室应用。",
  "nickname": "小林",
  "email": "reader@example.com",
  "subscribe_updates": true
}
```

成功响应：

```json
{
  "data": {
    "id": "随机生成的反馈编号",
    "status": "pending",
    "subscription_status": "pending_confirmation"
  }
}
```

不订阅时 `subscription_status=none`。相同 request_id 和相同规范化内容的重试返回 200 与原回执，不重复建记录或邮件；相同 request_id 携带不同内容返回 409。

主要错误：400 INVALID_PARAM / INVALID_TOKEN，401 UNAUTHENTICATED，403 FORBIDDEN / CSRF_INVALID，404 FEEDBACK_NOT_FOUND，409 CONFLICT / IDEMPOTENCY_CONFLICT，410 TOKEN_EXPIRED，413 PAYLOAD_TOO_LARGE，429 RATE_LIMITED，503 FEEDBACK_UNAVAILABLE。SMTP 运行时故障通过后台投递状态体现，不使已成功保存的投稿返回失败。

## 10. 频控、页面隔离与抓取规则

在现有全站限流之外增加投稿与邮件触发频控。初始建议：每 IP 投稿 5 次/小时、20 次/天；确认邮件每收件人 3 次/小时、10 次/天，且同一反馈重发至少间隔 60 秒；管理员登录每 IP/账号 15 分钟最多 5 次失败。参数可配置，学校或实验室共享网络可根据日志调整。持久化邮箱/IP 限流键使用部署密钥 HMAC。

所有用户内容按文本展示，外链经过协议校验并设置安全打开属性。测试邮件与重试接口不允许请求方指定任意收件人。

当前根布局注入全站 Umami 统计与录制。计划以 `(site)` 与 `(private)` 路由组建立独立根布局，共享视觉 Shell；仅阅读区根布局加载第三方统计。`/feedback`、确认/退订页、`/admin/*` 均不加载统计或录制。跨根布局会完整重载，以结束之前阅读页已加载的录制脚本；同组阅读体验保留。已有页面 URL 不变，字体资源路径、not-found/error/loading 和 metadata 路由在迁移时一起核对。框架行为依据：[Next.js Route Groups](https://nextjs.org/docs/app/api-reference/file-conventions/route-groups)。

对 `/admin/*`、确认/退订页设置 noindex，并从 sitemap 排除；robots 增加 `/admin/`、确认和退订路径，实际访问权限仍由服务端控制。`/feedback` 可加入 sitemap。敏感页面设置 `Referrer-Policy: no-referrer`。

## 11. 配置与部署

| 环境变量 | 默认 / 说明 |
|---|---|
| `BIODAYS_FEEDBACK_ENABLED` | false；上线时显式设为 true，旧部署可先保持故事服务运行 |
| `BIODAYS_FEEDBACK_DB_PATH` | 本地 `./feedback.db`；容器 `/var/lib/biodays/feedback.db` |
| `BIODAYS_ADMIN_USERNAME` | 启用反馈时必填 |
| `BIODAYS_ADMIN_PASSWORD_HASH` | 启用反馈时必填；由哈希工具生成 |
| `BIODAYS_ADMIN_SESSION_TTL` | 8h |
| `BIODAYS_ABUSE_KEY_SECRET` | 启用反馈时必填，部署生成至少 32 字节随机密钥 |
| `BIODAYS_FEEDBACK_SUBSCRIPTIONS_ENABLED` | true；还需 SMTP 启用后才向用户提供订阅选项 |
| `BIODAYS_SMTP_ENABLED` | false；第一版生产上线设为 true |
| `BIODAYS_SMTP_HOST` | 邮件服务商主机 |
| `BIODAYS_SMTP_PORT` | 587，按服务商设置 |
| `BIODAYS_SMTP_TLS_MODE` | `starttls` 或 `implicit_tls`，禁止隐式降级 |
| `BIODAYS_SMTP_USERNAME` | SMTP 认证账号 |
| `BIODAYS_SMTP_PASSWORD` | SMTP 授权码或密码，由部署环境注入 |
| `BIODAYS_SMTP_FROM` | 站点固定发件地址 |
| `BIODAYS_FEEDBACK_NOTIFY_TO` | 管理员通知与测试邮件收件地址 |
| `BIODAYS_SMTP_TIMEOUT` | 10s，落实到连接与协议读写 deadline |
| `BIODAYS_SITE_URL` | 沿用现有配置；确认/退订链接使用该可信 HTTPS origin |

反馈关闭时 `/feedback/options` 返回 enabled=false，表单展示尚未启用，其他反馈及管理操作不可用；现有故事服务不受影响。启用时校验反馈库、管理员凭据和密钥；SMTP 显式启用时校验必填配置。运行期间 SMTP 无法连接则后台显示故障，worker 按策略处理。

SMTP 关闭时后台显示“邮件发送暂停”，普通反馈仍可保存并保留管理员通知任务；订阅选项不展示，服务端拒绝新请求中 `subscribe_updates=true` 的不支持组合。SMTP 恢复后继续处理仍有效的任务。

修改根 `.env.example`、docker-compose.yml、Dockerfile、本地开发说明和数据库忽略规则，新增 `server/feedback.db*`。SMTP、管理员密码哈希和限流密钥仅进入运行环境，不写 NEXT_PUBLIC 变量，不烘焙进镜像。

启用反馈后 `/readyz` 检查故事库与反馈库；SMTP 可达性不纳入站点 readiness，防止邮件故障造成整个站点反复重启。`/healthz` 继续只检查存活。发布不执行破坏性数据库迁移，镜像回滚保留反馈卷；迁移和还原前都先做一致性备份。

实际邮件联调前需部署方提供服务商、主机、端口/加密方式、认证方式、发件人和管理员接收地址；凭据在部署环境设置。这些信息影响最终联调，不阻塞页面、API、队列和测试实现。

## 12. 代码变更范围

以下为实施时的建议文件归属，不表示本次已创建功能代码。

| 范围 | 计划变更 |
|---|---|
| `server/internal/model/` | feedback、event、session、subscription_token、mail_outbox、abuse_bucket 模型与 DTO |
| `server/internal/store/` | FeedbackStore、事务、列表过滤、令牌校验/消费、邮件任务领取和迁移 |
| `server/internal/service/` | 投稿去重、状态事件、订阅确认/退订、管理员认证业务 |
| 新增 `server/internal/mailer/` | Mailer 接口、SMTP 实现、邮件模板和 worker |
| `server/internal/handler/` | 投稿、订阅、认证、管理和邮件接口；扩展错误 DTO |
| `server/internal/middleware/` | 管理员会话、CSRF、投稿/发信频控、敏感日志处理 |
| `server/internal/config/`、`server/cmd/biodays/` | 新配置、哈希工具、反馈库和 worker 生命周期、readiness、清理/备份工具 |
| `server/internal/router/`、`server/docs/` | 路由分组、缓存边界、CORS 调整、Swagger 注释及生成文档 |
| `web/app/` | 两组根布局、反馈/确认/退订页、后台登录/收件箱/邮件页；迁移现有阅读页面的目录位置 |
| `web/components/feedback/`、`web/components/admin/` | 表单、回执、订阅提示、收件箱、状态操作和邮件任务 UI |
| `web/components/win/` | 桌面图标、开始菜单和主菜单入口，共享 Win98 视觉外壳 |
| `web/lib/` | 支持 JSON 写入的 API client、管理会话与 CSRF 请求封装、公开反馈配置类型 |
| `web/app/robots.ts`、`web/app/sitemap.ts` | 索引与抓取规则 |
| 根部署文件、README、`web/README.md` | 持久化、SMTP、后台初始化、备份与运行说明 |

当前依赖实际为 Next.js 16.4.2、React 19.3.0、React95、TanStack Query、Go 1.27.3、Gin 1.12.0、GORM 1.31.2、SQLite；以 package.json/go.mod 和源码为准。现有旧 SPEC.md 中的 shadcn、旧部署说明和“无后台”目标不作为本需求约束。

## 13. 执行顺序与完成标准

| 阶段 | 工作 | 完成标准 |
|---|---|---|
| M1 数据基础 | 独立反馈库、模型/索引、配置、迁移、投稿 API、去重与频控 | 匿名反馈可落库；重复请求不重复建记录；SMTP 不参与请求事务 |
| M2 管理后台 | 管理员初始化、会话/CSRF、列表详情、状态/备注/删除、事件记录 | 未登录访问被拒绝；可完整处理一条反馈；内部备注不会进入用户输出 |
| M3 邮件与订阅 | SMTP、outbox、worker、确认/重发/退订、结果通知、后台邮件面板 | 邮箱确认前无结果邮件；确认后可通知；退订后停止；失败与不确定状态可查看处理 |
| M4 前端与入口 | Win98 表单、回执、订阅页面、后台 UI、桌面与故事入口、录制隔离 | 手机和桌面流程完整；空邮箱可提交；邮箱订阅不要求注册；敏感页无第三方录制 |
| M5 部署与验收 | 持久化卷、权限、备份恢复、文档、Swagger、CI 和容器联调 | 重建容器反馈仍在；新镜像故事更新正常；真实 SMTP 往返与退订成功 |

前后端实现围绕同一 API 合同推进。交付时更新文档中的实际路径和最终依赖版本。所有阶段均属于第一版，用户订阅不是留待后续的扩展项。

## 14. 验收与测试

后端采用现有 httptest、临时 SQLite 与可替换 Mailer/Clock，验证真实事务和状态变化；测试不向真实邮箱发信。

| 场景 | 预期 |
|---|---|
| 仅填写类型、标题、内容 | 成功提交，显示匿名访客，无订阅 |
| 只填昵称，或留邮箱不订阅 | 成功保存，无自动用户邮件 |
| 勾选订阅但没有邮箱 / 邮箱非法 | 前后端字段提示，服务端返回 400 |
| 勾选订阅且邮箱有效 | 反馈成功保存，管理员通知与确认任务原子创建 |
| 重复提交 / 请求超时后重试 | 同 request_id 同内容只有一条反馈及一组任务 |
| outbox 插入失败 | 整个投稿事务回滚，无半成功记录 |
| 未确认邮箱时管理员修改状态 | 无用户结果邮件 |
| GET 确认或退订链接 | 不修改订阅状态；按钮 POST 后才生效 |
| 确认令牌错误、过期、重复使用 | 明确且稳定的页面/API 结果；无越权 |
| 确认前已有处理结果 | 确认后只排队一次最新可通知状态 |
| 只修改内部备注 / 标记垃圾 | 不向用户发信；内部文本不会泄露 |
| 重复保存同一状态 / 并发编辑 | 不重复发信；旧 version 返回 409 |
| 退订、删除与 worker 并发 | 发送前复查有效性；未发送任务被取消 |
| SMTP 超时、认证失败、永久拒收 | 投稿仍成功；后台显示准确任务状态 |
| SMTP 接受后 QUIT 失败 / 发送时崩溃 | 不盲目重发；可显示 unknown 并人工重试 |
| 服务重启 | 待发送任务保留；过期租约进入明确恢复流程 |
| 未登录、会话过期、跨站管理请求 | 401/403，无敏感内容；注销后旧会话立即失效 |
| 错误账号密码连续尝试 | 独立频控生效，无凭据日志 |
| 关闭 SMTP / 关闭反馈功能 | 能力提示与 API 合同一致，现有故事服务仍可用 |
| 已加载录制的阅读页进入反馈或后台 | 完整导航隔离，敏感页无统计/录制请求 |
| 备份还原、镜像更新和容器重建 | 反馈保留；会话失效；邮件避免盲目重发；故事数据仍按镜像更新 |

质量门槛沿用仓库现有 Go vet、带 race 的 Go 测试、前端 typecheck、Biome、生产构建、数据清单检查及镜像构建。对匿名提交—确认订阅—后台处理—收到结果—退订这一完整链路补充浏览器验收。检查 360px 手机、桌面、键盘导航、错误提示与 Win98 视觉一致性。

新源码出现后再次检查知识图谱覆盖并按需要回退源码核验；本次项目图谱尚未成功索引，计划依据直接读取的当前源码及配置，未将图谱缺失作为功能不存在的唯一证据。


## 15. 实施记录（2026-10-24）

### 15.1 落地文件

| 范围 | 实际路径 |
|---|---|
| 模型与 DTO | `server/internal/model/feedback.go`、`feedback_dto.go` |
| 独立反馈库 | `server/internal/store/feedback_store.go`（WAL、事务、令牌、outbox 租约、频控、清理、备份） |
| 业务 | `server/internal/service/feedback_service.go`、`admin_service.go` |
| 邮件 | `server/internal/mailer/smtp.go`、`templates.go`、`worker.go` |
| 认证/频控中间件 | `server/internal/middleware/admin.go` |
| HTTP | `server/internal/handler/feedback_handler.go`、`admin_handler.go`、`mail_handler.go` |
| 路由/就绪 | `server/internal/router/router.go`、`server/internal/handler/health.go` |
| 配置 | `server/internal/config/config.go` |
| CLI | `server/cmd/biodays/main.go`：`hash-password` / `cleanup` / `backup` / `restore` |
| 前端布局 | `web/app/(site)/layout.tsx`、`web/app/(private)/layout.tsx`、`web/components/app-shell.tsx` |
| 前端页面 | `web/app/(private)/feedback/*`、`admin/*` |
| 前端组件 | `web/components/feedback/*`、`web/components/admin/*` |
| 前端请求层 | `web/lib/feedback-api.ts`、`admin-api.ts`、`feedback-labels.ts` |
| 入口 | `web/components/win/{desktop-icons,taskbar,window}.tsx`、`(site)/about/page.tsx`、`(site)/story/[id]/page.tsx` |
| 部署/文档 | `Dockerfile`、`docker-compose.yml`、`.env.example`、`.gitignore`、`.dockerignore`、`README.md`、`web/README.md` |
| Swagger | `server/docs/*` 已重新生成，包含全部反馈与管理端点 |

### 15.2 最终依赖版本

- `github.com/wneessen/go-mail v0.8.1`
- `golang.org/x/crypto v0.58.0`（bcrypt）
- `golang.org/x/term v0.47.0`（交互式密码输入）
- 其余按 `server/go.mod`：Go 1.27.3、Gin 1.12.0、GORM 1.31.2、`gorm.io/driver/sqlite` 1.6.0
- 前端按 `web/package.json`：Next.js 16.4.2、React 19.3.0、React95 10.0.6

### 15.3 验收证据

- `go vet ./...`、`go test -race -count=1 ./...` 全部通过；覆盖投稿去重/校验/频控、订阅确认/过期/重发、退订取消、管理员会话/CSRF/登录限流、outbox 重试/unknown/租约恢复、清理保留策略、备份恢复。
- 浏览器实测（本机 13000 端口，真实 Next 同源代理 + Go 服务）：
  - 匿名/带订阅投稿成功，失败保留输入后可重试；重发确认邮件返回统一 202；
  - 邮件 fragment 令牌页面读取后清除地址栏，GET 不改变状态，按钮 POST 后确认成功；重复确认稳定；
  - 管理员登录、收件箱筛选/分页、详情编辑、状态事件、内部备注与公开说明分离、保存并通知结果提示；
  - 退订后订阅状态为 `unsubscribed`，待发送结果邮件立即变为 `cancelled`；
  - 邮件面板显示 SMTP 状态、队列计数、失败原因与人工重试；SMTP 不可达时投稿仍成功。
  - 360px 视口下反馈表单、收件箱（状态/标题/打开）、详情页、邮件面板均无横向溢出且可完成操作；桌面 1280px 下双栏详情正常。
- 统计隔离：`/` 的 HTML 含 Umami 脚本；`/feedback`、`/admin/*` 的 HTML 不含 `ana.juniortree.com`。`(site)` 与 `(private)` 分属不同根布局，切换为整页重载。
- 抓取规则：`robots.txt` 排除 `/admin/`、`/feedback/confirm`、`/feedback/unsubscribe`；sitemap 包含 `/feedback`；敏感路由响应带 `X-Robots-Tag: noindex`、`Referrer-Policy: no-referrer`、`no-store`。
- 备份/恢复演练：`backup` 生成故事与反馈一致性快照；恢复反馈库后管理员会话清零、未完成邮件任务由 `failed` 变为 `unknown`；恢复故事库后 408 篇故事完整。`cleanup` 可重复执行。
- 浏览器验收中发现并修复两个真实缺陷：
  1. 同源 POST 被 gin-cors 拒绝：现在 `BIODAYS_SITE_URL` 的 origin 自动并入允许来源，`*` 行为保留；
  2. `feedback_events` 早期唯一索引只包含 `seq`，导致第二条投稿失败：已改为 `(feedback_id, seq)`，并在启动时检测/重建旧索引。

### 15.4 未完成的外部依赖

- 协议级 SMTP 往返已用本地 STARTTLS + AUTH 中继验证；接入真实服务商仍需部署方提供主机、端口/加密方式、认证账号、固定发件地址与管理员接收地址，并在目标环境验证 SPF/DKIM/退信与收件箱投递。连接失败场景另已验证：投稿仍成功、后台显示失败原因、任务保留且可人工重试。
- 本地 STARTTLS + AUTH SMTP 中继实测：四种邮件（订阅确认、管理员新反馈、处理结果、SMTP 测试）全部投递成功，outbox 状态均为 `sent`；确认邮件不含投稿自由文本；结果邮件只含公开处理说明，内部备注未泄露；退订后订阅状态为 `unsubscribed`。中继使用自签证书并通过 `SSL_CERT_FILE` 信任，验证了 TLS 强制、AUTH PLAIN、超时与投递路径。
- 容器联调已在本机完成：`docker build --platform linux/amd64` 成功；容器启用反馈并写入一条投稿后，删除并重建容器，命名卷中的反馈仍为 1 条；以运行用户 `65532` 直接执行 `biodays cleanup` 成功，证明空卷初始化后的目录可写。生产 CNB/Compose 的首次空卷与镜像回滚仍需在目标环境复核。
