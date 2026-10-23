export type StorySource = {
  type: string;
  citation: string;
  url: string;
};

export type Story = {
  id: string;
  month: number;
  day: number;
  year: number;
  title: string;
  lab_item: string;
  tags: string[];
  summary: string;
  content: string;
  geek_quote: string;
  source: StorySource;
};

export type TodayResponse = {
  data: Story;
  meta: { is_fallback: boolean };
};

export type StoryListResponse = {
  data: Story[];
  meta: { total: number; page: number; page_size: number };
};

export type TagCount = { name: string; count: number };

/** Shape returned by /api/v1/stories, after making sure the lists really are arrays. */
export type StoryList = {
  data: Story[];
  meta: { total: number; page: number; page_size: number };
};

export type DatasetMeta = {
  total: number;
  quarters: string[];
  coverage: string;
};

type ApiErrorPayload = {
  error?: { code?: string; message?: string };
};

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

const SERVER_API_ORIGIN = process.env.BIODAYS_API_URL || "http://127.0.0.1:8080";
const DEFAULT_REVALIDATE = 300;

type NextFetchInit = RequestInit & {
  next?: { revalidate?: number | false; tags?: string[] };
};

type QueryValue = string | number | undefined | null;

function buildSearch(params: Record<string, QueryValue>): string {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") continue;
    search.set(key, String(value));
  }
  const query = search.toString();
  return query ? `?${query}` : "";
}

async function request<T>(
  path: string,
  params: Record<string, QueryValue> = {},
  revalidate = DEFAULT_REVALIDATE,
): Promise<T> {
  const isServer = typeof window === "undefined";
  const origin = isServer ? SERVER_API_ORIGIN : "";
  const init: NextFetchInit = {
    headers: { Accept: "application/json" },
  };
  if (isServer) {
    init.next = { revalidate };
  }

  const response = await fetch(`${origin}/api/v1${path}${buildSearch(params)}`, init);
  if (!response.ok) {
    let payload: ApiErrorPayload | null = null;
    try {
      payload = (await response.json()) as ApiErrorPayload;
    } catch {
      payload = null;
    }
    throw new ApiError(
      response.status,
      payload?.error?.code ?? "UNKNOWN",
      payload?.error?.message ?? `API request failed with ${response.status}`,
    );
  }
  return (await response.json()) as T;
}

export function getToday(options?: { tz?: string; fallback?: "random" | "none" }) {
  return request<TodayResponse>("/today", {
    tz: options?.tz ?? "+08:00",
    fallback: options?.fallback ?? "random",
  });
}

export type StoryQuery = {
  month?: number;
  day?: number;
  tag?: string;
  q?: string;
  page?: number;
  pageSize?: number;
};

/**
 * The server is the source of truth, but a list response that arrives without `data`
 * (null/undefined instead of []) would reach `.map()` in render and blank the page,
 * so normalise the envelope here — this is the boundary where the data enters React.
 */
export async function getStories(query: StoryQuery = {}): Promise<StoryList> {
  const payload = await request<Partial<StoryListResponse> | null>("/stories", {
    month: query.month,
    day: query.day,
    tag: query.tag,
    q: query.q,
    page: query.page ?? 1,
    page_size: query.pageSize ?? 20,
  });
  const data = Array.isArray(payload?.data) ? payload.data : [];
  const total = payload?.meta?.total;
  return {
    data,
    meta: {
      total: typeof total === "number" ? total : data.length,
      page: payload?.meta?.page ?? query.page ?? 1,
      page_size: payload?.meta?.page_size ?? query.pageSize ?? 20,
    },
  };
}

export function getStory(id: string) {
  return request<Story>(`/stories/${encodeURIComponent(id)}`);
}

export function getRandomStory(exclude: string[] = []) {
  return request<Story>("/stories/random", {
    exclude: exclude.length > 0 ? exclude.join(",") : undefined,
  });
}

export async function getTags(): Promise<{ data: TagCount[] }> {
  const payload = await request<{ data?: TagCount[] } | null>("/tags");
  return { data: Array.isArray(payload?.data) ? payload.data : [] };
}

export function getMeta() {
  return request<{ data: DatasetMeta }>("/meta");
}

export async function getAllStories(): Promise<Story[]> {
  const first = await getStories({ page: 1, pageSize: 100 });
  const stories = [...first.data];
  const totalPages = Math.ceil(first.meta.total / 100);
  for (let page = 2; page <= totalPages && page <= 10; page++) {
    const next = await getStories({ page, pageSize: 100 });
    stories.push(...next.data);
  }
  return stories;
}
