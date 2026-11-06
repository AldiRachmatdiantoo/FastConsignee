// Bentuk jawaban openapi-fetch untuk tes layar.
export function ok<T>(data: T) {
  return Promise.resolve({ data, response: { ok: true } as Response });
}

export function problem(code: string, status = 409) {
  return Promise.resolve({
    error: { type: "about:blank", title: "Gagal", status, code, request_id: "r" },
    response: { ok: false } as Response,
  });
}

export function page<T>(data: T[]) {
  return ok({ data, next_cursor: null });
}
