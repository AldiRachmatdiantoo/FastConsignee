package dashboard_test

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

type orderBody struct {
	ID    string `json:"id"`
	Lines []struct {
		ID     string `json:"id"`
		ItemID string `json:"item_id"`
	} `json:"lines"`
	Payment struct {
		Reference string `json:"reference"`
	} `json:"payment"`
}

type saleBody struct {
	OrderID               string `json:"order_id"`
	OrderLineID           string `json:"order_line_id"`
	Quantity              int    `json:"quantity"`
	LineTotalIDR          int64  `json:"line_total_idr"`
	CommissionIDR         int64  `json:"commission_idr"`
	ConsignorShareIDR     int64  `json:"consignor_share_idr"`
	RefundedQuantity      int    `json:"refunded_quantity"`
	OrderDeductionIDR     int64  `json:"order_deduction_idr"`
	OrderTransferLamports string `json:"order_transfer_lamports"`
	Signature             string `json:"signature"`
	ExplorerURL           string `json:"explorer_url"`
}

type deductionBody struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	AmountIDR    int64   `json:"amount_idr"`
	RemainingIDR int64   `json:"remaining_idr"`
	SettledAt    *string `json:"settled_at"`
}

type deductionList struct {
	Data        []deductionBody `json:"data"`
	NextCursor  *string         `json:"next_cursor"`
	Outstanding int64           `json:"outstanding_total_idr"`
}

type fixture struct {
	a       *testkit.App
	store   testkit.Store
	owner   testkit.User
	cashier testkit.User
	sari    testkit.User
	budi    testkit.User
	bag     uuid.UUID
	jam     uuid.UUID
	buyer   testkit.Wallet
	keys    int
}

func setup(t *testing.T) *fixture {
	t.Helper()
	a := testkit.NewApp(t)
	store := a.NewStore()
	f := &fixture{
		a: a, store: store, buyer: testkit.NewWallet(),
		owner:   a.NewUser(store.ID, auth.RoleOwner),
		cashier: a.NewUser(store.ID, auth.RoleCashier),
		sari:    a.NewConsignor(store.ID),
		budi:    a.NewConsignor(store.ID),
	}
	f.bag = a.NewItem(f.sari.ConsignorID, 100000, 2000, 50)
	f.jam = a.NewItem(f.budi.ConsignorID, 50000, 1500, 50)
	return f
}

func (f *fixture) key() string {
	f.keys++
	return fmt.Sprintf("kunci-dasbor-%d", f.keys)
}

func (f *fixture) create(t *testing.T, lines ...map[string]any) orderBody {
	t.Helper()
	var o orderBody
	f.a.PostKey(f.cashier, "/api/v1/orders", f.key(), map[string]any{"lines": lines}).Want(201).JSON(&o)
	return o
}

func (f *fixture) pay(t *testing.T, o orderBody) {
	t.Helper()
	var res struct {
		Transaction string `json:"transaction"`
	}
	f.a.Public("POST", "/api/v1/solana-pay/orders/"+o.ID, map[string]any{"account": f.buyer.Address}).Want(200).JSON(&res)
	tx, _, err := solana.DecodeBase64(res.Transaction)
	if err != nil {
		t.Fatal(err)
	}
	tx.Signature = testkit.NewSignature()
	f.a.Solana.Publish(o.Payment.Reference, tx)
	f.a.Tick()
}

func line(id uuid.UUID, qty int) map[string]any {
	return map[string]any{"item_id": id, "quantity": qty}
}

func (f *fixture) sold(t *testing.T, lines ...map[string]any) orderBody {
	t.Helper()
	o := f.create(t, lines...)
	f.pay(t, o)
	return o
}

// refund menyelesaikan refund satu unit pada baris pertama.
func (f *fixture) refund(t *testing.T, o orderBody) {
	t.Helper()
	var d struct {
		Reference   string `json:"reference"`
		Transaction string `json:"transaction"`
	}
	f.a.PostKey(f.owner, "/api/v1/orders/"+o.ID+"/refunds", f.key(), map[string]any{"order_line_id": o.Lines[0].ID}).Want(201).JSON(&d)
	tx, _, err := solana.DecodeBase64(d.Transaction)
	if err != nil {
		t.Fatal(err)
	}
	tx.Signature = testkit.NewSignature()
	f.a.Solana.Publish(d.Reference, tx)
	f.a.Tick()
}

func TestConsignorSeesOwnSales(t *testing.T) {
	f := setup(t)
	o := f.sold(t, line(f.bag, 2), line(f.jam, 1))
	f.create(t, line(f.bag, 1))

	var page struct {
		Data []saleBody `json:"data"`
	}
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/sales").Want(200).JSON(&page)
	if len(page.Data) != 1 {
		t.Fatalf("hanya order confirmed yang tampil: %+v", page.Data)
	}
	s := page.Data[0]
	if s.OrderID != o.ID || s.Quantity != 2 || s.LineTotalIDR != 200000 || s.CommissionIDR != 40000 || s.ConsignorShareIDR != 160000 {
		t.Fatalf("penjualan salah: %+v", s)
	}
	if s.OrderTransferLamports != "64000000" || !strings.Contains(s.ExplorerURL, s.Signature) {
		t.Fatalf("transfer salah: %+v", s)
	}
	f.a.Get(f.budi, "/api/v1/dashboard/consignor/sales").Want(200).JSON(&page)
	if len(page.Data) != 1 || page.Data[0].LineTotalIDR != 50000 {
		t.Fatalf("penitip lain: %+v", page.Data)
	}
}

func TestConsignorSalesIncludeAllItemsOfTheTransfer(t *testing.T) {
	f := setup(t)
	second := f.a.NewItem(f.sari.ConsignorID, 10000, 0, 5)
	f.sold(t, line(f.bag, 1), line(second, 1))
	var page struct {
		Data []saleBody `json:"data"`
	}
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/sales").Want(200).JSON(&page)
	if len(page.Data) != 2 {
		t.Fatalf("penjualan %d", len(page.Data))
	}
	for _, s := range page.Data {
		if s.OrderTransferLamports != "36000000" {
			t.Fatalf("transfer order harus sama untuk semua barangnya: %+v", s)
		}
	}
}

func TestSalesPaginationAndPeriod(t *testing.T) {
	f := setup(t)
	for range 5 {
		f.sold(t, line(f.bag, 1))
		f.a.Clock.Advance(time.Hour)
	}
	seen := map[string]bool{}
	path := "/api/v1/dashboard/consignor/sales?limit=2"
	for {
		var page struct {
			Data       []saleBody `json:"data"`
			NextCursor *string    `json:"next_cursor"`
		}
		f.a.Get(f.sari, path).Want(200).JSON(&page)
		for _, s := range page.Data {
			if seen[s.OrderLineID] {
				t.Fatalf("baris %s muncul dua kali", s.OrderLineID)
			}
			seen[s.OrderLineID] = true
		}
		if page.NextCursor == nil {
			break
		}
		path = "/api/v1/dashboard/consignor/sales?limit=2&cursor=" + url.QueryEscape(*page.NextCursor)
	}
	if len(seen) != 5 {
		t.Fatalf("penjualan %d", len(seen))
	}
	start := f.a.Clock.Now().Add(-5 * time.Hour)
	stamp := func(at time.Time) string { return url.QueryEscape(at.UTC().Format(time.RFC3339)) }
	var page struct {
		Data []saleBody `json:"data"`
	}
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/sales?from="+stamp(start.Add(time.Hour))+"&to="+stamp(start.Add(3*time.Hour))).Want(200).JSON(&page)
	if len(page.Data) != 2 {
		t.Fatalf("awal inklusif dan akhir eksklusif: %d", len(page.Data))
	}
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/sales?from=kemarin").WantProblem(400, "VALIDATION_FAILED")
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/sales?limit=0").WantProblem(400, "VALIDATION_FAILED")
}

func TestDeductionLifecycle(t *testing.T) {
	f := setup(t)
	first := f.sold(t, line(f.bag, 1))
	f.refund(t, first)

	var list deductionList
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/deductions").Want(200).JSON(&list)
	if len(list.Data) != 1 || list.Outstanding != 80000 {
		t.Fatalf("potongan salah: %+v", list)
	}
	d := list.Data[0]
	if d.Status != "outstanding" || d.AmountIDR != 80000 || d.RemainingIDR != 80000 || d.SettledAt != nil {
		t.Fatalf("potongan salah: %+v", d)
	}
	var budi deductionList
	f.a.Get(f.budi, "/api/v1/dashboard/consignor/deductions").Want(200).JSON(&budi)
	if len(budi.Data) != 0 || budi.Outstanding != 0 {
		t.Fatalf("penitip lain melihat potongan: %+v", budi)
	}

	cheap := f.a.NewItem(f.sari.ConsignorID, 50000, 0, 5)
	f.sold(t, line(cheap, 1))
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/deductions").Want(200).JSON(&list)
	if list.Data[0].RemainingIDR != 30000 || list.Outstanding != 30000 || list.Data[0].Status != "outstanding" {
		t.Fatalf("pelunasan sebagian salah: %+v", list)
	}
	var sales struct {
		Data []saleBody `json:"data"`
	}
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/sales?limit=1").Want(200).JSON(&sales)
	if sales.Data[0].OrderDeductionIDR != 50000 || sales.Data[0].OrderTransferLamports != "0" {
		t.Fatalf("penjualan harus menampilkan potongan: %+v", sales.Data[0])
	}

	f.sold(t, line(cheap, 1))
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/deductions").Want(200).JSON(&list)
	d = list.Data[0]
	if d.Status != "settled" || d.RemainingIDR != 0 || d.SettledAt == nil || list.Outstanding != 0 {
		t.Fatalf("potongan harus lunas: %+v %+v", d, list)
	}
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/deductions?status=outstanding").Want(200).JSON(&list)
	if len(list.Data) != 0 {
		t.Fatalf("filter outstanding: %+v", list)
	}
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/deductions?status=settled").Want(200).JSON(&list)
	if len(list.Data) != 1 {
		t.Fatalf("filter settled: %+v", list)
	}
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/deductions?status=lunas").WantProblem(400, "VALIDATION_FAILED")
}

func TestExpiredSettlementReopensDeductionOnDashboard(t *testing.T) {
	f := setup(t)
	f.refund(t, f.sold(t, line(f.bag, 1)))
	f.create(t, line(f.bag, 1))

	var list deductionList
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/deductions").Want(200).JSON(&list)
	if list.Outstanding != 0 {
		t.Fatalf("order pending sudah memesan pelunasan: %+v", list)
	}
	f.a.Clock.Advance(8 * time.Minute)
	f.a.Tick()
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/deductions").Want(200).JSON(&list)
	if list.Outstanding != 80000 || list.Data[0].Status != "outstanding" {
		t.Fatalf("potongan harus terbuka lagi: %+v", list)
	}
}

func TestDeductionsAreNeverCached(t *testing.T) {
	f := setup(t)
	f.refund(t, f.sold(t, line(f.bag, 1)))
	var list deductionList
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/deductions").Want(200).JSON(&list)
	f.a.Exec(`UPDATE refunds SET consignor_deduction_idr = 1`)
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/deductions").Want(200).JSON(&list)
	if list.Outstanding != 1 {
		t.Fatalf("potongan terbaca dari cache: %+v", list)
	}
}

func TestStockMatchesCatalog(t *testing.T) {
	f := setup(t)
	hidden := f.a.NewItem(f.sari.ConsignorID, 1000, 0, 3)
	f.a.Exec(`UPDATE items SET active = false WHERE id = $1`, hidden)
	f.create(t, line(f.bag, 4))

	var page struct {
		Data []struct {
			ItemID    string `json:"item_id"`
			Available int    `json:"stock_available"`
			Active    bool   `json:"active"`
		} `json:"data"`
		NextCursor *string `json:"next_cursor"`
	}
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/stock").Want(200).JSON(&page)
	if len(page.Data) != 2 {
		t.Fatalf("stok harus memuat barang nonaktif: %+v", page.Data)
	}
	var catalog struct {
		Available int `json:"stock_available"`
	}
	f.a.Get(f.owner, "/api/v1/items/"+f.bag.String()).Want(200).JSON(&catalog)
	for _, it := range page.Data {
		if it.ItemID == f.bag.String() && it.Available != catalog.Available {
			t.Fatalf("stok %d, katalog %d", it.Available, catalog.Available)
		}
	}
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/stock?limit=1").Want(200).JSON(&page)
	if len(page.Data) != 1 || page.NextCursor == nil {
		t.Fatalf("halaman salah: %+v", page)
	}
	f.a.Get(f.budi, "/api/v1/dashboard/consignor/stock").Want(200).JSON(&page)
	if len(page.Data) != 1 || page.Data[0].ItemID != f.jam.String() {
		t.Fatalf("penitip lain: %+v", page.Data)
	}
}

func TestStoreSummary(t *testing.T) {
	f := setup(t)
	f.sold(t, line(f.bag, 1), line(f.jam, 2))
	f.sold(t, line(f.bag, 1))
	f.create(t, line(f.bag, 1))

	var s struct {
		From       *string `json:"from"`
		Sales      int64   `json:"total_sales_idr"`
		Commission int64   `json:"total_commission_idr"`
		Count      int     `json:"transaction_count"`
	}
	f.a.Get(f.owner, "/api/v1/dashboard/store/summary").Want(200).JSON(&s)
	if s.Sales != 300000 || s.Commission != 55000 || s.Count != 2 || s.From != nil {
		t.Fatalf("ringkasan salah: %+v", s)
	}

	f.sold(t, line(f.jam, 1))
	f.a.Get(f.owner, "/api/v1/dashboard/store/summary").Want(200).JSON(&s)
	if s.Count != 2 {
		t.Fatalf("ringkasan seharusnya dari cache: %+v", s)
	}
	f.a.Clock.Advance(31 * time.Second)
	f.a.Get(f.owner, "/api/v1/dashboard/store/summary").Want(200).JSON(&s)
	if s.Count != 3 {
		t.Fatalf("cache seharusnya kedaluwarsa: %+v", s)
	}

	future := url.QueryEscape(f.a.Clock.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	f.a.Get(f.owner, "/api/v1/dashboard/store/summary?from="+future).Want(200).JSON(&s)
	if s.Count != 0 || s.From == nil {
		t.Fatalf("filter periode: %+v", s)
	}
	f.a.Get(f.owner, "/api/v1/dashboard/store/summary?to=kemarin").WantProblem(400, "VALIDATION_FAILED")
}

func TestSummaryIsPerStore(t *testing.T) {
	f := setup(t)
	f.sold(t, line(f.bag, 1))
	other := f.a.NewStore()
	stranger := f.a.NewUser(other.ID, auth.RoleOwner)
	var s struct {
		Count int `json:"transaction_count"`
	}
	f.a.Get(stranger, "/api/v1/dashboard/store/summary").Want(200).JSON(&s)
	if s.Count != 0 {
		t.Fatalf("toko lain: %+v", s)
	}
}

func TestDashboardRoles(t *testing.T) {
	f := setup(t)
	for _, path := range []string{"sales", "deductions", "stock"} {
		f.a.Get(f.owner, "/api/v1/dashboard/consignor/"+path).WantProblem(403, "FORBIDDEN")
		f.a.Get(f.cashier, "/api/v1/dashboard/consignor/"+path).WantProblem(403, "FORBIDDEN")
	}
	f.a.Get(f.sari, "/api/v1/dashboard/store/summary").WantProblem(403, "FORBIDDEN")
	f.a.Get(f.cashier, "/api/v1/dashboard/store/summary").WantProblem(403, "FORBIDDEN")

	f.a.Exec(`UPDATE consignors SET wallet_verified_at = NULL WHERE id = $1`, f.sari.ConsignorID)
	f.a.Get(f.sari, "/api/v1/dashboard/consignor/sales").WantProblem(403, "WALLET_NOT_LINKED")
}
