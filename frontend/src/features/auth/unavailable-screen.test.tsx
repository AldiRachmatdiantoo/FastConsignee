import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { UnavailableScreen } from "./unavailable-screen";

describe("UnavailableScreen", () => {
  it("menampilkan pesan dan mencoba lagi sekali per klik", async () => {
    let finish = () => {};
    const onRetry = vi.fn(() => new Promise<void>((resolve) => (finish = resolve)));
    render(<UnavailableScreen message="The API cannot be reached." onRetry={onRetry} />);

    expect(screen.getByRole("alert")).toHaveTextContent("The API cannot be reached.");
    await userEvent.click(screen.getByRole("button", { name: "Try again" }));

    expect(onRetry).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "Trying..." })).toBeDisabled();
    finish();
    expect(await screen.findByRole("button", { name: "Try again" })).toBeEnabled();
  });
});
