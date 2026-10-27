import { FeedbackApiError } from "@/lib/feedback-api";

export { FeedbackApiError };

export type FeedbackStatus =
  | "pending"
  | "reviewing"
  | "accepted"
  | "completed"
  | "rejected"
  | "spam";

export type AdminSession = {
  username: string;
  csrf_token: string;
  expires_at: string;
};

export type FeedbackListItem = {
  id: string;
  kind: string;
  title: string;
  summary: string;
  status: FeedbackStatus;
  subscription_status: string;
  story_id?: string;
  has_email: boolean;
  has_nickname: boolean;
  spam_score: number;
  spam_verdict: string;
  version: number;
  created_at: string;
  updated_at: string;
};

export type FeedbackEvent = {
  seq: number;
  from_status: string;
  to_status: string;
  internal_note: string;
  public_reply: string;
  actor: string;
  created_at: string;
};

export type MailTask = {
  id: string;
  type: string;
  status: string;
  attempts: number;
  next_attempt_at: string;
  last_error?: string;
  provider_accepted_at?: string;
  provider_message_id?: string;
  delivery_status?: string;
  created_at: string;
  updated_at: string;
};

export type FeedbackDetail = {
  id: string;
  kind: string;
  title: string;
  content: string;
  source_url: string;
  story_id: string;
  nickname: string;
  email: string;
  email_verified: boolean;
  status: FeedbackStatus;
  internal_note: string;
  public_reply: string;
  version: number;
  subscription_status: string;
  consent_version: string;
  consented_at?: string;
  confirmed_at?: string;
  unsubscribed_at?: string;
  spam_score: number;
  spam_verdict: string;
  spam_signals: string[];
  spam_source: string;
  spam_model: string;
  spam_confidence: number;
  spam_probability: number | null;
  verification_method: string;
  verified_at?: string;
  created_at: string;
  updated_at: string;
  closed_at?: string;
  events: FeedbackEvent[];
  mail: MailTask[];
};

export type PageMeta = { total: number; page: number; page_size: number };

export type MailStatus = {
  resend_enabled: boolean;
  configured: boolean;
  missing: string[];
  notify_configured: boolean;
  error?: string;
  counts: Record<string, number>;
  recent_errors: MailTask[];
};

export type UpdateFeedbackInput = {
  status?: FeedbackStatus;
  internal_note?: string;
  public_reply?: string;
  version: number;
  notify_subscriber: boolean;
};

export type UpdateFeedbackResult = {
  data: FeedbackDetail;
  meta: { notified: boolean; notify_skipped_note?: string };
};

let csrfToken = "";

export function setCSRFToken(token: string) {
  csrfToken = token;
}

export function clearCSRFToken() {
  csrfToken = "";
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const method = (init.method ?? "GET").toUpperCase();
  const headers: Record<string, string> = {
    Accept: "application/json",
    ...(init.body ? { "Content-Type": "application/json" } : {}),
    ...((init.headers as Record<string, string> | undefined) ?? {}),
  };
  if (method !== "GET" && method !== "HEAD" && csrfToken) {
    headers["X-CSRF-Token"] = csrfToken;
  }
  const response = await fetch(`/api/v1${path}`, {
    ...init,
    method,
    headers,
    credentials: "same-origin",
    cache: "no-store",
  });
  let payload: unknown = null;
  try {
    payload = await response.json();
  } catch {
    payload = null;
  }
  if (!response.ok) {
    const error = (
      payload as {
        error?: { code?: string; message?: string; fields?: Record<string, string> };
      } | null
    )?.error;
    throw new FeedbackApiError(
      response.status,
      error?.code ?? "UNKNOWN",
      error?.message ?? `请求失败（${response.status}）`,
      error?.fields ?? {},
    );
  }
  return payload as T;
}

export async function login(username: string, password: string): Promise<AdminSession> {
  const response = await request<{ data: AdminSession }>("/admin/login", {
    method: "POST",
    body: JSON.stringify({ username, password }),
  });
  setCSRFToken(response.data.csrf_token);
  return response.data;
}

export async function logout(): Promise<void> {
  try {
    await request("/admin/logout", { method: "POST" });
  } finally {
    clearCSRFToken();
  }
}

export async function me(): Promise<AdminSession> {
  const response = await request<{ data: AdminSession }>("/admin/me");
  setCSRFToken(response.data.csrf_token);
  return response.data;
}

export function listFeedback(params: {
  kind?: string;
  status?: string;
  q?: string;
  page?: number;
  pageSize?: number;
}): Promise<{ data: FeedbackListItem[]; meta: PageMeta }> {
  const search = new URLSearchParams();
  if (params.kind) search.set("kind", params.kind);
  if (params.status) search.set("status", params.status);
  if (params.q) search.set("q", params.q);
  if (params.page) search.set("page", String(params.page));
  if (params.pageSize) search.set("page_size", String(params.pageSize));
  const query = search.toString();
  return request(`/admin/feedback${query ? `?${query}` : ""}`);
}

export function getFeedback(id: string): Promise<{ data: FeedbackDetail }> {
  return request(`/admin/feedback/${encodeURIComponent(id)}`);
}

export async function updateFeedback(
  id: string,
  input: UpdateFeedbackInput,
): Promise<UpdateFeedbackResult> {
  return request(`/admin/feedback/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export async function deleteFeedback(id: string): Promise<void> {
  await request(`/admin/feedback/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export function mailStatus(): Promise<{ data: MailStatus }> {
  return request("/admin/mail/status");
}

export function mailOutbox(params: {
  status?: string;
  type?: string;
  page?: number;
  pageSize?: number;
}): Promise<{ data: MailTask[]; meta: PageMeta }> {
  const search = new URLSearchParams();
  if (params.status) search.set("status", params.status);
  if (params.type) search.set("type", params.type);
  if (params.page) search.set("page", String(params.page));
  if (params.pageSize) search.set("page_size", String(params.pageSize));
  const query = search.toString();
  return request(`/admin/mail/outbox${query ? `?${query}` : ""}`);
}

export async function sendTestMail(): Promise<void> {
  await request("/admin/mail/test", { method: "POST" });
}

export async function retryMail(id: string): Promise<void> {
  await request(`/admin/mail/outbox/${encodeURIComponent(id)}/retry`, { method: "POST" });
}

// ── 运行时设置 ────────────────────────────────────────────────────────────

export type SecretState = {
  configured: boolean;
  encrypted: boolean;
  readable: boolean;
  source: "database" | "environment" | "none";
};

export type ResendSettings = {
  enabled: boolean;
  api_key: SecretState;
  from: string;
  notify_to: string;
};

export type TurnstileSettings = {
  enabled: boolean;
  site_key: string;
  hostnames: string[];
  timeout_ms: number;
  secret_key: SecretState;
};

export type SpamSettings = {
  enabled: boolean;
  review_score: number;
  block_score: number;
};

export type AISpamSettings = {
  enabled: boolean;
  model: string;
  timeout_ms: number;
  threshold: number;
  daily_limit: number;
  max_chars: number;
  api_key: SecretState;
};

export type RateLimitSettings = {
  submit_hourly: number;
  submit_daily: number;
  resend_hourly: number;
  resend_daily: number;
  confirm_hourly: number;
  confirm_daily: number;
  confirm_daily_global: number;
  admin_mail_hourly: number;
};

export type RuntimeSettings = {
  revision: number;
  updated_at: string;
  updated_by: string;
  encryption_key_configured: boolean;
  clear_sentinel: string;
  subscriptions_enabled: boolean;
  resend: ResendSettings;
  rate_limits: RateLimitSettings;
  turnstile: TurnstileSettings;
  spam: SpamSettings;
  ai_spam: AISpamSettings;
};

export type ResendPatch = Omit<ResendSettings, "api_key"> & { api_key?: string };
export type TurnstilePatch = Omit<TurnstileSettings, "secret_key" | "hostnames"> & {
  secret_key?: string;
  hostnames?: string[];
};
export type AISpamPatch = Omit<AISpamSettings, "api_key"> & { api_key?: string };

export type SettingsPatch = {
  base_revision: number;
  subscriptions_enabled?: boolean;
  resend?: ResendPatch;
  rate_limits?: RateLimitSettings;
  turnstile?: TurnstilePatch;
  spam?: SpamSettings;
  ai_spam?: AISpamPatch;
};

export type SettingsAuditRecord = {
  id: number;
  revision: number;
  actor: string;
  changes: string;
  created_at: string;
};

export function getSettings(): Promise<{ data: RuntimeSettings }> {
  return request("/admin/settings");
}

export async function updateSettings(patch: SettingsPatch): Promise<{ data: RuntimeSettings }> {
  return request("/admin/settings", { method: "PATCH", body: JSON.stringify(patch) });
}

export function listSettingsAudit(params: {
  page?: number;
  pageSize?: number;
}): Promise<{ data: SettingsAuditRecord[]; meta: PageMeta }> {
  const search = new URLSearchParams();
  if (params.page) search.set("page", String(params.page));
  if (params.pageSize) search.set("page_size", String(params.pageSize));
  const query = search.toString();
  return request(`/admin/settings/audit${query ? `?${query}` : ""}`);
}
