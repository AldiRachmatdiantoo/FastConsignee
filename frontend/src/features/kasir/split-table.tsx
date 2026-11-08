import { formatRupiah, formatSol } from "@/lib/format";
import type { components } from "@/lib/api/schema";

export type Recipient = components["schemas"]["SplitRecipient"];

// SplitTable menampilkan bagian tiap penerima. Angkanya selalu dari server.
export function SplitTable({ recipients }: { recipients: Recipient[] }) {
  return (
    <ul className="divide-y divide-border text-sm">
      {recipients.map((recipient) => (
        <li key={recipient.wallet_address} className="flex items-start justify-between gap-3 py-2">
          <div className="min-w-0">
            <p className="font-medium">{recipient.name}</p>
            <p className="text-muted-foreground">{recipient.recipient_type === "store" ? "Shop commission" : "Consignor share"}</p>
            {recipient.deduction_idr > 0 ? (
              <p className="text-muted-foreground">Refund deductions {formatRupiah(recipient.deduction_idr)}</p>
            ) : null}
          </div>
          <div className="text-right whitespace-nowrap">
            <p className="font-semibold">{formatRupiah(recipient.amount_idr)}</p>
            <p className="font-mono text-xs text-muted-foreground">{formatSol(recipient.amount_lamports)}</p>
          </div>
        </li>
      ))}
    </ul>
  );
}
