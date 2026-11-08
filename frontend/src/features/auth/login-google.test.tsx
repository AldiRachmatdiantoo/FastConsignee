import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { signInWithGoogle } from "./google";
import { LoginScreen } from "./login-screen";

vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }) }));
vi.mock("./session", () => ({ useSession: () => ({ session: { status: "anonymous" }, signInLocal: vi.fn() }) }));
vi.mock("./google", () => ({ googleEnabled: true, signInWithGoogle: vi.fn() }));

beforeEach(() => {
  vi.clearAllMocks();
});

describe("tombol Google", () => {
  it("aktif dan membuka login Google kalau Supabase sudah diatur", async () => {
    vi.mocked(signInWithGoogle).mockResolvedValue(null);
    render(<LoginScreen />);

    expect(screen.queryByText("Google sign-in is not connected yet.")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Sign in with Google" }));

    expect(signInWithGoogle).toHaveBeenCalledTimes(1);
  });

  it("menampilkan pesan kalau login Google gagal", async () => {
    vi.mocked(signInWithGoogle).mockResolvedValue("Google sign-in failed. Try again.");
    render(<LoginScreen />);

    await userEvent.click(screen.getByRole("button", { name: "Sign in with Google" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Google sign-in failed. Try again.");
    expect(screen.getByRole("button", { name: "Sign in with Google" })).toBeEnabled();
  });
});
