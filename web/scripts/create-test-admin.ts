import { createClient } from "@supabase/supabase-js";

// This fixture never configures or grants access in a hosted Supabase project.
const url = process.env.SUPABASE_URL ?? "";
const key = process.env.SUPABASE_SECRET_KEY;
const email = process.env.BIODAYS_ADMIN_EMAIL ?? "admin@biodays.test";
if (
  !/^http:\/\/(127\.0\.0\.1|localhost):\d+$/.test(url) ||
  !key ||
  !email.endsWith("@biodays.test") ||
  process.env.BIODAYS_ADMIN_GITHUB_IDS !== "999000001"
)
  throw new Error("This fixture requires the isolated local test environment.");
const client = createClient(url, key, { auth: { persistSession: false, autoRefreshToken: false } });
const users = await client.auth.admin.listUsers({ page: 1, perPage: 1000 });
if (users.error) throw new Error("Cannot read local fixture users.");
let user = users.data.users.find((value) => value.email === email);
if (!user) {
  const result = await client.auth.admin.createUser({ email, email_confirm: true });
  if (result.error) throw new Error("Cannot create the local identity fixture.");
  user = result.data.user;
}
if (!/^[\da-f-]{36}$/.test(user.id)) throw new Error("Invalid fixture user ID.");
const sql = `insert into auth.identities
  (id, user_id, provider, provider_id, identity_data, created_at, updated_at, last_sign_in_at)
  select gen_random_uuid(), id, 'github', '999000001',
    '{"sub":"999000001","user_name":"local-admin-fixture"}'::jsonb, now(), now(), now()
  from auth.users where id = '${user.id}'
  on conflict (provider_id, provider) do update set
    user_id = excluded.user_id, identity_data = excluded.identity_data;`;
const child = Bun.spawn(
  [
    "docker",
    "exec",
    "-i",
    "supabase_db_biodays",
    "psql",
    "-U",
    "postgres",
    "-d",
    "postgres",
    "-v",
    "ON_ERROR_STOP=1",
    "-c",
    sql,
  ],
  { stdout: "pipe", stderr: "pipe" },
);
if ((await child.exited) !== 0) throw new Error("Cannot seed the isolated local GitHub identity.");
console.log("Local GitHub identity fixture ready; no production accounts or email were used.");
