# 匿名反馈反垃圾实施计划与记录

日期：2026-10-25  
状态：已实现并完成本机 Go/Bun 验证；Turnstile 与 Workers AI 默认关闭，待部署方提供凭据后灰度。

## 目标

在保留匿名投稿、`request_id` 幂等和现有邮件链路的前提下，建立分层反垃圾：

1. 自托管 PoW 作为大陆访客主验证路径。
2. Cloudflare Turnstile 作为海外入口可选增强；官方不支持中国大陆，前端加载/就绪超时 8 秒后回退 PoW。
3. 规则评分 + 自动隔离；AI 只复核“待复核”区间，不阻塞所有投稿。
4. 首次确认邮件按收件邮箱 + 全局日额度限制，重发另受重发额度约束；管理员逐条通知按小时封顶。
5. Workers AI 模型 ID 与请求形态通过环境变量切换。

## 请求顺序

`RateLimit(60/min)` → `NoStore` → `RequirePublicOrigin` → `Content-Type: application/json` → 幂等检查 → 验证尝试桶 → PoW/Turnstile → 投稿频控 → 规则评分 → 可选 AI → 单事务写入（含挑战消费）→ 邮件排队。

同一 `request_id` + 同内容的重试在验证前直接返回原回执；验证字段不进入 `normalizedSubmission`，现有正文哈希保持不变。

## 数据模型

- `feedbacks` 新增：`content_hash`、`email_domain`、`spam_score`、`spam_verdict`、`spam_signals`、`spam_source`、`spam_model`、`spam_confidence`、`verification_method`、`verified_at`、`confirmation_email`。全部带默认值，可被 AutoMigrate 直接添加。
- 新表 `feedback_challenges`：`id`、`ip_hash`、`salt`、`signature`、`difficulty`、`min_seconds`、`issued_at`、`expires_at`、`consumed_at`、`feedback_id`。挑战在 `CreateSubmission` 事务内消费，失败回滚不烧挑战。
- `Cleanup` 删除过期挑战，并在 90 天联系方式清理时清空 `email_domain`。

## 配置

主要变量见 `.env.example` 的“反垃圾”段。默认值：

- PoW：开启，难度 4，TTL 10m，最少等待 3s。
- Turnstile：关闭。
- 规则：0–29 正常、30–69 待复核、70–100 自动隔离。
- 确认邮件：1/小时、3/天/邮箱，全局 100/天。
- 管理员通知：10/小时。
- AI：关闭；模型 `@cf/cloudflare/clef-flash`，style `decision`，超时 2s，阈值 0.8，500 次/天。

模型切换示例：

```bash
# 默认：Clef-Flash 决策模型
BIODAYS_AI_SPAM_MODEL='@cf/cloudflare/clef-flash'
BIODAYS_AI_SPAM_STYLE=decision

# 中文聊天模型
BIODAYS_AI_SPAM_MODEL='@cf/qwen/qwen3-30b-a3b-fp8'
BIODAYS_AI_SPAM_STYLE=chat
```

## API

- `POST /api/v1/feedback/challenge`：签发一次性 PoW 挑战，`no-store`。
- `GET /api/v1/feedback/options`：新增 `verification`（pow/turnstile/ai_assisted）。
- `POST /api/v1/feedback`：新增 `challenge_id`、`challenge_nonce`、`turnstile_token`。
- 回执新增 `confirmation_email: none|queued|deferred`。
- 管理端列表/详情返回评分、判定、信号、来源、模型、置信度、验证方式与时间；自动 spam 可“恢复为待处理”。

## 前端

- PoW：`web/lib/feedback/pow-worker.ts` + `pow-solver.ts`，Worker 失败回退主线程分片计算。
- Turnstile：`web/lib/feedback/turnstile.ts`，runtime site key，8 秒加载超时；widget 明确失败不回退 PoW，只有加载失败/超时才回退。
- 表单：双重提交保护、挑战过期自动刷新并同 `request_id` 重试一次、失败后重置验证、deferred 回执文案、AI 隐私提示。
- 管理端：收件箱风险徽标；详情反垃圾 Fieldset 与恢复按钮。

## 验证

```bash
cd server
gofmt -l .
go vet ./...
go test -race -count=1 ./...

cd web
bun install --frozen-lockfile
bun run typecheck
bun run lint
bun run build
```

Go 单测覆盖：PoW 向量、挑战过期/重放/过快、幂等重试不消费挑战、事务回滚不消费、Turnstile 成功/失败/主机名、Clef decision 与 chat JSON 解析、AI 超时/错误/预算、启发式评分、自动隔离不发信、确认节流与管理员封顶。

## 部署与回滚

1. 用旧镜像执行 `biodays backup`，再部署新镜像；AutoMigrate 会加列并创建挑战表。
2. 首轮保持 AI/Turnstile 关闭，验证 PoW 投稿、评分字段和管理员通知。
3. 配置 Cloudflare 凭据后再开 AI；Turnstile 仅海外入口开启。
4. 回滚优先改环境变量并重启；旧二进制忽略新增列/表。数据损坏时用升级前快照 `biodays restore --kind feedback --force`。
5. `/readyz` 不依赖 Turnstile/Workers AI；外部服务故障不会导致容器重启。

## 残余风险

- 自动隔离存在误判，管理员需定期查看 spam 文件夹并恢复。
- 分布式唯一邮箱仍可能消耗确认邮件，受全局 100/天与 SMTP 配额约束，需要监控退信。
- Turnstile 在中国大陆不可用；不要把 PoW 关闭后再把 Turnstile 当唯一入口。
- Workers AI 免费额度为 10,000 Neurons/天；用真实样本评估准确率与消耗后再调高 `BIODAYS_AI_SPAM_DAILY_LIMIT`。
