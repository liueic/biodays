# 管理端设置中心与 PoW 移除实施计划

日期：2026-10-27（Asia/Shanghai）。状态：设计已与部署方确认，待实施。

已确认的决策：

1. **双平面配置**：环境变量管基础设施（端口、DSN、站点 URL 等启动级），数据库管运营设置（SMTP、频控、反滥用、订阅开关），管理端可编辑、保存后热生效，无需重启容器。
2. **v1 纳入管理端的设置组**：SMTP 全套、频控限额、反滥用（Turnstile / 垃圾评分 / Workers AI 复核）、订阅与通知开关。
3. **PoW 整体移除**（用户决策）：大陆访客将不再有交互式人机挑战，剩余防线为蜜罐、每 IP HMAC 频控、规则垃圾评分、可选 Workers AI 复核与管理邮件限额。频控/评分阈值移入管理端后热生效，可在被灌水时即时调整。
4. **密钥类设置落盘**：AES-256-GCM 加密，密钥来自独立环境变量 `BIODAYS_SETTINGS_ENC_KEY`（32 字节 base64），密文在数据库、密钥在 `.env`。密钥永不回显；未配置加密密钥时拒绝保存密钥项（可先保存非密钥项）。密文带 `v1:` 版本前缀，支持未来换钥。

## 1. 设置文档模型

`app_settings` 表新增一个 key：`runtime_settings`，value 是 JSON 文档。文档结构与服务层运行时快照一一对应：

```go
// internal/settings/settings.go
type Document struct {
    Revision     int             `json:"revision"`      // 乐观并发，PATCH 必带
    UpdatedAt    time.Time       `json:"updated_at"`
    UpdatedBy    string          `json:"updated_by"`
    SMTP         SMTPGroup       `json:"smtp"`
    RateLimits   RateLimitsGroup `json:"rate_limits"`
    Turnstile    TurnstileGroup  `json:"turnstile"`
    Spam         SpamGroup       `json:"spam"`
    AISpam       AISpamGroup     `json:"ai_spam"`
    Subscriptions bool           `json:"subscriptions_enabled"`
}

type SMTPGroup struct {
    Enabled   bool   `json:"enabled"`
    Host      string `json:"host"`
    Port      int    `json:"port"`
    TLSMode   string `json:"tls_mode"` // starttls | implicit_tls
    Username  string `json:"username"`
    Password  string `json:"password"` // 密文（v1:base64），API 永不回传
    From      string `json:"from"`
    NotifyTo  string `json:"notify_to"`
    TimeoutMS int    `json:"timeout_ms"`
}

type RateLimitsGroup struct {
    SubmitHourly, SubmitDaily             int
    ResendHourly, ResendDaily             int
    ConfirmHourly, ConfirmDaily           int
    ConfirmDailyGlobal, AdminMailHourly   int
}

type TurnstileGroup struct {
    Enabled   bool     `json:"enabled"`
    SiteKey   string   `json:"site_key"`   // 公开，回显
    SecretKey string   `json:"secret_key"` // 密文，不回显
    Hostnames []string `json:"hostnames"`
    TimeoutMS int      `json:"timeout_ms"`
}

type SpamGroup struct {
    Enabled     bool `json:"enabled"`
    ReviewScore int  `json:"review_score"`
    BlockScore  int  `json:"block_score"`
}

type AISpamGroup struct {
    Enabled    bool    `json:"enabled"`
    AccountID  string  `json:"account_id"`
    APIToken   string  `json:"api_token"` // 密文，不回显
    Model      string  `json:"model"`
    Style      string  `json:"style"` // decision | chat
    TimeoutMS  int     `json:"timeout_ms"`
    Threshold  float64 `json:"threshold"`
    DailyLimit int     `json:"daily_limit"`
    MaxChars   int     `json:"max_chars"`
}
```

来源语义：首次启动若 `runtime_settings` 不存在，用当前 env 有效值播种写入（已有部署升级后行为零变化）；此后 DB 值优先，env 同名项退化为首启默认。管理端展示每项的来源（`env 首启导入` / `数据库`），并在 env 与 DB 不一致时提示「.env 中的同名变量已被数据库设置覆盖，修改请在后台进行」。

## 2. 校验单一来源

现有 `config.validateFeedbackConfig` 中与运营设置相关的不变量抽出为共享纯函数（`internal/settings/validate.go`），env 加载路径（构造 Document 后校验）与 API 保存路径调用同一套：

- SMTP enabled 时 host/from/notify_to 必填且合法邮箱、port 1–65535、tls_mode 枚举、timeout 1–60s、username 非空则 password 非空；
- 频控各项 > 0 且上限宽松（≤ 10000）；
- spam：1 ≤ review < block ≤ 100，block ∈ [50,100]；
- turnstile enabled 时 site_key/secret_key 必填、timeout 1–10s、hostname 合法；
- ai_spam enabled 时要求 spam.enabled、account_id/token/model 必填、style 枚举、timeout 1–10s、threshold ∈ [0.5,1]、daily_limit 1–10000、max_chars 500–8000；
- 加密密钥缺失时，涉及密钥字段非空的保存返回 400（错误码 `SETTINGS_SECRET_UNWRITABLE`），非密钥项可正常保存。

保存路径额外规则：密钥字段空字符串 = 保持现值不变（write-only 语义）；显式清空用哨兵值 `"__clear__"`。

## 3. 热生效机制

- `internal/settings.Manager`：持有 `atomic.Pointer[Snapshot]`；`Snapshot()` 零拷贝返回；`Update()` 走 校验 → 解密合并（密钥空=保留旧值） → 事务写 DB（revision+1）→ 构建派生客户端 → 原子切换 → 写审计。
- `Snapshot` 内含派生客户端：`SMTPSender`（懒构建，构建失败降级为 errorSender 并在 status 暴露原因）、`TurnstileVerifier`、AI `Classifier`、`Judge`、`Renderer`（站点名/URL 固定，不随设置变）。这些客户端构造本就是纯函数，直接在快照切换时重建。
- `FeedbackService`、`AdminService` 不再持有固化 settings struct：`FeedbackSettings` 字段全部改为从 `manager.Snapshot()` 读取。本次改造是机械替换（约 40 处 `s.settings.X` → `snap().X`）。
- mail worker：`Worker.sender` 改为 `func() Sender`（或持有 manager 引用），每批任务取当前快照的 sender；SMTP 关闭时快照 sender 为 `pausedSender`（任务保持 pending，行为与现在关 SMTP 一致）。worker 生命周期不变——只要 feedback 子系统启动就常驻，由快照决定发不发。
- MailStatusDTO 的 `Configured/Missing` 改为按当前快照实时计算。

## 4. PoW 移除清单

后端删除：

- `internal/antispam/pow.go` + `pow_test.go`；
- `internal/service/challenge.go`（IssueChallenge/verifyPoW/challengeSignature/consumeIssueLimits）；
- `internal/store/feedback_store.go` 的 CreateChallenge/GetChallenge、`Cleanup` 中 challenges 清理、`model.FeedbackChallenge`（表保留在库里不 Drop，代码模型移除后 AutoMigrate 不再管理；历史库无影响）；
- `internal/service/verification.go` 重写：`consumeVerificationAttempts` 仅在 Turnstile 开启时消耗验证频控；`verifySubmission` 只处理 Turnstile 分支；
- `model.VerificationMethodPow` 常量与 DTO 中 pow_* 字段（`VerificationOptionsDTO` 精简为 turnstile + ai_assisted）；`ChallengeDTO` 删除；
- handler：`/api/v1/feedback/challenge` 路由与 `Challenge` 方法、submitRequest 的 `challenge_id/challenge_nonce`、错误码 CHALLENGE_*；
- `submitRequest`/`SubmitInput` 保留 `turnstile_token`；
- config：`BIODAYS_FEEDBACK_POW_*` 四项删除，相关校验删除；
- 历史数据：`feedback.verification_method` 里的既有 `"pow"` 值保留展示（前端标签映射保留「工作量证明」）。

前端删除：

- `web/lib/feedback/pow-solver.ts` 整文件；
- `feedback-form.tsx` 验证状态机精简为 none/turnstile 两态，移除 challenge 请求、PoW 重试码集合、min-seconds 等待（保留表单打开到提交的自然耗时供评分使用——`ElapsedSeconds` 输入信号在 Judge 的 `Score()` 中依赖 `in.ElapsedSeconds`，提交时由前端带上 `opened_at` 时间戳换算，-1 表示未知）；
- `lib/feedback-api.ts`：`PowChallenge` 类型与 `fetchPowChallenge` 删除，`VerificationOptions` 去 pow 字段，`SubmitFeedbackInput` 去 challenge 字段。

## 5. API 设计

全部挂现有 admin 组（Cookie 会话 + CSRF + Origin 校验，与 feedback 管理端一致）：

```
GET   /api/v1/admin/settings          # 分组返回有效值 + revision + 每组 updated_at/updated_by；
                                     # 密钥字段只返回 {configured: bool, encrypted: bool}，无明文
PATCH /api/v1/admin/settings          # body: {base_revision, smtp?, rate_limits?, turnstile?, spam?, ai_spam?, subscriptions_enabled?}
                                     # 未提供的组不动；密钥字段 null/缺省=不变，""=不变，"__clear__"=清除
                                     # 200 返回新文档；409 SETTINGS_REVISION_CONFLICT；400 带字段错误
GET   /api/v1/admin/settings/audit?page=&page_size=   # 审计记录，新到旧分页
```

审计表 `settings_audit`（GORM 模型 `model.SettingsAudit`）：id、revision、actor、changes（JSON：每字段 {old, new}，密钥字段恒为 "••••"）、created_at、索引 (id desc)。保留策略：`cleanup` 命令清 180 天前记录。

`GET /api/v1/feedback/options` 公共响应同步精简（去 pow 字段）。

## 6. 前端页面

`/admin/settings`（Win98 风格，与 mail-panel 一致的 Frame/表格语言）：

- 顶部：revision 提示 + 刷新 + 保存按钮（保存成功后刷新 revision）；
- 分组卡片：邮件 SMTP / 频控限额 / Turnstile / 垃圾评分 / Workers AI / 功能开关；
- 密钥输入框：`type=password`，placeholder 显示「已配置（加密存储）」或「未配置」，留空提交表示不变，提供「清除」小按钮（置 `__clear__`）；
- 409 冲突时提示「设置已被其他会话修改，请刷新后重试」，拒绝盲写；
- 底部：审计历史表格（时间 / 操作者 / 变更摘要 / revision），分页；
- 导航入口：mail-panel 顶部与 admin 各页统一的导航区加「设置」链接。

`/admin/mail` 页保留（队列视角），但顶部状态区改为读 settings 快照数据，加「在设置中编辑 SMTP」跳转。

## 7. 环境变量与部署文档变更

`.env.example`：

- 新增 `BIODAYS_SETTINGS_ENC_KEY`（可选，`openssl rand -base64 32`；不设则密钥项不可保存，其余设置可用）；
- SMTP/频控/Turnstile/Spam/AI 各段注明「首次启动导入数据库，之后请在 /admin/settings 管理；此处修改不生效」；
- 删除 PoW 四项。

`docker-compose.yml` 注释同步。SPEC.md 增补「运行时设置」一节（双平面模型说明）。README 管理后台一节加 /admin/settings。

## 8. 代码变更范围

后端新增：`internal/settings/{settings.go, validate.go, crypto.go, manager.go, manager_test.go, validate_test.go, crypto_test.go}`、`internal/handler/settings_handler.go`、`model.SettingsAudit`、store 的 settings 读写 + 审计写入/列表。
后端修改：`feedback_service.go`（快照化）、`verification.go`（去 PoW）、`admin_service.go`（注入 manager，不改逻辑）、`worker.go`（动态 sender）、`main.go`（装配：构造 manager、播种、注入）、`router.go`（新路由、删 challenge 路由）、`config.go`（去 PoW、新增 ENC_KEY）、删除 `antispam/pow.go`、`service/challenge.go`。
测试：新增 settings 单测（校验/加密 roundtrip/manager 并发 revision）；修改 antispam_flow/feedback_flow/router_test 等去 PoW 分支；swag 重新生成（`~/go/bin/swag init`）。

前端新增：`app/(private)/admin/settings/page.tsx`、`components/admin/settings-panel.tsx`、`lib/admin-api.ts` 增 settings 类型与 API。
前端修改：`feedback-form.tsx`、`feedback-api.ts`、`mail-panel.tsx`、删除 `pow-solver.ts`。

## 9. 执行顺序与完成标准

1. settings 包 + 单测（纯逻辑，先行）；
2. store 持久化 + 审计；
3. config/main 装配 + 播种逻辑（此时服务仍用旧固化路径，保持可运行）；
4. feedback_service/worker 快照化 + PoW 移除（一起做，因为 verification 重写共享）；
5. handler + 路由 + swag；
6. 前端两部分；
7. 文档 + 全量回归（`go test ./...`、`bun run typecheck && bun run lint && bun run build`）。

完成标准：后台改 SMTP 密码 → 发送测试邮件成功，全程无重启；改频控限额后新请求立即按新值限流；PoW 相关路由/字段全链路（API、DTO、前端、文档）不再出现；两处并发保存必有一方 409；密钥在任何 API 响应/日志/审计中不出现明文。

## 10. 残余风险

- **加密密钥丢失**：密钥项需重新录入（UI 显示「不可读，请重填」），其余设置不受影响。文档中明确。
- **单实例假设**：热生效基于进程内 atomic 快照，多副本部署时各副本独立切换、最终一致（revision 冲突检测仍有效）。当前单容器部署无此问题。
- **PoW 移除后滥用面**：频控+评分仍可热调，但无交互挑战。若后续需要恢复，git 历史可回溯。
