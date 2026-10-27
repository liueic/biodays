import { expect, test } from "bun:test";
import { heuristic, redact } from "@/lib/backend/abuse";
import { requireBearer, sameOrigin } from "@/lib/backend/auth";
import { normalizeSubmission, submissionHash } from "@/lib/backend/feedback";
import { failureStatus, message } from "@/lib/backend/mail";

const input = {
  request_id: "test-request-id-0001",
  kind: "issue",
  title: "标题",
  content: "这里是详细的生物学问题反馈内容",
  email: "Example@Email.com",
  subscribe_updates: true,
};
test("submission validation and canonical idempotency", () => {
  const normalized = normalizeSubmission(input);
  expect(normalized.email).toBe("example@email.com");
  expect(submissionHash(normalized)).toBe(
    submissionHash({ ...normalized, turnstile_token: "new challenge" }),
  );
  for (const patch of [
    { content: "short" },
    { email: "bad" },
    { source_url: "javascript:alert(1)" },
    { story_id: "missing" },
    { kind: "toString" },
  ])
    expect(() => normalizeSubmission({ ...input, ...patch })).toThrow();
});
test("HTML escapes input and uncertain delivery cannot silently retry", () => {
  expect(
    message("test", "test", "x@example.com", "test", "<script>alert(1)</script>").payload.html,
  ).not.toContain("<script>");
  const recent = new Date().toISOString();
  expect(failureStatus(429, recent)).toBe("pending");
  expect(failureStatus(500, recent)).toBe("pending");
  expect(failureStatus(422, recent)).toBe("failed");
  expect(failureStatus(undefined, recent)).toBe("unknown");
  expect(failureStatus(409, recent)).toBe("unknown");
  expect(failureStatus(500, new Date(Date.now() - 25 * 3600000).toISOString())).toBe("unknown");
});
test("abuse signals and AI redaction", () => {
  expect(heuristic({ title: "test", content: "x".repeat(30) }, 3).score).toBeGreaterThanOrEqual(60);
  expect(redact("email@example.com +8613912345678 https://x.test/path?secret=1", 100)).not.toMatch(
    /example.com|13912345678|secret/,
  );
});
test("origin and cron auth fail closed", () => {
  const old = process.env.BIODAYS_SITE_URL;
  delete process.env.BIODAYS_SITE_URL;
  expect(() =>
    sameOrigin(
      new Request("https://biodays.test/api", { headers: { Origin: "https://evil.test" } }),
    ),
  ).toThrow();
  expect(() =>
    sameOrigin(
      new Request("https://biodays.test/api", { headers: { Origin: "https://biodays.test" } }),
    ),
  ).not.toThrow();
  expect(() => requireBearer(new Request("https://biodays.test"), undefined)).toThrow();
  if (old) process.env.BIODAYS_SITE_URL = old;
});

test("legacy shortener signals remain available to the Jev review stage", () => {
  expect(
    heuristic({ title: "科学反馈", content: "这是一篇科学反馈，参考 https://buff.ly/example" })
      .signals,
  ).toContain("shortener_link");
});
