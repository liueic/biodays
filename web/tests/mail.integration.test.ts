import { expect, test } from "bun:test";
import { db, newID, rpc } from "@/lib/backend/db";
import { message, processMail } from "@/lib/backend/mail";
import { defaults, settings } from "@/lib/backend/settings";

const local = /^http:\/\/(127\.0\.0\.1|localhost):/.test(process.env.SUPABASE_URL ?? "");
test.skipIf(!local)(
  "Resend SDK freezes content, retries stable key, clears PII and parks uncertain sends",
  async () => {
    const old = await settings();
    const doc = defaults();
    doc.resend.enabled = true;
    const oldFrom = process.env.RESEND_FROM,
      oldNotify = process.env.BIODAYS_NOTIFY_TO;
    process.env.RESEND_FROM = "sender@biodays.test";
    process.env.BIODAYS_NOTIFY_TO = "admin@biodays.test";
    const oldKey = process.env.RESEND_API_KEY;
    process.env.RESEND_API_KEY = "re_local_test";
    await db().from("biodays_settings").update({ document: doc }).eq("id", true);
    // Scope the queue to this dedicated local project's test, never a remote DB.
    await db().from("biodays_outbox").delete().neq("id", "");
    const task = message(
      "resend_test",
      `test:${newID()}`,
      "test@biodays.test",
      "test",
      "private content",
    );
    await rpc("enqueue", { p_mail: [task] });
    const originalFetch = globalThis.fetch,
      calls: { key: string | null; body: string }[] = [];
    let outcome: "retry" | "success" | "unknown" = "retry";
    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
      if (String(input).startsWith("https://api.resend.com/emails")) {
        calls.push({
          key: new Headers(init?.headers).get("idempotency-key"),
          body: String(init?.body),
        });
        if (outcome === "unknown") throw new Error("connection lost");
        return outcome === "retry"
          ? Response.json(
              { name: "rate_limit_exceeded", statusCode: 429, message: "rate limited" },
              { status: 429 },
            )
          : Response.json({ id: `provider-${task.id}` });
      }
      return originalFetch(input, init);
    }) as typeof fetch;
    try {
      await processMail();
      expect(
        (await db().from("biodays_outbox").select("status").eq("id", task.id).single()).data
          ?.status,
      ).toBe("pending");
      process.env.RESEND_FROM = "changed@biodays.test";
      await db().from("biodays_settings").update({ document: doc }).eq("id", true);
      await db()
        .from("biodays_outbox")
        .update({ next_attempt_at: new Date(0).toISOString() })
        .eq("id", task.id);
      outcome = "success";
      await processMail();
      expect(calls).toHaveLength(2);
      expect(calls[0]).toEqual(calls[1]);
      const sent = await db()
        .from("biodays_outbox")
        .select("status,payload,recipient,provider_message_id")
        .eq("id", task.id)
        .single();
      expect(sent.data).toEqual({
        status: "sent",
        payload: null,
        recipient: "",
        provider_message_id: `provider-${task.id}`,
      });
      const rotated = message(
        "resend_test",
        `test:${newID()}`,
        "test@biodays.test",
        "test",
        "payload",
      );
      await rpc("enqueue", { p_mail: [rotated] });
      outcome = "retry";
      await processMail();
      const beforeRotation = calls.length;
      process.env.RESEND_API_KEY = "re_different_account";
      await db()
        .from("biodays_outbox")
        .update({ next_attempt_at: new Date(0).toISOString() })
        .eq("id", rotated.id);
      await processMail();
      expect(calls).toHaveLength(beforeRotation);
      expect(
        (await db().from("biodays_outbox").select("status").eq("id", rotated.id).single()).data
          ?.status,
      ).toBe("unknown");
      process.env.RESEND_API_KEY = "re_local_test";
      const uncertain = message(
        "resend_test",
        `test:${newID()}`,
        "test@biodays.test",
        "test",
        "payload",
      );
      await rpc("enqueue", { p_mail: [uncertain] });
      outcome = "unknown";
      await processMail();
      expect(
        (await db().from("biodays_outbox").select("status").eq("id", uncertain.id).single()).data
          ?.status,
      ).toBe("unknown");
      const count = calls.length;
      await processMail();
      expect(calls).toHaveLength(count);
    } finally {
      globalThis.fetch = originalFetch;
      await db().from("biodays_outbox").delete().neq("id", "");
      await db().from("biodays_settings").update({ document: old.document }).eq("id", true);
      if (oldFrom === undefined) delete process.env.RESEND_FROM;
      else process.env.RESEND_FROM = oldFrom;
      if (oldNotify === undefined) delete process.env.BIODAYS_NOTIFY_TO;
      else process.env.BIODAYS_NOTIFY_TO = oldNotify;
      if (oldKey) process.env.RESEND_API_KEY = oldKey;
      else delete process.env.RESEND_API_KEY;
    }
  },
  20000,
);
