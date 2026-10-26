# biodays:PostgreSQL 双模式存储 + 管理员文章发布体系

## 背景与目标

现状:408 篇故事的源文件是 git 里的 `data/*.json`,构建镜像时烧成 SQLite 打进镜像层,**发布/修改一篇文章必须重建镜像**。管理员认证(cookie 会话 + CSRF)已存在但只覆盖反馈管理。

目标(按你的四个决策):
1. **双模式存储**:新增 `BIODAYS_DB_DSN`,设了就全部走 PostgreSQL(故事+反馈同库),不设保持现状 SQLite。本地 `go run` 与单测继续用内存 SQLite,零依赖。
2. **双认证的文章管理 API**:静态 `BIODAYS_ADMIN_API_TOKEN` 支持 `Authorization: Bearer` 头(脚本/curl 友好),网页端继续用现有 cookie 会话 + `X-CSRF-Token`。
3. **WordPress 式双入口**:网页管理界面(列表+编辑)+ 命令行(`scripts/stories.py publish`)。
4. **PG 全新启动**:不做存量数据迁移,故事由镜像内置 JSON 首次自举;旧 feedback.db 留在原卷里不用。

## A. 双模式存储(后端)

**`server/internal/store/store.go` + `feedback_store.go`**
- 新增 `store.OpenPostgres(dsn string)` / `store.OpenFeedbackPostgres(dsn string)`(`gorm.io/driver/postgres`,保持 `TranslateError: true`,错误映射 `ErrDuplicatedKey`→`ErrRequestConflict` 继续生效)。
- 保留现有 `Open(path)`/`OpenFeedback(path)`(SQLite);连接池按方言区分:PG 用 `SetMaxOpenConns(10)/SetMaxIdleConns(5)`,SQLite 维持现状。
- `feedback_store.go:71-85` 的 `PRAGMA index_info` 修复逻辑仅在 SQLite 模式执行(PG 下跳过,AutoMigrate 已建唯一索引)。
- `BackupTo`(`VACUUM INTO`,store.go:82、feedback_store.go:107)在 PG 模式返回明确错误"请使用 pg_dump";`biodays backup/restore` 子命令在 DSN 模式下同样报错退出。
- 其余 SQL 已确认全部 PG 兼容:`ON CONFLICT ... excluded.*`(feedback_store.go:714-739)、GORM `clause.OnConflict`、`LIKE ? ESCAPE '\'`、Go 侧随机采样、UTC 时间戳。

**`server/internal/config/config.go`**
- 新增 `DBDSN`(`BIODAYS_DB_DSN`,默认空=SQLite;校验必须以 `postgres://` 或 `postgresql://` 开头)。
- 新增 `AdminAPIToken`(`BIODAYS_ADMIN_API_TOKEN`,可选;设置时要求 ≥32 字符)。
- DSN 设置时忽略 `DBPath`/`FeedbackDBPath` 并打日志说明。

**`server/cmd/biodays/main.go`**
- `openStore`/`startFeedback`/`migrate`/`cleanup` 子命令按 DSN 分流打开对应 store(PG 模式下故事与反馈共用同一个 PG 库,表名无冲突)。
- **autoSeed 语义调整**(关键):PG 模式下仅当 `stories` 表为空时执行首播导入(否则管理员在网页/CLI 做的修改会被重启时的 JSON upsert 覆盖回 git 版本);SQLite 模式保持现状(每次启动幂等 upsert,因为镜像层无持久化)。

**Dockerfile**
- 运行时镜像增加 `COPY data/ /stories-data/`(JSON 很小),并设 `BIODAYS_DATA_DIR=/stories-data`,供 PG 首播自举;现有 data-build SQLite 阶段保留(支撑无 PG 部署)。

## B. 管理员文章 API + 双认证

**新中间件 `server/internal/middleware/admin_token.go`**
- `AdminOrSessionAuth(adminService, token)`:请求带 `Authorization: Bearer <token>` → 常量时间比对静态 token(`security.ConstantTimeEqual`),通过即放行(**跳过 Origin/CSRF 检查**——token 本身就是凭据,无 CSRF 面);无 Authorization 头 → 走现有 `AdminAuth`(cookie)+ 写操作 `RequireCSRF`。
- 这样 CLI/curl 不需要 Origin 头(现有 `RequireOrigin` 会拒掉无 Origin 的非 GET 请求,必须为 Bearer 请求豁免);网页端沿用现有安全链不变。
- token 作用域**仅限文章管理**,不能操作反馈/邮件(最小权限)。

**新服务 `server/internal/service/story_admin_service.go` + handler**
- 校验复用 `seed.validate`(seed.go:122-154):把 `inputStory`/`validate` 导出或抽到共享位置,id≤16 字符、合法日历日期、title/content/geek_quote 非空等规则与现有数据完全一致。
- 复用 `Store.Upsert`(story_store.go:186-221,幂等 upsert + 标签替换)与 `CleanupOrphanTags`。

**路由(挂在 `/api/v1/admin/stories`,NoStore)**
| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/admin/stories` | 管理列表(搜索/标签/月日/分页,summary 视图) |
| GET | `/admin/stories/:id` | 完整字段 |
| POST | `/admin/stories` | 单篇或 `{"stories":[...]}` 批量(≤200/次),upsert 语义,逐条返回 created/updated |
| PATCH | `/admin/stories/:id` | 部分更新(id 不可改) |
| DELETE | `/admin/stories/:id` | 删除 + 孤儿标签清理 |

- CORS `AllowHeaders` 增加 `Authorization`;swagger 注解 + 重新生成 `server/docs/`。
- 缓存说明:公开故事路由与 Next.js 缓存均为 300s,管理端修改后最多 5 分钟全站生效(可接受,不做主动 purge)。

## C. 网页管理界面(web)

- `web/lib/admin-api.ts` 新增 `listStories/getStory/upsertStory/updateStory/deleteStory`(继续走 cookie+CSRF)。
- 新页 `web/app/(private)/admin/stories/page.tsx`:列表(搜索、分页、标签筛选)+ 新建/编辑表单(id、月/日/年、标题、lab_item、标签逗号分隔、摘要、正文 textarea、geek_quote、source 三字段),删除带确认;新组件 `web/components/admin/story-manager.tsx`。
- 管理导航加"故事"入口;表单校验镜像服务端规则,服务端错误按字段回显(沿用反馈表单的 ValidationError 模式)。

## D. CLI 发布(`scripts/stories.py`)

- 新增 `publish` 子命令:`python3 scripts/stories.py publish --api https://站点 --token XXX [--file data/stories_q1.json | --id Q1-03 | --all]`,内部 `POST /api/v1/admin/stories` Bearer 认证;新增 `list` 便于命令行查看。
- git JSON 从"发布通道"降级为"首播语料 + 备份",`sync/manifest/check` 流程保留不动。

## E. 部署改造

**docker-compose.yml**
- 新增 `db` 服务:`postgres:17-alpine`,`POSTGRES_USER/PASSWORD/DB`(默认 biodays),`pg_isready` 健康检查,命名卷 `biodays-pg`;app `depends_on: db: service_healthy`,`BIODAYS_DB_DSN=postgres://biodays:${POSTGRES_PASSWORD}@db:5432/biodays`。
- 移除 `biodays-feedback` 卷挂载(反馈数据进 PG,全新开始)。
- 顺手落地之前讨论的精简:改用 `env_file: [.env]` 透传,`environment:` 只留镜像固定值与关键项。

**.env.example**:新增"数据库"段(`BIODAYS_DB_DSN`、`POSTGRES_PASSWORD`、`BIODAYS_ADMIN_API_TOKEN` 附 `openssl rand -base64 48` 说明)。

**README**:更新部署(PG 拓扑、备份改 pg_dump)与"持续更新故事"章节(新工作流:网页编辑或 `stories.py publish`,不再重建镜像)。

## 实施顺序

1. Go:config DSN/token → 双驱动 store open → PRAGMA/backup 门控 → 子命令分流 → autoSeed 语义。
2. Go:story admin service/handler/路由/中间件 → swagger 再生成。
3. Web:admin-api + stories 管理页 + 导航。
4. scripts/stories.py publish/list。
5. Dockerfile/compose/.env.example/README。
6. 验证(见下)。

## 测试与验证

- Go 单测(内存 SQLite,现有模式):story admin service 校验/upsert/删除、token 中间件(有效/无效/缺 token 时回落会话、Bearer 豁免 CSRF/Origin)、路由集成(会话与 Bearer 两条链)。
- PG 集成测试:由 `BIODAYS_TEST_PG_DSN` 环境变量门控(未设则 skip),覆盖 AutoMigrate、`ConsumeRateLimit` upsert、`CreateSubmission` 事务+挑战消费、story Upsert、index 修复跳过。
- `gofmt -l . && go vet ./... && go test -race -count=1 ./...`;`cd web && bun run typecheck && bun run lint && bun run build`;`python3 scripts/test_stories.py`;`docker compose config` 校验。
- 端到端冒烟(本地若可行):起 PG + `go run ./cmd/biodays serve` 用 DSN,验证首播自举、网页登录编辑一篇、`stories.py publish` 发布一篇、重启后修改仍在(不被 JSON 覆盖)。

## 风险与注意

- **autoSeed 语义是最大坑**:PG 模式必须"仅空库首播",否则重启会把管理员修改回滚成 git JSON——已有专项测试覆盖。
- Bearer token 等同发布权限,文档强调 ≥32 字节随机、只放服务端/CI secret;全局 60/min 限流兜底防爆破(熵足够,不额外加限制)。
- 公开缓存 300s 内新旧并存;`today`/`random` 是 no-store 即时生效。
- 双模式长期并存:SQLite 分支不删,测试主力仍在 SQLite,PG 靠门控集成测试保障。