/// <reference lib="webworker" />

type SolveRequest = {
  salt: string;
  difficulty: number;
  start: number;
};

type SolveResponse =
  | { type: "progress"; attempts: number }
  | { type: "done"; nonce: string; attempts: number };

const scope = self as unknown as DedicatedWorkerGlobalScope;

function toHex(buffer: ArrayBuffer): string {
  return Array.from(new Uint8Array(buffer), (byte) => byte.toString(16).padStart(2, "0")).join("");
}

scope.onmessage = async (event: MessageEvent<SolveRequest>) => {
  const { salt, difficulty, start } = event.data;
  const target = "0".repeat(Math.max(1, difficulty));
  const encoder = new TextEncoder();
  let nonce = Number.isSafeInteger(start) && start >= 0 ? start : 0;

  while (true) {
    const digest = await crypto.subtle.digest("SHA-256", encoder.encode(`${salt}:${nonce}`));
    if (toHex(digest).startsWith(target)) {
      const response: SolveResponse = { type: "done", nonce: String(nonce), attempts: nonce + 1 };
      scope.postMessage(response);
      return;
    }
    nonce += 1;
    if (nonce % 2048 === 0) {
      const response: SolveResponse = { type: "progress", attempts: nonce };
      scope.postMessage(response);
    }
  }
};
