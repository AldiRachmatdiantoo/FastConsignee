import { notFound } from "next/navigation";
import { BayarScreen } from "@/features/bayar/bayar-screen";

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export default async function PayPage({ params }: { params: Promise<{ orderId: string }> }) {
  const { orderId } = await params;
  if (!UUID.test(orderId)) {
    notFound();
  }
  return <BayarScreen orderId={orderId} />;
}
