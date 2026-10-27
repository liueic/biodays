# Biodays Web

Next.js 应用包含公共阅读、匿名反馈、管理员后台和所有服务端 API。

```sh
bun install --frozen-lockfile
bun run dev
bun run build
bun run test
bun run typecheck
bun run lint
```

本地使用 Node.js 24.22.1，生产 / CI 使用 24.x，Vercel 构建日志会显示实际版本。构建自动验证 Markdown 并生成内容索引与字体。业务数据使用 Supabase；邮件使用官方 Resend SDK；可疑反馈通过官方 AI SDK 调用 Vercel AI Gateway 的 Jev 模型。环境变量见 `.env.example`。

开发、构建及本地启动都会检查实际 Node.js 大版本。如果终端仍是 22，请先启用仓库 `.prototools` 指定的运行时。

完整说明见[项目 README](../README.md)和[部署文档](../docs/deployment.md)。
