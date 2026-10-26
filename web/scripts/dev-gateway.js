/**
 * Dev asset-path gateway for the CNB workspace preview.
 *
 * The CNB preview gateway (https://<id>-<port>.cnb.run) returns 403
 * Unauthorized for every URL containing the substring "/_next/static/"
 * (except its media/ subdirectory), which breaks every Next.js dev app:
 * all JS/CSS chunks live under /_next/static/chunks/.
 *
 * This tiny proxy sits in front of `next dev` and rewrites the path both
 * ways, so the browser never requests a URL the gateway rejects:
 *
 *   request:   /_nx/**            -> upstream /_next/**
 *   response:  body "/_next/"     -> "/_nx/"   (HTML + JS text)
 *   websocket: /_nx/webpack-hmr   -> upstream /_next/webpack-hmr (HMR)
 *
 * Run: bun scripts/dev-gateway.js
 * Expects `next dev` on 127.0.0.1:3100; listens on 0.0.0.0:3000.
 */

const UPSTREAM_HOST = process.env.UPSTREAM_HOST ?? "127.0.0.1";
const UPSTREAM_PORT = Number(process.env.UPSTREAM_PORT ?? 3100);
const LISTEN_PORT = Number(process.env.LISTEN_PORT ?? 3000);

const upstreamBase = `http://${UPSTREAM_HOST}:${UPSTREAM_PORT}`;

const server = Bun.serve({
  hostname: "0.0.0.0",
  port: LISTEN_PORT,
  idleTimeout: 255, // keep dev/HMR long requests alive

  fetch(request, srv) {
    // HMR websocket: upgrade first, the upstream link is established in
    // websocket.open where both sockets are available for piping.
    if (request.headers.get("upgrade")?.toLowerCase() === "websocket") {
      const url = new URL(request.url);
      const path = url.pathname.replace(/^\/_nx\//, "/_next/");
      const ok = srv.upgrade(request, { data: { path, query: url.search } });
      if (ok) return; // handled by the websocket callbacks below
      return new Response("websocket upgrade failed", { status: 500 });
    }

    const url = new URL(request.url);
    url.pathname = url.pathname.replace(/^\/_nx\//, "/_next/");

    const headers = new Headers(request.headers);
    headers.set("host", `${UPSTREAM_HOST}:${UPSTREAM_PORT}`);
    const upstream = new Request(`${upstreamBase}${url.pathname}${url.search}`, {
      method: request.method,
      headers,
      body: ["GET", "HEAD"].includes(request.method) ? undefined : request.body,
      redirect: "manual",
    });

    return fetch(upstream).then((response) => {
      const contentType = response.headers.get("content-type") ?? "";

      // Binary assets (fonts, images, wasm) pass through untouched. Only
      // rewrite textual payloads, where "/_next/" appears as a literal.
      const textual = /text\/|javascript|json|source-map/.test(contentType);
      if (!textual) {
        return new Response(response.body, response);
      }

      return response.text().then((body) => {
        const rewritten = body.replaceAll("/_next/", "/_nx/");
        const out = new Headers(response.headers);
        out.delete("content-length"); // rewritten bodies change length
        return new Response(rewritten, {
          status: response.status,
          statusText: response.statusText,
          headers: out,
        });
      });
    });
  },

  websocket: {
    open(ws) {
      const { path, query } = ws.data;
      const upstream = new WebSocket(
        `ws://${UPSTREAM_HOST}:${UPSTREAM_PORT}${path}${query ?? ""}`,
      );
      upstream.binaryType = "arraybuffer";
      upstream.onmessage = (event) => {
        try {
          ws.send(event.data);
        } catch {}
      };
      upstream.onclose = () => {
        try {
          ws.close();
        } catch {}
      };
      ws.data.upstream = upstream;
    },
    message(ws, message) {
      try {
        ws.data?.upstream?.send(message);
      } catch {}
    },
    close(ws) {
      try {
        ws.data?.upstream?.close();
      } catch {}
    },
  },
});

console.log(`dev-gateway listening on 0.0.0.0:${server.port} -> ${upstreamBase}`);
