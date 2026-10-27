import { openapi } from "@/lib/openapi";
export async function GET(_request: Request, { params }: { params: Promise<{ file: string }> }) {
  const { file } = await params;
  if (file === "doc.json") return Response.json(openapi);
  if (file !== "index.html") return new Response("Not found", { status: 404 });
  const rows = Object.entries(openapi.paths)
    .flatMap(([path, methods]) =>
      Object.entries(methods)
        .filter(([method]) => ["get", "post", "patch", "delete"].includes(method))
        .map(
          ([method, value]) =>
            `<tr><td>${method.toUpperCase()}</td><td><code>${path}</code></td><td>${"summary" in value ? value.summary : ""}</td></tr>`,
        ),
    )
    .join("");
  return new Response(
    `<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Biodays API</title><style>body{font:16px system-ui;margin:2em;line-height:1.6}td{padding:8px;border-bottom:1px solid #ccc}code{overflow-wrap:anywhere}</style><h1>Biodays API</h1><p><a href="/about">关于</a> · <a href="/api/openapi">OpenAPI JSON</a></p><p>文章由 Markdown 构建。管理接口需要登录、同源请求和 X-CSRF-Token；运行设置使用 base_revision 防止覆盖。</p><table>${rows}</table></html>`,
    { headers: { "Content-Type": "text/html; charset=utf-8" } },
  );
}
