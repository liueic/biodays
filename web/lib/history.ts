// Serialize writes before a draw so a just-opened story is already excluded.
let pendingHistory: Promise<void> = Promise.resolve();

export function recordStoryView(id: string): Promise<void> {
  if (typeof window === "undefined") return Promise.resolve();
  pendingHistory = pendingHistory
    .then(async () => {
      await fetch("/api/v1/history", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ id }),
        credentials: "same-origin",
        cache: "no-store",
        keepalive: true,
      });
    })
    .catch(() => {
      // Reading remains available if cookies/network access are blocked.
    });
  return pendingHistory;
}

export function waitForHistory(): Promise<void> {
  return pendingHistory;
}
