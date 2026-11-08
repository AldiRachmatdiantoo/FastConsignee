import createClient, { type Client } from "openapi-fetch";
import type { components, paths } from "./schema";

export type Me = components["schemas"]["Me"];
export type Problem = components["schemas"]["Problem"];
export type ErrorCode = components["schemas"]["ErrorCode"];
export type Api = Client<paths>;

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

// ApiError membawa Problem RFC 9457. Keputusan diambil dari `code`, bukan dari teks `detail`.
export class ApiError extends Error {
  constructor(readonly problem: Problem) {
    super(problem.detail ?? problem.title);
    this.name = "ApiError";
  }

  get code(): ErrorCode {
    return this.problem.code;
  }

  get status(): number {
    return this.problem.status;
  }
}

// ApiUnavailableError berarti jawaban API tidak sampai, misalnya server mati.
export class ApiUnavailableError extends Error {
  constructor() {
    super("The API cannot be reached. Try again shortly.");
    this.name = "ApiUnavailableError";
  }
}

type ClientOptions = {
  getToken: () => string | null;
  onUnauthorized: () => void;
  baseUrl?: string;
  fetch?: typeof globalThis.fetch;
};

export function createApiClient({ getToken, onUnauthorized, baseUrl = API_URL, fetch }: ClientOptions): Api {
  const client = createClient<paths>({ baseUrl, fetch });
  client.use({
    onRequest({ request }) {
      const token = getToken();
      if (token) {
        request.headers.set("Authorization", `Bearer ${token}`);
      }
      return request;
    },
    onResponse({ response }) {
      if (response.status === 401) {
        onUnauthorized();
      }
      return response;
    },
  });
  return client;
}

function isProblem(value: unknown): value is Problem {
  return typeof value === "object" && value !== null && "code" in value && "status" in value;
}

type Result<T> = { data?: T; error?: unknown; response: Response };

// unwrap mengembalikan isi jawaban, atau melempar ApiError dan ApiUnavailableError.
export async function unwrap<T>(call: Promise<Result<T>>): Promise<T> {
  let result: Result<T>;
  try {
    result = await call;
  } catch {
    throw new ApiUnavailableError();
  }
  if (result.response.ok) {
    return result.data as T;
  }
  if (isProblem(result.error)) {
    throw new ApiError(result.error);
  }
  throw new ApiUnavailableError();
}
