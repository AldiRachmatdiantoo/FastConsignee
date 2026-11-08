import { ApiError, ApiUnavailableError, type ErrorCode } from "./client";

const MESSAGES: Partial<Record<ErrorCode, string>> = {
  FORBIDDEN: "This account cannot access that data.",
  NOT_FOUND: "Data not found.",
  RATE_LIMITED: "Too many requests. Wait a moment, then try again.",
  SERVICE_UNAVAILABLE: "The Solana network cannot be reached right now. Try again shortly.",
  WALLET_NOT_LINKED: "Link a wallet first.",
  STORE_WALLET_NOT_LINKED: "The shop wallet is not linked yet. Ask the store owner to link it.",
  RECIPIENT_WALLET_MISSING: "A consignor has no wallet yet. Add it in the consignor list first.",
  BELOW_RENT_MINIMUM: "One recipient's share is too small for an empty wallet. Add items or fund that wallet.",
  INSUFFICIENT_STOCK: "Not enough stock. Reduce the quantity.",
  TOO_MANY_CONSIGNORS: "One cart can hold at most 5 consignors.",
  DISCOUNT_TOO_LARGE: "The discount is larger than the share of whoever bears it.",
  RATE_UNAVAILABLE: "The SOL rate is unavailable right now. Try again shortly.",
  STOCK_BELOW_RESERVED: "Stock cannot be lower than the quantity currently held.",
  CONSIGNOR_EMAIL_TAKEN: "This email is already used by another consignor.",
  ORDER_NOT_REFUNDABLE: "This order is not confirmed yet, so it cannot be refunded.",
  ALREADY_REFUNDED: "Every unit on this line has been refunded.",
  REFUND_IN_PROGRESS: "A refund for this line is still in progress.",
  REFUND_NOT_AWAITING_SIGNATURE: "This refund is no longer waiting for a signature.",
};

// errorMessage mengubah kegagalan API menjadi kalimat untuk pengguna. Keputusannya memakai `code`.
export function errorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.code === "VALIDATION_FAILED" && error.problem.errors?.length) {
      return error.problem.errors.map(({ message }) => message).join(" ");
    }
    return MESSAGES[error.code] ?? error.message;
  }
  if (error instanceof ApiUnavailableError) {
    return error.message;
  }
  return "Something went wrong. Try again.";
}
