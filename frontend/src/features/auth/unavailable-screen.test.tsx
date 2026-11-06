import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { UnavailableScreen } from "./unavailable-screen";

describe("UnavailableScreen", () => {
  it("menampilkan pesan dan mencoba lagi sekali per klik", async () => {
    let finish = () => {};
    const onRetry = vi.fn(() => new Promise<void>((resolve) => (finish = resolve)));
    render(<UnavailableScreen message="API tidak bisa dihubungi." onRetry={onRetry} />);

    expect(screen.getByRole("alert")).toHaveTextContent("API tidak bisa dihubungi.");
    await userEvent.click(screen.getByRole("button", { name: "Coba lagi" }));

    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "Mencoba..." })).toBeDisabled();
    finish();
    expect(await screen.findByRole("button", { name: "Coba lagi" })).toBeEnabled();
  });
});
