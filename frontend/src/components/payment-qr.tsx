import { QRCodeSVG } from "qrcode.react";

// Warna QR sengaja hitam di atas putih supaya tetap terbaca kamera HP.
export function PaymentQr({ value }: { value: string }) {
  return (
    <div className="inline-block rounded-lg bg-white p-3">
      <QRCodeSVG value={value} size={224} level="M" title="Solana Pay payment QR" />
    </div>
  );
}
