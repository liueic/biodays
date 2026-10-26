"use client";

import { CriticalStopIcon } from "@/components/win/dialog-icons";

// Root-layout crashes replace the whole document, so this page must be fully
// self-contained: it ships its own <html>/<body> and inline Win98 styles
// instead of relying on the app shell, global CSS or React95.
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  return (
    <html lang="zh-CN">
      <body style={{ margin: 0, minHeight: "100svh", background: "#008080" }}>
        <div
          style={{
            display: "flex",
            justifyContent: "center",
            padding: "10vh 16px 0",
          }}
        >
          <div
            role="alert"
            style={{
              width: "100%",
              maxWidth: 440,
              background: "#c0c7c8",
              padding: 3,
              boxShadow:
                "inset -1px -1px #0a0a0a, inset 1px 1px #ffffff, inset -2px -2px #808080, inset 2px 2px #dfdfdf",
            }}
          >
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: 4,
                background: "linear-gradient(90deg, #000080, #1084d0)",
                color: "#fff",
                fontWeight: 700,
                fontSize: 14,
                padding: "3px 4px",
              }}
            >
              biodays
            </div>
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: 20,
                minHeight: 96,
                padding: "20px 24px",
              }}
            >
              <CriticalStopIcon size={32} />
              <div>
                <p style={{ margin: 0, fontWeight: 700 }}> biodays 遇到了严重错误</p>
                <p style={{ margin: "8px 0 0" }}>
                  页面框架崩溃了。可以重试；如果反复出现，请稍后再来。
                </p>
                {error.digest && (
                  <p style={{ margin: "8px 0 0", fontSize: 12, opacity: 0.7 }}>
                    代码：{error.digest}
                  </p>
                )}
              </div>
            </div>
            <div
              style={{
                display: "flex",
                justifyContent: "center",
                gap: 8,
                paddingBottom: 12,
              }}
            >
              <button
                type="button"
                onClick={reset}
                style={{
                  minWidth: 88,
                  padding: "4px 16px",
                  fontFamily: "inherit",
                  fontSize: 14,
                  background: "#c0c7c8",
                  boxShadow:
                    "inset -1px -1px #0a0a0a, inset 1px 1px #ffffff, inset -2px -2px #808080, inset 2px 2px #dfdfdf",
                }}
              >
                重试
              </button>
            </div>
          </div>
        </div>
      </body>
    </html>
  );
}
