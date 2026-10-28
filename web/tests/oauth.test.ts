import { expect, test } from "bun:test";
import { AuthSessionMissingError, createClient, type User } from "@supabase/supabase-js";
import { admin, authOrigin } from "@/lib/backend/auth";

test("only a verified GitHub identity with an allowlisted immutable ID can administer", async () => {
  const previous = process.env.BIODAYS_ADMIN_GITHUB_IDS;
  process.env.BIODAYS_ADMIN_GITHUB_IDS = "69223323";
  let verified = true;
  const user: User = {
    id: crypto.randomUUID(),
    email: "admin@example.com",
    aud: "authenticated",
    created_at: new Date().toISOString(),
    app_metadata: { provider: "github" },
    user_metadata: { user_name: "liueic", provider_id: "69223323", role: "admin" },
    identities: [],
  };
  const identity = {
    id: "69223323",
    user_id: user.id,
    identity_id: crypto.randomUUID(),
    provider: "github",
    identity_data: { sub: "69223323", user_name: "liueic" },
  };
  const client = createClient("http://127.0.0.1:54321", "local-test-publishable-key", {
    auth: { persistSession: false, autoRefreshToken: false },
  });
  client.auth.getUser = async () =>
    verified
      ? { data: { user }, error: null }
      : { data: { user: null }, error: new AuthSessionMissingError() };
  try {
    // Names, email and user-editable metadata never grant access.
    await expect(admin(client)).rejects.toMatchObject({ status: 403, code: "FORBIDDEN" });
    user.identities = [{ ...identity, id: "999000002" }];
    await expect(admin(client)).rejects.toMatchObject({ status: 403 });
    user.identities = [{ ...identity, provider: "google" }];
    await expect(admin(client)).rejects.toMatchObject({ status: 403 });
    user.identities = [{ ...identity, user_id: crypto.randomUUID() }];
    await expect(admin(client)).rejects.toMatchObject({ status: 403 });
    user.identities = [identity];
    expect((await admin(client)).user.id).toBe(user.id);
    identity.identity_data.user_name = "renamed-account";
    expect((await admin(client)).user.id).toBe(user.id);
    process.env.BIODAYS_ADMIN_GITHUB_IDS = "999000002";
    await expect(admin(client)).rejects.toMatchObject({ status: 403 });
    process.env.BIODAYS_ADMIN_GITHUB_IDS = "";
    await expect(admin(client)).rejects.toMatchObject({ status: 503 });
    process.env.BIODAYS_ADMIN_GITHUB_IDS = "liueic";
    await expect(admin(client)).rejects.toMatchObject({ status: 503 });
    verified = false;
    await expect(admin(client)).rejects.toMatchObject({ status: 401 });
  } finally {
    if (previous === undefined) delete process.env.BIODAYS_ADMIN_GITHUB_IDS;
    else process.env.BIODAYS_ADMIN_GITHUB_IDS = previous;
  }
});

test("hosted authentication never redirects to a configured localhost", () => {
  const previousURL = process.env.BIODAYS_SITE_URL;
  const previousVercel = process.env.VERCEL;
  process.env.VERCEL = "1";
  const request = new Request("https://www.oneleaf.me/api/v1/admin/login/github");
  try {
    process.env.BIODAYS_SITE_URL = "http://localhost:3000";
    expect(() => authOrigin(request)).toThrow();
    process.env.BIODAYS_SITE_URL = "https://www.oneleaf.me/";
    expect(authOrigin(request)).toBe("https://www.oneleaf.me");
    delete process.env.BIODAYS_SITE_URL;
    expect(authOrigin(request)).toBe("https://www.oneleaf.me");
  } finally {
    if (previousURL === undefined) delete process.env.BIODAYS_SITE_URL;
    else process.env.BIODAYS_SITE_URL = previousURL;
    if (previousVercel === undefined) delete process.env.VERCEL;
    else process.env.VERCEL = previousVercel;
  }
});
