// Uses the bundled Node runtime; works without a shell, wget, or curl.
const port = process.env.PORT || "3000";
fetch(`http://127.0.0.1:${port}/readyz`, { signal: AbortSignal.timeout(4000) })
  .then((response) => process.exit(response.ok ? 0 : 1))
  .catch(() => process.exit(1));
