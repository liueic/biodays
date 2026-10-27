import { expect, test } from "bun:test";
import { verifyTurnstile } from "@/lib/backend/abuse";
import { check, db } from "@/lib/backend/db";
import {
  deploymentStatus,
  resendEnvironment,
  turnstileEnvironment,
} from "@/lib/backend/deployment";
import {
  defaults,
  jevConfiguration,
  mailConfiguration,
  normalizeSettings,
  publicSettings,
  saveSettings,
  settings,
} from "@/lib/backend/settings";

async function withEnvironment(
  values: Record<string, string | undefined>,
  action: () => void | Promise<void>,
) {
  const previous = Object.fromEntries(Object.keys(values).map((key) => [key, process.env[key]]));
  try {
    for (const [key, value] of Object.entries(values)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
    await action();
  } finally {
    for (const [key, value] of Object.entries(previous)) {
      if (value === undefined) delete process.env[key];
      else process.env[key] = value;
    }
  }
}

test("environment is authoritative even with legacy ciphertext and stale database addresses", async () => {
  await withEnvironment(
    {
      AI_GATEWAY_API_KEY: "gateway-current-environment",
      RESEND_API_KEY: "re_current_environment",
      RESEND_FROM: "Biodays <current@biodays.test>",
      BIODAYS_NOTIFY_TO: "admin@biodays.test",
      TURNSTILE_SITE_KEY: "current-site",
      TURNSTILE_SECRET_KEY: "current-turnstile",
      TURNSTILE_HOSTNAMES: "biodays.test, preview.biodays.test",
      BIODAYS_SETTINGS_ENC_KEY: undefined,
    },
    () => {
      const legacy = {
        ...defaults(),
        resend: {
          enabled: true,
          api_key: "v1:unreadable-old-key",
          from: "old@biodays.test",
          notify_to: "old-admin@biodays.test",
        },
        turnstile: {
          ...defaults().turnstile,
          enabled: true,
          secret_key: "v1:old-turnstile",
          site_key: "old-site",
          hostnames: ["old.test"],
        },
        ai_spam: { ...defaults().ai_spam, api_key: "v1:unreadable-old-gateway" },
      };
      const normalized = normalizeSettings(legacy);
      expect(normalized.resend).toEqual({ enabled: true });
      expect(jevConfiguration(normalized.ai_spam).apiKey).toBe("gateway-current-environment");
      expect(mailConfiguration(normalized)).toMatchObject({
        configured: true,
        enabled: true,
        apiKey: "re_current_environment",
        from: "Biodays <current@biodays.test>",
        notifyTo: "admin@biodays.test",
      });
      expect(turnstileEnvironment()).toMatchObject({
        configured: true,
        secretKey: "current-turnstile",
        siteKey: "current-site",
        hostnames: ["biodays.test", "preview.biodays.test"],
      });
      const visible = publicSettings({
        document: legacy,
        revision: 5,
        updated_at: "",
        updated_by: "test",
      });
      expect(JSON.stringify(visible)).not.toMatch(
        /gateway-current-environment|re_current_environment|current-turnstile|old@|v1:|old-site|old\.test/,
      );
      expect(visible.deployment.checks.find((v) => v.name === "AI_GATEWAY_API_KEY")).toEqual({
        name: "AI_GATEWAY_API_KEY",
        label: "AI Gateway API Key",
        status: "configured",
        secret: true,
      });
      expect(JSON.stringify(defaults())).not.toMatch(
        /api_key|secret_key|notify_to|hostnames|current/,
      );
      // A later deployment must not be pinned to the first bootstrap's values.
      process.env.RESEND_API_KEY = "re_new_deployment";
      process.env.RESEND_FROM = "new@biodays.test";
      expect(mailConfiguration(normalized)).toMatchObject({
        apiKey: "re_new_deployment",
        from: "new@biodays.test",
      });
      delete process.env.AI_GATEWAY_API_KEY;
      expect(jevConfiguration(normalized.ai_spam).configured).toBe(false);
    },
  );
});

test("configuration checks report missing and invalid environment without leaking secret values", async () => {
  await withEnvironment(
    {
      AI_GATEWAY_API_KEY: undefined,
      RESEND_API_KEY: undefined,
      RESEND_FROM: "not-an-address",
      BIODAYS_NOTIFY_TO: "invalid",
      TURNSTILE_SECRET_KEY: undefined,
      TURNSTILE_SITE_KEY: undefined,
      TURNSTILE_HOSTNAMES: "https://wrong.test/path",
      BIODAYS_SITE_URL: "javascript:alert(1)",
      SUPABASE_URL: "https://user:password@supabase.test",
      VERCEL_ENV: "preview",
      BIODAYS_MAIL_ALLOWLIST: undefined,
    },
    () => {
      expect(resendEnvironment()).toMatchObject({
        configured: false,
        missing: ["RESEND_API_KEY"],
        invalid: ["RESEND_FROM", "BIODAYS_NOTIFY_TO"],
      });
      expect(turnstileEnvironment().configured).toBe(false);
      const status = deploymentStatus();
      for (const name of ["RESEND_FROM", "TURNSTILE_HOSTNAMES", "SUPABASE_URL", "BIODAYS_SITE_URL"])
        expect(status.checks.find((c) => c.name === name)?.status).toBe("invalid");
      expect(status.checks.find((c) => c.name === "AI_GATEWAY_API_KEY")?.status).toBe("missing");
      expect(status.checks.find((c) => c.name === "BIODAYS_MAIL_ALLOWLIST")?.value).toContain(
        "暂停发送",
      );
      expect(JSON.stringify(status)).not.toContain("password");
      expect(() =>
        publicSettings({ document: defaults(), revision: 0, updated_at: "", updated_by: "test" }),
      ).not.toThrow();
    },
  );
});

test("Turnstile uses environment credentials, validates hostname and fails closed when configuration disappears", async () => {
  await withEnvironment(
    {
      TURNSTILE_SITE_KEY: "current-site",
      TURNSTILE_SECRET_KEY: "current-secret",
      TURNSTILE_HOSTNAMES: "biodays.test",
    },
    async () => {
      const config = defaults();
      config.turnstile.enabled = true;
      const originalFetch = globalThis.fetch;
      let hostname = "biodays.test",
        calls = 0;
      globalThis.fetch = (async (input, init) => {
        expect(String(input)).toBe("https://challenges.cloudflare.com/turnstile/v0/siteverify");
        expect(new URLSearchParams(String(init?.body)).get("secret")).toBe("current-secret");
        calls++;
        return Response.json({ success: true, hostname, action: "feedback" });
      }) as typeof fetch;
      try {
        const request = new Request("https://biodays.test/feedback");
        await verifyTurnstile("test-token", request, config);
        hostname = "wrong.test";
        await expect(verifyTurnstile("test-token", request, config)).rejects.toMatchObject({
          code: "TURNSTILE_FAILED",
        });
        await expect(verifyTurnstile(undefined, request, config)).rejects.toMatchObject({
          code: "TURNSTILE_REQUIRED",
        });
        delete process.env.TURNSTILE_SECRET_KEY;
        await expect(verifyTurnstile("test-token", request, config)).rejects.toMatchObject({
          code: "VERIFICATION_UNAVAILABLE",
        });
        expect(calls).toBe(2);
      } finally {
        globalThis.fetch = originalFetch;
      }
    },
  );
});

test.skipIf(!/^http:\/\/(127\.0\.0\.1|localhost):/.test(process.env.SUPABASE_URL ?? ""))(
  "read-only writes never change revision or audit; business saves never persist environment values",
  async () => {
    const previous = await settings();
    try {
      const initial = await settings();
      for (const patch of [
        { resend: { api_key: "attempted-secret" } },
        { resend: { from: "new@biodays.test" } },
        { resend: { notify_to: "new@biodays.test" } },
        { ai_spam: { api_key: "attempted-secret" } },
        { turnstile: { secret_key: "attempted-secret" } },
        { turnstile: { site_key: "new-site" } },
        { turnstile: { hostnames: ["new.test"] } },
        { deployment: {} },
      ])
        await expect(
          saveSettings({ base_revision: initial.revision, ...patch }, "environment-test"),
        ).rejects.toMatchObject({ code: "READ_ONLY_SETTING" });
      expect((await settings()).revision).toBe(initial.revision);
      const audit = await db()
        .from("biodays_settings_audit")
        .select("id", { count: "exact", head: true })
        .eq("actor", "environment-test");
      check(audit.error);
      expect(audit.count).toBe(0);
      await withEnvironment({ AI_GATEWAY_API_KEY: "private-environment-only-key" }, async () => {
        const result = await saveSettings(
          { base_revision: initial.revision, ai_spam: { daily_limit: 321 } },
          "environment-test",
        );
        expect(result.ai_spam.daily_limit).toBe(321);
        expect(result.revision).toBe(initial.revision + 1);
        const stored = await db()
          .from("biodays_settings")
          .select("document")
          .eq("id", true)
          .single();
        check(stored.error);
        expect(JSON.stringify(stored.data)).not.toMatch(
          /api_key|secret_key|private-environment-only-key|notify_to|hostnames/,
        );
        const audit = await db()
          .from("biodays_settings_audit")
          .select("changes")
          .eq("actor", "environment-test");
        expect(JSON.stringify(audit.data)).not.toContain("private-environment-only-key");
      });
    } finally {
      check(
        (await db().from("biodays_settings").update({ document: previous.document }).eq("id", true))
          .error,
      );
      check(
        (await db().from("biodays_settings_audit").delete().eq("actor", "environment-test")).error,
      );
    }
  },
);
