import { expect, test } from "bun:test";
import { classifyWithJev, type GatewayFetch } from "@/lib/backend/jev";
import {
  defaults,
  encryptSecret,
  JEV_MODEL,
  jevConfiguration,
  normalizeSettings,
  publicSettings,
} from "@/lib/backend/settings";

function config() {
  process.env.BIODAYS_SETTINGS_ENC_KEY ??= Buffer.alloc(32, 7).toString("base64");
  return { ...defaults().ai_spam, enabled: true, api_key: encryptSecret("gateway-test-key") };
}
const answer = (probability: unknown) => ({
  model: JEV_MODEL,
  answers: { is_spam: { type: "boolean", probability } },
  usage: { inputTokens: 25, outputTokens: 0 },
});

test("official AI SDK sends a typed Boolean question to Gateway with a private key", async () => {
  const calls: { url: string; headers: Headers; body: Record<string, unknown> }[] = [];
  const fetcher: GatewayFetch = async (input, init) => {
    calls.push({
      url: String(input),
      headers: new Headers(init?.headers),
      body: JSON.parse(String(init?.body)),
    });
    return Response.json(answer(0.2));
  };
  expect(await classifyWithJev("脱敏后的科学反馈", config(), fetcher)).toEqual({
    probability: 0.2,
    model: JEV_MODEL,
  });
  expect(calls).toHaveLength(1);
  expect(calls[0].url).toBe("https://ai-gateway.vercel.sh/v4/ai/decision-model");
  expect(calls[0].headers.get("authorization")).toBe("Bearer gateway-test-key");
  expect(calls[0].headers.get("ai-model-id")).toBe(JEV_MODEL);
  expect(calls[0].body.stateParts).toEqual([
    { type: "json", value: { feedback: "脱敏后的科学反馈" } },
  ]);
  expect(calls[0].body.questions).toMatchObject({ is_spam: { type: "boolean" } });
});

test("SDK rejects missing, malformed, refused and out-of-range answers", async () => {
  for (const body of [
    answer(-0.1),
    answer(1.1),
    answer("0.9"),
    { answers: {} },
    { answers: { is_spam: { type: "refusal" } } },
  ]) {
    const fetcher: GatewayFetch = async () => Response.json(body);
    await expect(classifyWithJev("待判断反馈", config(), fetcher)).rejects.toThrow();
  }
});

test("transient provider errors make one attempt and cancellation reaches the transport", async () => {
  for (const status of [429, 503]) {
    let calls = 0;
    const fetcher: GatewayFetch = async () => {
      calls += 1;
      return Response.json({ error: { message: "test failure" } }, { status });
    };
    await expect(classifyWithJev("待判断反馈", config(), fetcher)).rejects.toThrow();
    expect(calls).toBe(1);
  }
  let aborted = false;
  const fetcher: GatewayFetch = async (_input, init) =>
    new Promise<Response>((_resolve, reject) => {
      init?.signal?.addEventListener(
        "abort",
        () => {
          aborted = true;
          reject(init.signal?.reason);
        },
        { once: true },
      );
    });
  await expect(
    classifyWithJev("待判断反馈", { ...config(), timeout_ms: 1000 }, fetcher),
  ).rejects.toThrow();
  expect(aborted).toBe(true);
});

test("legacy settings disable AI, preserve other settings and cannot leak Cloudflare credentials", () => {
  const original = {
    ...defaults(),
    subscriptions_enabled: false,
    ai_spam: {
      enabled: true,
      account_id: "legacy-account",
      api_token: "legacy-cloudflare-secret",
      style: "decision",
      model: "@cf/cloudflare/clef-flash",
      threshold: 0.4,
      daily_limit: 123,
    },
  };
  const normalized = normalizeSettings(original);
  expect(normalized.ai_spam).toMatchObject({
    enabled: false,
    model: JEV_MODEL,
    threshold: 0.8,
    daily_limit: 123,
    api_key: "",
  });
  expect(normalized.subscriptions_enabled).toBe(false);
  expect(original.ai_spam.api_token).toBe("legacy-cloudflare-secret");
  const visible = publicSettings({
    revision: 17,
    document: original as unknown as ReturnType<typeof defaults>,
    updated_at: "",
    updated_by: "test",
  });
  expect(visible.revision).toBe(17);
  expect(JSON.stringify(visible)).not.toMatch(/legacy-cloudflare-secret|api_token|account_id/);
});

test("saved Gateway key overrides environment, unreadable keys never fall back, secrets are redacted", () => {
  const old = process.env.AI_GATEWAY_API_KEY;
  process.env.AI_GATEWAY_API_KEY = "environment-test-key";
  try {
    const saved = config();
    expect(jevConfiguration(saved).apiKey).toBe("gateway-test-key");
    expect(jevConfiguration({ ...saved, api_key: "" }).apiKey).toBe("environment-test-key");
    expect(jevConfiguration({ ...saved, api_key: "v1:broken" }).configured).toBe(false);
    expect(jevConfiguration({ ...saved, model: "other/model" }).configured).toBe(false);
    const document = defaults();
    document.ai_spam = saved;
    const visible = publicSettings({ revision: 1, document, updated_at: "", updated_by: "test" });
    expect(JSON.stringify(visible)).not.toMatch(/gateway-test-key|environment-test-key|v1:/);
    expect(
      ((visible as Record<string, unknown>).ai_spam as { api_key: unknown }).api_key,
    ).toMatchObject({
      configured: true,
      encrypted: true,
      readable: true,
      source: "database",
    });
  } finally {
    if (old === undefined) delete process.env.AI_GATEWAY_API_KEY;
    else process.env.AI_GATEWAY_API_KEY = old;
  }
});
