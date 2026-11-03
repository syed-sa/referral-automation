import { env } from "./env";
import { supabase } from "./supabase";

export interface Pagination {
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
}

export interface Page<T> {
  data: T[];
  pagination: Pagination;
}

export class ApiError extends Error {
  status: number;
  code: string;
  details?: Record<string, string>;
  requestId?: string;

  constructor(status: number, code: string, message: string, details?: Record<string, string>, requestId?: string) {
    super(message);
    this.status = status;
    this.code = code;
    this.details = details;
    this.requestId = requestId;
  }
}

type Query = Record<string, string | number | boolean | undefined | null>;

function buildUrl(path: string, query?: Query) {
  const url = new URL(`${env.apiUrl}/api/v1${path}`);
  if (query) {
    for (const [k, v] of Object.entries(query)) {
      if (v !== undefined && v !== null && v !== "") url.searchParams.set(k, String(v));
    }
  }
  return url.toString();
}

async function accessToken(): Promise<string> {
  const { data } = await supabase.auth.getSession();
  const token = data.session?.access_token;
  if (!token) throw new ApiError(401, "unauthorized", "You are signed out.");
  return token;
}

async function send(method: string, path: string, opts: { query?: Query; body?: unknown } = {}): Promise<unknown> {
  const token = await accessToken();
  let res: Response;
  try {
    res = await fetch(buildUrl(path, opts.query), {
      method,
      headers: {
        Authorization: `Bearer ${token}`,
        ...(opts.body !== undefined ? { "Content-Type": "application/json" } : {}),
      },
      body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    });
  } catch {
    throw new ApiError(0, "network_error", "Cannot reach the server. Check your connection and try again.");
  }

  if (res.status === 204) return undefined;

  let json: any = null;
  try {
    json = await res.json();
  } catch {
    /* non-JSON body */
  }

  if (!res.ok) {
    if (res.status === 401) {
      // The token was rejected (expired or revoked). Sign out so the router sends the user to /login.
      void supabase.auth.signOut();
    }
    const e = json?.error;
    throw new ApiError(
      res.status,
      e?.code ?? "error",
      e?.message ?? `Request failed (${res.status})`,
      e?.details,
      e?.request_id,
    );
  }
  return json;
}

/** Single-object endpoints: returns the `data` envelope. */
export async function request<T>(method: string, path: string, opts: { query?: Query; body?: unknown } = {}): Promise<T> {
  const json = (await send(method, path, opts)) as { data: T } | undefined;
  return json?.data as T;
}

/** List endpoints: returns data plus pagination. */
export async function requestPage<T>(path: string, query?: Query): Promise<Page<T>> {
  return (await send("GET", path, { query })) as Page<T>;
}

export const api = {
  get: <T>(path: string, query?: Query) => request<T>("GET", path, { query }),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, { body: body ?? {} }),
  put: <T>(path: string, body: unknown) => request<T>("PUT", path, { body }),
  patch: <T>(path: string, body: unknown) => request<T>("PATCH", path, { body }),
  delete: <T = void>(path: string) => request<T>("DELETE", path),
  page: requestPage,
};

export function errorMessage(err: unknown): string {
  if (err instanceof ApiError) return err.message;
  if (err instanceof Error) return err.message;
  return "Something went wrong";
}
