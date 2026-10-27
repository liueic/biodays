const response = { description: "JSON response; errors use {error:{code,message,fields?}}" };
const operation = (summary: string) => ({
  summary,
  responses: {
    "200": response,
    "400": response,
    "401": response,
    "409": response,
    "503": response,
  },
});
export const openapi = {
  openapi: "3.1.0",
  info: {
    title: "Biodays API",
    version: "2.0.0",
    description:
      "Markdown stories and private feedback. Admin mutations require a Supabase session cookie, same-origin Origin and X-CSRF-Token from /admin/me. Story writes return 410; publish Markdown through Git.",
  },
  paths: {
    "/api/v1/today": {
      get: {
        ...operation("Today; unread preference in biodays_read_v1 cookie"),
        parameters: [
          { name: "tz", in: "query", schema: { type: "string", default: "+08:00" } },
          { name: "fallback", in: "query", schema: { type: "string", enum: ["random", "none"] } },
        ],
      },
    },
    "/api/v1/stories": {
      get: {
        ...operation("List; sorted month, day, year, id"),
        parameters: ["month", "day", "tag", "q", "page", "page_size", "view"].map((name) => ({
          name,
          in: "query",
          schema: { type: "string" },
        })),
      },
    },
    "/api/v1/stories/random": {
      get: {
        ...operation("Random unread story; starts a new round after exhaustion"),
        parameters: [
          {
            name: "exclude",
            in: "query",
            schema: { type: "string" },
            description: "Comma-separated IDs, max 100",
          },
        ],
      },
    },
    "/api/v1/stories/{id}": {
      get: {
        ...operation("Full story"),
        parameters: [{ name: "id", in: "path", required: true, schema: { type: "string" } }],
      },
    },
    "/api/v1/tags": { get: operation("Tags and counts") },
    "/api/v1/meta": { get: operation("Dataset coverage and version") },
    "/api/v1/history": {
      post: {
        ...operation("Record story ID in reader cookie; returns 204"),
        requestBody: {
          required: true,
          content: {
            "application/json": {
              schema: { type: "object", required: ["id"], properties: { id: { type: "string" } } },
            },
          },
        },
      },
    },
    "/api/v1/feedback/options": {
      get: operation("Feedback availability, limits and verification settings"),
    },
    "/api/v1/feedback": {
      post: {
        ...operation(
          "Submit feedback; same request_id and normalized content return original receipt; changed content returns 409",
        ),
        requestBody: {
          required: true,
          content: {
            "application/json": {
              schema: {
                type: "object",
                required: ["request_id", "kind", "title", "content", "subscribe_updates"],
                properties: {
                  request_id: { type: "string", minLength: 16, maxLength: 64 },
                  kind: { enum: ["topic", "resource", "feature", "issue"] },
                  title: { type: "string", minLength: 1, maxLength: 80 },
                  content: { type: "string", minLength: 10, maxLength: 5000 },
                  email: { type: "string" },
                  nickname: { type: "string", maxLength: 40 },
                  source_url: { type: "string" },
                  story_id: { type: "string" },
                  subscribe_updates: { type: "boolean" },
                  website: { type: "string" },
                  turnstile_token: { type: "string" },
                },
              },
            },
          },
        },
      },
    },
    ...Object.fromEntries(
      ["confirm", "unsubscribe", "resend"].map((action) => [
        `/api/v1/feedback/subscriptions/${action}`,
        {
          post: operation(
            action === "resend"
              ? "Resend confirmation with feedback_id and email; enumeration-safe response"
              : "Consume token from JSON {token}; GET never mutates subscription",
          ),
        },
      ]),
    ),
    ...Object.fromEntries(
      ["login", "logout"].map((action) => [
        `/api/v1/admin/${action}`,
        {
          post: operation(
            action === "login"
              ? "Email/password via {username,password}; returns CSRF token"
              : "End admin session",
          ),
        },
      ]),
    ),
    "/api/v1/admin/me": { get: operation("Current admin and CSRF token") },
    "/api/v1/admin/feedback": {
      get: operation("Private feedback inbox; kind,status,q,page,page_size"),
    },
    "/api/v1/admin/feedback/{id}": {
      parameters: [{ name: "id", in: "path", required: true, schema: { type: "string" } }],
      get: operation("Private detail including events and sanitized mail tasks"),
      patch: operation(
        "Update status, internal_note, public_reply; requires version and notify_subscriber",
      ),
      delete: operation("Delete feedback; refused while mail is in flight"),
    },
    "/api/v1/admin/settings": {
      get: operation(
        "Business settings and read-only environment configuration status; no secrets returned",
      ),
      patch: operation(
        "Save business settings with base_revision; deployment fields return 422, stale revision returns 409",
      ),
    },
    "/api/v1/admin/settings/audit": { get: operation("Settings audit; page,page_size") },
    "/api/v1/admin/mail/status": { get: operation("Resend availability and queue counts") },
    "/api/v1/admin/mail/outbox": { get: operation("Sanitized tasks; status,type,page,page_size") },
    "/api/v1/admin/mail/test": { post: operation("Queue test to configured admin inbox only") },
    "/api/v1/admin/mail/outbox/{id}/retry": {
      post: {
        ...operation("Retry known failures inside 23h; uncertain sends require reconciliation"),
        parameters: [{ name: "id", in: "path", required: true, schema: { type: "string" } }],
      },
    },
  },
};
