import { createClient } from "@supabase/supabase-js";

const url = process.env.SUPABASE_URL,
  key = process.env.SUPABASE_SECRET_KEY;
const email = process.env.BIODAYS_ADMIN_EMAIL,
  password = process.env.BIODAYS_ADMIN_PASSWORD || undefined;
if (!url || !key || !email || (password !== undefined && password.length < 12))
  throw new Error(
    "Set SUPABASE_URL, SUPABASE_SECRET_KEY and BIODAYS_ADMIN_EMAIL. An optional BIODAYS_ADMIN_PASSWORD must have at least 12 characters; omit it for GitHub-only login.",
  );
const client = createClient(url, key, { auth: { persistSession: false, autoRefreshToken: false } });
const result = await client.auth.admin.createUser({ email, password, email_confirm: true });
if (result.error)
  throw new Error(
    "Admin creation failed. For an existing user, add its UUID to biodays_admins in the SQL editor.",
  );
const member = await client.from("biodays_admins").insert({ user_id: result.data.user.id });
if (member.error) {
  await client.auth.admin.deleteUser(result.data.user.id);
  throw new Error("Could not grant administrator membership. Apply the database migration first.");
}
console.log("Administrator created. Public sign-up must remain disabled in Supabase.");
