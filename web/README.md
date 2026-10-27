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

生产使用 Node.js 24，构建自动验证 Markdown 并生成内容索引与字体。业务数据使用 Supabase；邮件使用官方 Resend SDK。环境变量见 `.env.example`。

完整说明见[项目 README](../README.md)和[部署文档](../docs/deployment.md)。
