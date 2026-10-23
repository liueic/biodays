// Container supervisor: starts the Go API (which migrates and seeds the
// database itself on boot), waits for it to become ready, then starts the
// Next.js standalone server. Both run as child processes; signals are
// forwarded symmetrically, and if either process dies the container exits.

const { spawn } = require("node:child_process");

const apiUrl = process.env.BIODAYS_API_URL || "http://127.0.0.1:8080";
const serverBin = process.env.BIODAYS_SERVER_BIN || "/usr/local/bin/biodays";
const nextEntry = process.env.BIODAYS_NEXT_ENTRY || "/app/server.js";

const children = new Set();
let shuttingDown = false;

function start(name, command, args) {
  console.log(`[launcher] starting ${name}: ${command} ${args.join(" ")}`);
  const child = spawn(command, args, { stdio: "inherit", env: process.env });
  children.add(child);
  child.on("exit", (code, signal) => {
    children.delete(child);
    if (shuttingDown) return;
    console.error(`[launcher] ${name} exited unexpectedly (code=${code}, signal=${signal})`);
    shutdown("SIGTERM", code ?? 1);
  });
  return child;
}

function shutdown(signal, exitCode = 0) {
  if (shuttingDown) return;
  shuttingDown = true;
  const waiters = [...children].map(
    (child) => new Promise((resolve) => child.once("exit", resolve)),
  );
  for (const child of children) child.kill(signal);
  // Escalate if a child ignores the graceful signal.
  const force = setTimeout(() => {
    for (const child of children) child.kill("SIGKILL");
  }, 5000);
  Promise.all(waiters).then(() => {
    clearTimeout(force);
    process.exit(exitCode);
  });
}

process.on("SIGTERM", () => shutdown("SIGTERM", 0));
process.on("SIGINT", () => shutdown("SIGINT", 0));

async function waitForApi() {
  const deadline = Date.now() + 60000;
  while (Date.now() < deadline) {
    try {
      const response = await fetch(`${apiUrl}/readyz`);
      if (response.ok) return;
    } catch {
      // API is still migrating/seeding/starting.
    }
    await new Promise((resolve) => setTimeout(resolve, 250));
  }
  throw new Error(`API did not become ready at ${apiUrl}`);
}

async function main() {
  // `serve` runs schema migration and the idempotent seed before listening,
  // so no separate seed step is needed here.
  start("api", serverBin, ["serve"]);
  await waitForApi();
  if (shuttingDown) return;
  start("web", process.execPath, [nextEntry]);
}

main().catch((error) => {
  console.error("[launcher]", error);
  shutdown("SIGTERM", 1);
});
