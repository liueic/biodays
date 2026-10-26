import type { PowChallenge } from "@/lib/feedback-api";

export type PowSolverHandle = {
  promise: Promise<string>;
  cancel: () => void;
};

type SolverOptions = {
  timeoutMs?: number;
  onProgress?: (attempts: number) => void;
};

function toHex(buffer: ArrayBuffer): string {
  return Array.from(new Uint8Array(buffer), (byte) => byte.toString(16).padStart(2, "0")).join("");
}

export function createPowSolver(
  challenge: PowChallenge,
  options: SolverOptions = {},
): PowSolverHandle {
  const timeoutMs = options.timeoutMs ?? 60_000;
  const target = "0".repeat(Math.max(1, challenge.difficulty));
  let cancelled = false;
  let settled = false;
  let worker: Worker | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;
  let rejectPromise: (reason?: unknown) => void = () => {};

  const cleanup = () => {
    if (timer) {
      clearTimeout(timer);
      timer = null;
    }
    worker?.terminate();
    worker = null;
  };

  const promise = new Promise<string>((resolve, reject) => {
    rejectPromise = reject;
    const finish = (nonce: string) => {
      if (settled || cancelled) return;
      settled = true;
      cleanup();
      resolve(nonce);
    };
    const fail = (reason: unknown) => {
      if (settled || cancelled) return;
      settled = true;
      cleanup();
      reject(reason);
    };

    if (typeof crypto === "undefined" || !crypto.subtle) {
      fail(new Error("pow_unavailable"));
      return;
    }

    timer = setTimeout(() => fail(new Error("pow_timeout")), timeoutMs);

    const solveOnMainThread = async () => {
      const encoder = new TextEncoder();
      for (let nonce = 0; ; nonce += 1) {
        if (cancelled) return;
        const digest = await crypto.subtle.digest(
          "SHA-256",
          encoder.encode(`${challenge.salt}:${nonce}`),
        );
        if (toHex(digest).startsWith(target)) {
          finish(String(nonce));
          return;
        }
        if (nonce > 0 && nonce % 2048 === 0) {
          options.onProgress?.(nonce);
          await new Promise((resolve) => setTimeout(resolve, 0));
        }
      }
    };

    try {
      worker = new Worker(new URL("./pow-worker.ts", import.meta.url), { type: "module" });
    } catch {
      void solveOnMainThread().catch(fail);
      return;
    }

    worker.onmessage = (
      event: MessageEvent<{ type: string; nonce?: string; attempts?: number }>,
    ) => {
      if (cancelled) return;
      if (event.data.type === "done" && event.data.nonce) {
        finish(event.data.nonce);
        return;
      }
      if (event.data.type === "progress" && typeof event.data.attempts === "number") {
        options.onProgress?.(event.data.attempts);
      }
    };
    worker.onerror = () => {
      worker?.terminate();
      worker = null;
      void solveOnMainThread().catch(fail);
    };
    worker.postMessage({ salt: challenge.salt, difficulty: challenge.difficulty, start: 0 });
  });

  return {
    promise,
    cancel: () => {
      if (settled) return;
      cancelled = true;
      cleanup();
      rejectPromise(new Error("pow_cancelled"));
    },
  };
}

export function waitForMinimumSubmit(challenge: PowChallenge): Promise<void> {
  const readyAt = new Date(challenge.issued_at).getTime() + challenge.min_submit_seconds * 1000;
  const delay = readyAt - Date.now();
  if (!Number.isFinite(delay) || delay <= 0) return Promise.resolve();
  return new Promise((resolve) => setTimeout(resolve, delay));
}
