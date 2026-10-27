import { expect, test } from "bun:test";
import { check, db, newID } from "@/lib/backend/db";
import { submit } from "@/lib/backend/feedback";
import {
  defaults,
  JEV_MODEL,
  publicSettings,
  saveSettings,
  settings,
} from "@/lib/backend/settings";

const local = /^http:\/\/(127\.0\.0\.1|localhost):/.test(process.env.SUPABASE_URL ?? "");

test.skipIf(!local)(
  "anonymous feedback uses real SDK probabilities, falls back safely, enforces budget and sees hot settings",
  async () => {
    const old = await settings();
    const oldKey = process.env.AI_GATEWAY_API_KEY;
    process.env.AI_GATEWAY_API_KEY = "gateway-first-key";
    const document = defaults();
    document.resend.enabled = false;
    document.ai_spam = {
      ...document.ai_spam,
      enabled: true,
    };
    check((await db().from("biodays_settings").update({ document }).eq("id", true)).error);
    check((await db().from("biodays_abuse").delete().neq("key_hash", "")).error);
    const originalFetch = globalThis.fetch;
    let probability = 0.5,
      providerStatus = 200;
    const calls: { key: string | null; model: string | null; body: string }[] = [];
    const ids: string[] = [];
    globalThis.fetch = (async (input, init) => {
      const url = String(input);
      if (url.startsWith("https://ai-gateway.vercel.sh/")) {
        const headers = new Headers(init?.headers);
        calls.push({
          key: headers.get("authorization"),
          model: headers.get("ai-model-id"),
          body: String(init?.body),
        });
        if (providerStatus === 0)
          return new Promise<Response>((_resolve, reject) => {
            init?.signal?.addEventListener("abort", () => reject(init.signal?.reason), {
              once: true,
            });
          });
        return providerStatus === 200
          ? Response.json({
              model: headers.get("ai-model-id"),
              answers: { is_spam: { type: "boolean", probability } },
            })
          : Response.json(
              { error: { message: "provider unavailable" } },
              { status: providerStatus },
            );
      }
      if (!url.startsWith(process.env.SUPABASE_URL ?? "http://127.0.0.1:"))
        throw new Error("Unexpected external request in local regression");
      return originalFetch(input, init);
    }) as typeof fetch;
    async function send(content?: string) {
      const id = newID();
      const result = await submit(
        {
          request_id: id,
          kind: "resource",
          title: `生物学资源 ${id}`,
          content:
            content ??
            "这是科学资源推荐。联系 reader@example.com 或 13912345678，参考 https://buff.ly/secret?token=private",
          subscribe_updates: false,
        },
        new Request("http://localhost/feedback", {
          headers: { "x-forwarded-for": id, "x-vercel-forwarded-for": id },
        }),
        (await settings()).document,
      );
      ids.push(String(result.data.id));
      const stored = await db()
        .from("biodays_feedback")
        .select("status,spam_verdict,spam_probability,spam_confidence,spam_source,spam_model")
        .eq("id", result.data.id)
        .single();
      check(stored.error);
      return stored.data;
    }
    try {
      for (const [value, verdict] of [
        [0, "allow"],
        [0.2, "allow"],
        [0.5, "review"],
        [0.8, "spam"],
        [1, "spam"],
      ] as const) {
        probability = value;
        expect(await send()).toMatchObject({
          status: verdict === "spam" ? "spam" : "pending",
          spam_verdict: verdict,
          spam_probability: value,
          spam_confidence: 0,
          spam_source: "ai",
          spam_model: JEV_MODEL,
        });
      }
      expect(calls).toHaveLength(5);
      expect(calls[0].body).not.toMatch(/reader@example.com|13912345678|token=private/);

      // Each submit reads the latest settings and creates a fresh SDK client.
      let current = await settings();
      await saveSettings(
        {
          base_revision: current.revision,
          ai_spam: { model: "typesafe-ai/jev", threshold: 0.95 },
        },
        "jev-hot-reload-test",
      );
      // Simulate a newly deployed environment; business edits never store this key.
      process.env.AI_GATEWAY_API_KEY = "gateway-rotated-key";
      probability = 0.9;
      expect(await send()).toMatchObject({ spam_verdict: "review", spam_model: "typesafe-ai/jev" });
      expect(calls.at(-1)).toMatchObject({
        key: "Bearer gateway-rotated-key",
        model: "typesafe-ai/jev",
      });

      providerStatus = 503;
      const beforeFailure = calls.length;
      expect(await send()).toMatchObject({
        status: "pending",
        spam_verdict: "review",
        spam_source: "ai_error",
        spam_probability: null,
      });
      expect(calls.length).toBe(beforeFailure + 1);

      current = await settings();
      await saveSettings(
        { base_revision: current.revision, ai_spam: { timeout_ms: 1000 } },
        "jev-hot-reload-test",
      );
      providerStatus = 0;
      const beforeTimeout = calls.length;
      expect(await send()).toMatchObject({
        status: "pending",
        spam_verdict: "review",
        spam_source: "ai_error",
        spam_probability: null,
      });
      expect(calls.length).toBe(beforeTimeout + 1);

      // Missing credentials leave the submission intact and spend no budget.
      current = await settings();
      await saveSettings(
        { base_revision: current.revision, ai_spam: { enabled: false } },
        "jev-hot-reload-test",
      );
      delete process.env.AI_GATEWAY_API_KEY;
      const unavailable = (await settings()).document;
      unavailable.ai_spam.enabled = true;
      check(
        (await db().from("biodays_settings").update({ document: unavailable }).eq("id", true))
          .error,
      );
      const beforeMissing = calls.length;
      expect(await send()).toMatchObject({ spam_source: "ai_unavailable", status: "pending" });
      expect(calls.length).toBe(beforeMissing);

      // Normal content and disabled review never contact a model.
      expect(await send("这是一条没有可疑信号的正常生物学问题反馈。")).toMatchObject({
        spam_source: "heuristic",
        spam_probability: null,
      });
      unavailable.ai_spam.enabled = false;
      check(
        (await db().from("biodays_settings").update({ document: unavailable }).eq("id", true))
          .error,
      );
      expect(await send()).toMatchObject({ spam_source: "heuristic", spam_verdict: "review" });
      expect(calls.length).toBe(beforeMissing);

      providerStatus = 200;
      process.env.AI_GATEWAY_API_KEY = "gateway-budget-key";
      current = await settings();
      await saveSettings(
        {
          base_revision: current.revision,
          ai_spam: { enabled: true, daily_limit: 1 },
        },
        "jev-hot-reload-test",
      );
      check((await db().from("biodays_abuse").delete().neq("key_hash", "")).error);
      expect(await send()).toMatchObject({ spam_source: "ai" });
      const beforeBudget = calls.length;
      expect(await send()).toMatchObject({
        spam_source: "ai_budget",
        spam_verdict: "review",
        spam_probability: null,
        status: "pending",
      });
      expect(calls.length).toBe(beforeBudget);
    } finally {
      globalThis.fetch = originalFetch;
      if (ids.length) await db().from("biodays_feedback").delete().in("id", ids);
      await db().from("biodays_settings").update({ document: old.document }).eq("id", true);
      await db().from("biodays_settings_audit").delete().eq("actor", "jev-hot-reload-test");
      if (oldKey === undefined) delete process.env.AI_GATEWAY_API_KEY;
      else process.env.AI_GATEWAY_API_KEY = oldKey;
    }
  },
  20000,
);

test.skipIf(!local)(
  "legacy settings migrate on CAS save, secrets stay private and bad thresholds fail",
  async () => {
    const old = await settings();
    const oldKey = process.env.AI_GATEWAY_API_KEY;
    process.env.AI_GATEWAY_API_KEY = "gateway-environment-key";
    const legacy = {
      ...defaults(),
      subscriptions_enabled: false,
      ai_spam: {
        enabled: true,
        model: "@cf/cloudflare/clef-flash",
        api_token: "v1:cloudflare-old-ciphertext",
        account_id: "old-account",
        style: "decision",
      },
    };
    check((await db().from("biodays_settings").update({ document: legacy }).eq("id", true)).error);
    try {
      const loaded = await settings();
      expect(loaded.document.ai_spam).toMatchObject({
        enabled: false,
        model: JEV_MODEL,
      });
      const raw = await db()
        .from("biodays_settings")
        .select("document,revision")
        .eq("id", true)
        .single();
      check(raw.error);
      expect(raw.data?.document.ai_spam.account_id).toBe("old-account");
      expect(raw.data?.revision).toBe(loaded.revision);
      expect(JSON.stringify(publicSettings(loaded))).not.toContain(legacy.ai_spam.api_token);
      for (const ai_spam of [
        { threshold: 0.5 },
        { threshold: 1.1 },
        { model: "unrelated/model" },
        { style: "chat" },
        { account_id: "legacy" },
      ]) {
        await expect(
          saveSettings({ base_revision: loaded.revision, ai_spam }, "jev-legacy-test"),
        ).rejects.toThrow();
      }
      const saves = await Promise.allSettled(
        [0.8, 0.9].map((threshold) =>
          saveSettings(
            { base_revision: loaded.revision, ai_spam: { enabled: true, threshold } },
            "jev-legacy-test",
          ),
        ),
      );
      expect(saves.map((r) => r.status).sort()).toEqual(["fulfilled", "rejected"]);
      const saved = await settings();
      expect(saved.revision).toBe(loaded.revision + 1);
      expect(saved.document.subscriptions_enabled).toBe(false);
      expect(saved.document.ai_spam.enabled).toBe(true);
      const serialized = JSON.stringify(publicSettings(saved));
      expect(serialized).not.toMatch(/cloudflare|gateway-environment-key|v1:/);
      const audit = await db()
        .from("biodays_settings_audit")
        .select("changes")
        .eq("actor", "jev-legacy-test");
      expect(JSON.stringify(audit.data)).not.toMatch(/gateway-environment-key|v1:/);
    } finally {
      await db().from("biodays_settings").update({ document: old.document }).eq("id", true);
      await db().from("biodays_settings_audit").delete().eq("actor", "jev-legacy-test");
      if (oldKey === undefined) delete process.env.AI_GATEWAY_API_KEY;
      else process.env.AI_GATEWAY_API_KEY = oldKey;
    }
  },
);
