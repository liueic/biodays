import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  cacheComponents: false,
  // Dev server rejects cross-origin /_next/static requests (CSRF guard).
  // The CNB cloud workspace proxies ports through https://<id>-<port>.cnb.run,
  // so allow that domain family for local development only.
  allowedDevOrigins: ["https://*.cnb.run", "http://*.cnb.run"],
  turbopack: {
    root: process.cwd(),
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
  async headers() {
    const privateHeaders = [
      { key: "X-Robots-Tag", value: "noindex, nofollow" },
      { key: "Referrer-Policy", value: "no-referrer" },
      { key: "Cache-Control", value: "no-store" },
    ];
    return [
      { source: "/admin/:path*", headers: privateHeaders },
      { source: "/feedback", headers: [{ key: "Cache-Control", value: "no-store" }] },
      { source: "/feedback/confirm", headers: privateHeaders },
      { source: "/feedback/unsubscribe", headers: privateHeaders },
    ];
  },
};

export default nextConfig;
