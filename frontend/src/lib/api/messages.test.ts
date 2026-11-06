import { describe, expect, it } from "vitest";
import { ApiError, ApiUnavailableError, type Problem } from "./client";
import { errorMessage } from "./messages";

function problem(overrides: Partial<Problem>): ApiError {
  return new ApiError({ type: "about:blank", title: "Gagal", status: 400, code: "NOT_FOUND", request_id: "r", ...overrides });
}

describe("errorMessage", () => {
  it("memilih kalimat dari code, bukan dari detail", () => {
    expect(errorMessage(problem({ code: "INSUFFICIENT_STOCK", detail: "stock too low" }))).toBe(
      "Stok tidak cukup. Kurangi jumlah barang.",
    );
  });

  it("menggabungkan kesalahan per kolom", () => {
    const error = problem({
      code: "VALIDATION_FAILED",
      errors: [
        { field: "name", message: "Nama wajib diisi." },
        { field: "price_idr", message: "Harga harus lebih dari 0." },
      ],
    });
    expect(errorMessage(error)).toBe("Nama wajib diisi. Harga harus lebih dari 0.");
  });

  it("memakai detail untuk code yang tidak dipetakan", () => {
    expect(errorMessage(problem({ code: "INTERNAL_ERROR", detail: "Ada masalah di server." }))).toBe("Ada masalah di server.");
  });

  it("menjelaskan saat API tidak bisa dihubungi", () => {
    expect(errorMessage(new ApiUnavailableError())).toContain("API tidak bisa dihubungi");
  });

  it("tetap punya kalimat untuk kegagalan lain", () => {
    expect(errorMessage(new Error("boom"))).toBe("Terjadi kesalahan. Coba lagi.");
  });
});
