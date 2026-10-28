import { expect, test } from "bun:test";
import { createClient, type User } from "@supabase/supabase-js";
import { admin } from "@/lib/backend/auth";

test("a verified GitHub identity still needs database administrator membership", async () => {
  let membership: "missing" | "present" | "error" = "missing";
  let selectedUser = "";
  const user: User = {
    id: crypto.randomUUID(),
    email: "admin@example.com",
    aud: "authenticated",
    created_at: new Date().toISOString(),
    app_metadata: { provider: "github" },
    // User-editable metadata must never authorize an administrator.
    user_metadata: { role: "admin", is_admin: true },
  };
  const server = Bun.serve({
    hostname: "127.0.0.1",
    port: 0,
    fetch(request) {
      const url = new URL(request.url);
      if (url.pathname !== "/rest/v1/biodays_admins") return new Response(null, { status: 404 });
      selectedUser = url.searchParams.get("user_id") ?? "";
      if (membership === "error")
        return Response.json({ message: "Database unavailable" }, { status: 403 });
      return Response.json(membership === "present" ? [{ user_id: user.id }] : []);
    },
  });
  const previousURL = process.env.SUPABASE_URL;
  const previousKey = process.env.SUPABASE_SECRET_KEY;
  process.env.SUPABASE_URL = server.url.origin;
  process.env.SUPABASE_SECRET_KEY = "local-test-service-key";
  try {
    const client = createClient(server.url.origin, "local-test-publishable-key", {
      auth: { persistSession: false, autoRefreshToken: false },
    });
    client.auth.getUser = async () => ({ data: { user }, error: null });
    await expect(admin(client)).rejects.toMatchObject({ status: 403, code: "FORBIDDEN" });
    expect(selectedUser).toBe(`eq.${user.id}`);
    membership = "present";
    expect((await admin(client)).user.id).toBe(user.id);
    membership = "error";
    await expect(admin(client)).rejects.toMatchObject({ status: 503 });
  } finally {
    if (previousURL === undefined) delete process.env.SUPABASE_URL;
    else process.env.SUPABASE_URL = previousURL;
    if (previousKey === undefined) delete process.env.SUPABASE_SECRET_KEY;
    else process.env.SUPABASE_SECRET_KEY = previousKey;
    server.stop(true);
  }
});

test.skipIf(
  !/^http:\/\/(127\.0\.0\.1|localhost):/.test(process.env.SUPABASE_URL ?? "") ||
    !process.env.SUPABASE_SECRET_KEY,
)("GitHub-only bootstrap creates an authorized account without a site password", async () => {
  const client = createClient(
    process.env.SUPABASE_URL ?? "",
    process.env.SUPABASE_SECRET_KEY ?? "",
    { auth: { persistSession: false, autoRefreshToken: false } },
  );
  const email = `github-admin-${crypto.randomUUID()}@biodays.test`;
  let userID: string | undefined;
  try {
    const child = Bun.spawn([process.execPath, "run", "scripts/create-admin.ts"], {
      env: { ...process.env, BIODAYS_ADMIN_EMAIL: email, BIODAYS_ADMIN_PASSWORD: "" },
      stdout: "pipe",
      stderr: "pipe",
    });
    expect(await child.exited).toBe(0);
    const users = await client.auth.admin.listUsers({ page: 1, perPage: 1000 });
    expect(users.error).toBeNull();
    const user = users.data.users.find((value) => value.email === email);
    userID = user?.id;
    expect(user?.email_confirmed_at).toBeTruthy();
    expect(userID).toBeTruthy();
    const membership = await client
      .from("biodays_admins")
      .select("user_id")
      .eq("user_id", userID ?? "")
      .single();
    expect(membership.error).toBeNull();
    expect(membership.data?.user_id).toBe(userID);
    const passwordLogin = await client.auth.signInWithPassword({
      email,
      password: process.env.BIODAYS_ADMIN_PASSWORD || "NotAnAssignedPassword123!",
    });
    expect(passwordLogin.error).not.toBeNull();
  } finally {
    if (userID) await client.auth.admin.deleteUser(userID);
  }
});
