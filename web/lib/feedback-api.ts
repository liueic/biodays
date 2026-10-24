export type FeedbackKind = "topic" | "resource" | "feature" | "issue";

export type FeedbackOptions = {
  enabled: boolean;
  subscriptions_enabled: boolean;
  kinds: { value: FeedbackKind; label: string }[];
  limits: {
    title_min: number;
    title_max: number;
    content_min: number;
    content_max: number;
    nickname_max: number;
    source_url_max: number;
  };
};

export type FeedbackReceipt = {
  id: string;
  status: string;
  subscription_status: string;
  created_at: string;
};

export type SubscriptionResult = {
  subscription_status: string;
  result: "confirmed" | "already_confirmed" | "unsubscribed" | "already_unsubscribed";
};

export type SubmitFeedbackInput = {
  request_id: string;
  kind: FeedbackKind;
  title: string;
  content: string;
  source_url?: string;
  story_id?: string;
  nickname?: string;
  email?: string;
  subscribe_updates: boolean;
  website?: string;
};

export type FieldErrors = Record<string, string>;

export class FeedbackApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: FieldErrors;

  constructor(status: number, code: string, message: string, fields: FieldErrors = {}) {
    super(message);
    this.name = "FeedbackApiError";
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}

type ErrorPayload = {
  error?: { code?: string; message?: string; fields?: FieldErrors };
};

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers: Record<string, string> = {
    Accept: "application/json",
    ...(init.body ? { "Content-Type": "application/json" } : {}),
    ...((init.headers as Record<string, string> | undefined) ?? {}),
  };
  const response = await fetch(`/api/v1${path}`, {
    ...init,
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
    const error = (payload as ErrorPayload | null)?.error;
    throw new FeedbackApiError(
      response.status,
      error?.code ?? "UNKNOWN",
      error?.message ?? `请求失败（${response.status}）`,
      error?.fields ?? {},
    );
  }
  return payload as T;
}

export async function fetchFeedbackOptions(): Promise<FeedbackOptions> {
  const response = await request<{ data: FeedbackOptions }>("/feedback/options", { method: "GET" });
  return response.data;
}

export function newRequestID(): string {
  const cryptoObject = globalThis.crypto;
  if (typeof cryptoObject?.randomUUID === "function") {
    return cryptoObject.randomUUID();
  }
  const bytes = new Uint8Array(16);
  if (typeof cryptoObject?.getRandomValues === "function") {
    cryptoObject.getRandomValues(bytes);
  } else {
    for (let index = 0; index < bytes.length; index += 1) {
      bytes[index] = Math.floor(Math.random() * 256);
    }
  }
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, "0")).join("");
}

export async function submitFeedback(input: SubmitFeedbackInput): Promise<FeedbackReceipt> {
  const response = await request<{ data: FeedbackReceipt }>("/feedback", {
    method: "POST",
    body: JSON.stringify(input),
  });
  return response.data;
}

export async function confirmSubscription(token: string): Promise<SubscriptionResult> {
  const response = await request<{ data: SubscriptionResult }>("/feedback/subscriptions/confirm", {
    method: "POST",
    body: JSON.stringify({ token }),
  });
  return response.data;
}

export async function resendConfirmation(feedbackID: string, email: string): Promise<void> {
  await request("/feedback/subscriptions/resend", {
    method: "POST",
    body: JSON.stringify({ feedback_id: feedbackID, email }),
  });
}

export async function unsubscribe(token: string): Promise<SubscriptionResult> {
  const response = await request<{ data: SubscriptionResult }>(
    "/feedback/subscriptions/unsubscribe",
    {
      method: "POST",
      body: JSON.stringify({ token }),
    },
  );
  return response.data;
}
