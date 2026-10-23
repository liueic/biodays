import { type NextRequest, NextResponse } from "next/server";

export function proxy(request: NextRequest) {
  // Read at request time: standalone next.config rewrites are frozen at build time.
  const origin = process.env.BIODAYS_API_URL || "http://127.0.0.1:8080";
  const destination = new URL(origin);
  destination.pathname = request.nextUrl.pathname;
  destination.search = request.nextUrl.search;
  return NextResponse.rewrite(destination);
}

export const config = {
  matcher: ["/api/:path*", "/healthz", "/readyz", "/swagger/:path*"],
};
