import type { Page } from "@playwright/test";
import { createServerClient } from "@supabase/ssr";
import { createClient } from "@supabase/supabase-js";

// Issue a real local Supabase session without contacting GitHub or sending email.
// Only the separate test fixture inserts a simulated provider identity, in local Postgres.
export async function signInLocalIdentity(
  page: Page,
  email = process.env.BIODAYS_ADMIN_EMAIL ?? "admin@biodays.test",
) {
  const url = process.env.SUPABASE_URL ?? "";
  if (!/^http:\/\/(127\.0\.0\.1|localhost):/.test(url) || !email.endsWith("@biodays.test"))
    throw new Error("Identity fixtures are restricted to the isolated local Supabase project.");
  const service = createClient(url, process.env.SUPABASE_SECRET_KEY ?? "", {
    auth: { persistSession: false, autoRefreshToken: false },
  });
  const link = await service.auth.admin.generateLink({ type: "magiclink", email });
  if (link.error) throw new Error("Could not issue a local test session.");
  const auth = createServerClient(url, process.env.SUPABASE_PUBLISHABLE_KEY ?? "", {
    cookieOptions: { httpOnly: true, sameSite: "lax", secure: false, path: "/" },
    cookies: {
      getAll: () => [],
      setAll: async (values) => {
        await page.context().addCookies(
          values.map(({ name, value }) => ({
            name,
            value,
            domain: "localhost",
            path: "/",
            httpOnly: true,
            secure: false,
            sameSite: "Lax" as const,
          })),
        );
      },
    },
  });
  const result = await auth.auth.verifyOtp({
    token_hash: link.data.properties.hashed_token,
    type: "magiclink",
  });
  if (result.error || !result.data.user)
    throw new Error("Could not verify the local test session.");
  return result.data.user.id;
}
