import type { NextConfig } from "next";

const apiOrigin = process.env.BIODAYS_API_URL ?? "http://localhost:8080";

const nextConfig: NextConfig = {
  output: "standalone",
  cacheComponents: false,
  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `${apiOrigin}/api/:path*`,
      },
      {
        source: "/healthz",
        destination: `${apiOrigin}/healthz`,
      },
      {
        source: "/readyz",
        destination: `${apiOrigin}/readyz`,
      },
      {
        source: "/swagger/:path*",
        destination: `${apiOrigin}/swagger/:path*`,
      },
    ];
  },
  turbopack: {
    root: process.cwd(),
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
};

export default nextConfig;
