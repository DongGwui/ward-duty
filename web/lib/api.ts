// fetch wrapper — 쿠키 자동 동봉 + 401 시 refresh 자동 시도 + envelope unwrap.
//
// Stage A C2/C3 결과:
//   - access는 wd_access 쿠키 (httpOnly)
//   - 401 받으면 POST /api/auth/refresh → 쿠키 갱신 → 원래 요청 재시도

import type { ApiEnvelope, ApiError } from "./types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "/api";

export class ApiException extends Error {
  status: number;
  body: ApiError;
  constructor(status: number, body: ApiError) {
    super(body.message || `HTTP ${status}`);
    this.status = status;
    this.body = body;
  }
}

interface FetchOpts {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  search?: Record<string, string | number | undefined>;
  skipRefresh?: boolean; // 무한 재귀 방지용 내부 플래그
}

export async function apiFetch<T = unknown>(path: string, opts: FetchOpts = {}): Promise<T> {
  const url = buildUrl(path, opts.search);
  const init: RequestInit = {
    method: opts.method ?? "GET",
    credentials: "include", // 쿠키 동봉
    headers: { "Content-Type": "application/json" },
    body: opts.body ? JSON.stringify(opts.body) : undefined,
  };
  const res = await fetch(url, init);

  // 401 → refresh 1회 시도
  if (res.status === 401 && !opts.skipRefresh && !path.startsWith("/auth/")) {
    const refreshed = await tryRefresh();
    if (refreshed) return apiFetch(path, { ...opts, skipRefresh: true });
  }

  const text = await res.text();
  if (!text) {
    if (res.ok) return undefined as T;
    throw new ApiException(res.status, { code: "UNKNOWN", message: res.statusText });
  }
  const json = JSON.parse(text) as ApiEnvelope<T>;
  if (!res.ok) {
    throw new ApiException(res.status, json.error ?? { code: "UNKNOWN", message: res.statusText });
  }
  return json.data as T;
}

// meta까지 받고 싶을 때 (PATCH cell의 violations 등).
export async function apiFetchFull<T = unknown>(
  path: string,
  opts: FetchOpts = {},
): Promise<{ data: T; meta?: Record<string, unknown> }> {
  const url = buildUrl(path, opts.search);
  const res = await fetch(url, {
    method: opts.method ?? "GET",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: opts.body ? JSON.stringify(opts.body) : undefined,
  });
  if (res.status === 401 && !opts.skipRefresh && !path.startsWith("/auth/")) {
    if (await tryRefresh()) return apiFetchFull(path, { ...opts, skipRefresh: true });
  }
  const json = (await res.json()) as ApiEnvelope<T>;
  if (!res.ok) {
    throw new ApiException(res.status, json.error ?? { code: "UNKNOWN", message: res.statusText });
  }
  return { data: json.data as T, meta: json.meta };
}

async function tryRefresh(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/auth/refresh`, {
      method: "POST",
      credentials: "include",
    });
    return res.ok;
  } catch {
    return false;
  }
}

function buildUrl(path: string, search?: Record<string, string | number | undefined>): string {
  const base = API_BASE + (path.startsWith("/") ? path : "/" + path);
  if (!search) return base;
  const params = new URLSearchParams();
  for (const [k, v] of Object.entries(search)) {
    if (v !== undefined && v !== "" && v !== null) params.set(k, String(v));
  }
  const s = params.toString();
  return s ? `${base}?${s}` : base;
}

export function loginUrl(): string {
  return `${API_BASE}/auth/oauth/google/start`;
}
