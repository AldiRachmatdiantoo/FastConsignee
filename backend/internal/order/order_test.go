package order_test

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/order"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/config"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

type recipient struct {
	RecipientType  string  `json:"recipient_type"`
	ConsignorID    *string `json:"consignor_id"`
	WalletAddress  string  `json:"wallet_address"`
	ShareIDR       int64   `json:"share_idr"`
	DeductionIDR   int64   `json:"deduction_idr"`
	RecoveredIDR   int64   `json:"recovered_idr"`
	AmountIDR      int64   `json:"amount_idr"`
	AmountLamports string  `json:"amount_lamports"`
}

type orderBody struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	SubtotalIDR   int64  `json:"subtotal_idr"`
	DiscountIDR   int64  `json:"discount_idr"`
	TotalIDR      int64  `json:"total_idr"`
	TotalLamports string `json:"total_lamports"`
	Rate          struct {
		IDRPerSOL int64 `json:"idr_per_sol"`
	} `json:"rate"`
	Lines []struct {
		ID             string `json:"id"`
		ItemID         string `json:"item_id"`
		LineTotalIDR   int64  `json:"line_total_idr"`
		CommissionIDR  int64  `json:"commission_idr"`
		ConsignorShare int64  `json:"consignor_share_idr"`
	} `json:"lines"`
	Recipients []recipient `json:"recipients"`
	Payment    struct {
		Reference    string `json:"reference"`
		SolanaPayURL string `json:"solana_pay_url"`
	} `json:"payment"`
	ExpiresAt time.Time `json:"expires_at"`
}

type fixture struct {
	a         *testkit.App
	store     testkit.Store
	owner     testkit.User
	cashier   testkit.User
	sari      testkit.User
	budi      testkit.User
	bag       uuid.UUID
	jam       uuid.UUID
	keyNumber int
}

func setup(t *testing.T, opts ...testkit.Option) *fixture {
	t.Helper()
	a := testkit.NewApp(t, opts...)
	store := a.NewStore()
	f := &fixture{
		a: a, store: store,
		owner:   a.NewUser(store.ID, auth.RoleOwner),
		cashier: a.NewUser(store.ID, auth.RoleCashier),
		sari:    a.NewConsignor(store.ID),
		budi:    a.NewConsignor(store.ID),
	}
	f.bag = a.NewItem(f.sari.ConsignorID, 100000, 2000, 5)
	f.jam = a.NewItem(f.budi.ConsignorID, 50000, 1500, 5)
	return f
}

func line(id uuid.UUID, qty int) map[string]any {
	return map[string]any{"item_id": id, "quantity": qty}
}

func cart(lines ...map[string]any) map[string]any {
	return map[string]any{"lines": lines}
}

func discounted(id uuid.UUID, qty int, amount int64, bearer string) map[string]any {
	l := line(id, qty)
	l["discount"] = map[string]any{"amount_idr": amount, "borne_by": bearer}
	return l
}

func (f *fixture) key() string {
	f.keyNumber++
	return fmt.Sprintf("kunci-test-%d", f.keyNumber)
}

func (f *fixture) create(t *testing.T, u testkit.User, body map[string]any) orderBody {
	t.Helper()
	var o orderBody
	f.a.PostKey(u, "/api/v1/orders", f.key(), body).Want(201).JSON(&o)
	return o
}

func (f *fixture) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	f.a.Scalar(&n, sql, args...)
	return n
}

// debt membuat potongan refund yang sudah selesai untuk penitip.
func (f *fixture) debt(t *testing.T, c testkit.User, amount int64) uuid.UUID {
	t.Helper()
	it := f.a.NewItem(c.ConsignorID, amount, 0, 1)
	orderID, lineID, refund, deduction := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	f.a.Exec(`INSERT INTO orders (id, created_by, status, reference, idr_per_sol, rate_quoted_at, payer_wallet, signature,
		idempotency_key, request_hash, expires_at, confirmed_at)
		VALUES ($1, $2, 'confirmed', $3, 2500000, now(), $4, $5, $6, 'h', now(), now())`,
		orderID, f.cashier.ID, uuid.NewString(), testkit.NewWallet().Address, testkit.NewSignature(), uuid.NewString())
	f.a.Exec(`INSERT INTO order_lines (id, order_id, item_id, quantity, unit_price_idr, commission_bps)
		VALUES ($1, $2, $3, 1, $4, 0)`, lineID, orderID, it, amount)
	f.a.Exec(`INSERT INTO refunds (id, order_line_id, created_by, status, amount_idr, amount_lamports, consignor_deduction_idr,
		source_wallet, reference, signature, idempotency_key, request_hash, expires_at, completed_at)
		VALUES ($1, $2, $3, 'completed', $4, 1, $4, $5, $6, $7, $8, 'h', now(), now())`,
		refund, lineID, f.owner.ID, amount, f.store.Wallet.Address, uuid.NewString(), testkit.NewSignature(), uuid.NewString())
	f.a.Exec(`INSERT INTO deductions (id, refund_id) VALUES ($1, $2)`, deduction, refund)
	return deduction
}

func TestPreviewMatchesContractExampleAndSavesNothing(t *testing.T) {
	f := setup(t)
	var p orderBody
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(f.bag, 1), line(f.jam, 2))).Want(200).JSON(&p)
	if p.TotalIDR != 200000 || p.TotalLamports != "80000000" || p.Rate.IDRPerSOL != 2500000 {
		t.Fatalf("total salah: %+v", p)
	}
	want := []string{"32000000", "34000000", "14000000"}
	for i, r := range p.Recipients {
		if r.AmountLamports != want[i] {
			t.Fatalf("penerima %d: %+v", i, r)
		}
	}
	if p.Recipients[0].WalletAddress != f.sari.Wallet.Address || p.Recipients[2].RecipientType != "store" {
		t.Fatalf("urutan penerima salah: %+v", p.Recipients)
	}
	if n := f.count(t, `SELECT count(*) FROM orders`); n != 0 {
		t.Fatalf("rincian tidak boleh menyimpan order, ada %d", n)
	}
}

func TestPreviewDiscount(t *testing.T) {
	f := setup(t)
	var p orderBody
	f.a.Post(f.owner, "/api/v1/orders/preview", cart(discounted(f.bag, 1, 5000, "store"))).Want(200).JSON(&p)
	if p.SubtotalIDR != 100000 || p.DiscountIDR != 5000 || p.TotalIDR != 95000 {
		t.Fatalf("total salah: %+v", p)
	}
	if p.Recipients[0].AmountIDR != 80000 || p.Recipients[1].AmountIDR != 15000 {
		t.Fatalf("diskon toko salah: %+v", p.Recipients)
	}
	f.a.Post(f.owner, "/api/v1/orders/preview", cart(discounted(f.bag, 1, 5000, "consignor"))).Want(200).JSON(&p)
	if p.Recipients[0].AmountIDR != 75000 || p.Recipients[1].AmountIDR != 20000 {
		t.Fatalf("diskon penitip salah: %+v", p.Recipients)
	}
	f.a.Post(f.owner, "/api/v1/orders/preview", cart(discounted(f.bag, 1, 20001, "store"))).WantProblem(422, "DISCOUNT_TOO_LARGE")
	f.a.Post(f.owner, "/api/v1/orders/preview", cart(discounted(f.bag, 1, 80001, "consignor"))).WantProblem(422, "DISCOUNT_TOO_LARGE")

	free := f.a.NewItem(f.sari.ConsignorID, 1000, 0, 5)
	f.a.Post(f.owner, "/api/v1/orders/preview", cart(discounted(free, 1, 1000, "consignor"))).WantProblem(422, "DISCOUNT_TOO_LARGE")
}

func TestCartValidation(t *testing.T) {
	f := setup(t)
	bad := map[string]any{
		"kosong":          cart(),
		"tanpa barang":    cart(map[string]any{"quantity": 1}),
		"tanpa jumlah":    cart(map[string]any{"item_id": f.bag}),
		"jumlah 0":        cart(line(f.bag, 0)),
		"jumlah kelewat":  cart(line(f.bag, 1001)),
		"barang ganda":    cart(line(f.bag, 1), line(f.bag, 1)),
		"diskon 0":        cart(discounted(f.bag, 1, 0, "store")),
		"penanggung asal": cart(discounted(f.bag, 1, 1, "pembeli")),
		"kolom asing":     map[string]any{"lines": []any{line(f.bag, 1)}, "total_idr": 1},
		"bukan id":        cart(map[string]any{"item_id": "abc", "quantity": 1}),
	}
	for name, body := range bad {
		t.Run(name, func(t *testing.T) {
			f.a.Post(f.cashier, "/api/v1/orders/preview", body).WantProblem(400, "VALIDATION_FAILED")
			f.a.PostKey(f.cashier, "/api/v1/orders", "kunci-valid-1", body).WantProblem(400, "VALIDATION_FAILED")
		})
	}
	lines := make([]map[string]any, 51)
	for i := range lines {
		lines[i] = line(uuid.New(), 1)
	}
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(lines...)).WantProblem(400, "VALIDATION_FAILED")
}

func TestCartRejectsUnknownAndForeignItems(t *testing.T) {
	f := setup(t)
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(uuid.New(), 1))).WantProblem(404, "NOT_FOUND")
	other := f.a.NewStore()
	stranger := f.a.NewConsignor(other.ID)
	foreign := f.a.NewItem(stranger.ConsignorID, 1000, 0, 5)
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(foreign, 1))).WantProblem(404, "NOT_FOUND")
}

func TestCartRejectsShortStock(t *testing.T) {
	f := setup(t)
	r := f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(f.bag, 6))).WantProblem(409, "INSUFFICIENT_STOCK")
	var p struct {
		Details struct {
			Items []struct {
				ItemID    string `json:"item_id"`
				Requested int    `json:"requested"`
				Available int    `json:"available"`
			} `json:"items"`
		} `json:"details"`
	}
	r.JSON(&p)
	if len(p.Details.Items) != 1 || p.Details.Items[0].Requested != 6 || p.Details.Items[0].Available != 5 {
		t.Fatalf("rincian stok salah: %+v", p)
	}
	f.a.Exec(`UPDATE items SET active = false WHERE id = $1`, f.bag)
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(f.bag, 1))).WantProblem(409, "INSUFFICIENT_STOCK")
}

func TestCartRejectsTooManyConsignors(t *testing.T) {
	f := setup(t)
	lines := []map[string]any{}
	for range 5 {
		lines = append(lines, line(f.a.NewItem(f.a.NewConsignor(f.store.ID).ConsignorID, 10000, 1000, 5), 1))
	}
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(lines...)).Want(200)
	lines = append(lines, line(f.bag, 1))
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(lines...)).WantProblem(422, "TOO_MANY_CONSIGNORS")
}

func TestCartRequiresWallets(t *testing.T) {
	f := setup(t)
	f.a.Exec(`UPDATE consignors SET wallet_address = NULL, wallet_verified_at = NULL WHERE id = $1`, f.sari.ConsignorID)
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(f.bag, 1))).WantProblem(422, "RECIPIENT_WALLET_MISSING")

	f.a.Exec(`UPDATE consignors SET wallet_address = $2 WHERE id = $1`, f.sari.ConsignorID, testkit.NewWallet().Address)
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(f.bag, 1))).Want(200)

	store := f.a.NewStoreWithoutWallet()
	cashier := f.a.NewUser(store.ID, auth.RoleCashier)
	c := f.a.NewConsignor(store.ID)
	it := f.a.NewItem(c.ConsignorID, 10000, 1000, 5)
	f.a.Post(cashier, "/api/v1/orders/preview", cart(line(it, 1))).WantProblem(422, "STORE_WALLET_NOT_LINKED")
}

func TestCartChecksRentForEmptyWallets(t *testing.T) {
	f := setup(t)
	small := f.a.NewItem(f.sari.ConsignorID, 2000, 0, 5)
	f.a.Solana.SetBalance(f.sari.Wallet.Address, 0)
	r := f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(small, 1))).WantProblem(422, "BELOW_RENT_MINIMUM")
	var p struct {
		Details struct {
			Wallet  string `json:"wallet_address"`
			Amount  string `json:"amount_lamports"`
			Minimum string `json:"minimum_lamports"`
		} `json:"details"`
	}
	r.JSON(&p)
	if p.Details.Wallet != f.sari.Wallet.Address || p.Details.Amount != "800000" || p.Details.Minimum != "890880" {
		t.Fatalf("rincian salah: %+v", p)
	}
	f.a.PostKey(f.cashier, "/api/v1/orders", f.key(), cart(line(small, 1))).WantProblem(422, "BELOW_RENT_MINIMUM")

	f.a.Solana.SetBalance(f.sari.Wallet.Address, 1)
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(small, 1))).Want(200)

	f.a.Solana.SetBalance(f.sari.Wallet.Address, 0)
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(f.bag, 1))).Want(200)
}

func TestCartFailsCleanlyWithoutRPCOrRate(t *testing.T) {
	f := setup(t)
	f.a.Solana.SetErr(errors.New("RPC mati"))
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(f.bag, 1))).WantProblem(503, "SERVICE_UNAVAILABLE")
	f.a.PostKey(f.cashier, "/api/v1/orders", f.key(), cart(line(f.bag, 1))).WantProblem(503, "SERVICE_UNAVAILABLE")
	if n := f.count(t, `SELECT count(*) FROM orders`); n != 0 {
		t.Fatalf("order tidak boleh tersimpan, ada %d", n)
	}

	g := setup(t, func(c *config.Config) { c.FixedIDRPerSOL = 0 })
	g.a.Post(g.cashier, "/api/v1/orders/preview", cart(line(g.bag, 1))).WantProblem(503, "RATE_UNAVAILABLE")
}

func TestCreateOrder(t *testing.T) {
	f := setup(t)
	r := f.a.PostKey(f.cashier, "/api/v1/orders", f.key(), cart(line(f.jam, 2), line(f.bag, 1))).Want(201)
	var o orderBody
	r.JSON(&o)
	if r.Header.Get("Location") != "/api/v1/orders/"+o.ID {
		t.Fatalf("Location salah: %s", r.Header.Get("Location"))
	}
	if r.Header.Get("Idempotency-Replayed") != "" {
		t.Fatal("order baru bukan replay")
	}
	if o.Status != "pending" || o.TotalIDR != 200000 || o.TotalLamports != "80000000" {
		t.Fatalf("order salah: %+v", o)
	}
	if o.Payment.SolanaPayURL != "solana:https://api.test/api/v1/solana-pay/orders/"+o.ID {
		t.Fatalf("URL salah: %s", o.Payment.SolanaPayURL)
	}
	if !o.ExpiresAt.Equal(f.a.Clock.Now().Add(5 * time.Minute)) {
		t.Fatalf("batas waktu salah: %s", o.ExpiresAt)
	}
	if o.Lines[0].ItemID != f.jam.String() || o.Lines[1].ItemID != f.bag.String() {
		t.Fatalf("urutan keranjang harus tersimpan: %+v", o.Lines)
	}
	if o.Recipients[0].ConsignorID == nil || *o.Recipients[0].ConsignorID != f.budi.ConsignorID.String() {
		t.Fatalf("urutan penerima harus sama dengan rincian: %+v", o.Recipients)
	}
	var again orderBody
	f.a.Get(f.owner, "/api/v1/orders/"+o.ID).Want(200).JSON(&again)
	if again.TotalLamports != o.TotalLamports || len(again.Recipients) != 3 {
		t.Fatalf("detail tidak sama: %+v", again)
	}
}

func TestCreateLocksRate(t *testing.T) {
	f := setup(t)
	o := f.create(t, f.cashier, cart(line(f.bag, 1)))
	f.a.Cache.Set("rate:sol-idr", order.Rate{IDRPerSOL: 5000000, QuotedAt: f.a.Clock.Now()}, time.Hour)

	var preview orderBody
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(f.bag, 1))).Want(200).JSON(&preview)
	if preview.Rate.IDRPerSOL != 5000000 || preview.TotalLamports != "20000000" {
		t.Fatalf("rincian harus memakai kurs baru: %+v", preview)
	}
	var got orderBody
	f.a.Get(f.cashier, "/api/v1/orders/"+o.ID).Want(200).JSON(&got)
	if got.Rate.IDRPerSOL != 2500000 || got.TotalLamports != "40000000" {
		t.Fatalf("order harus memakai kurs saat dibuat: %+v", got)
	}
}

func TestCreateRequiresIdempotencyKey(t *testing.T) {
	f := setup(t)
	f.a.Post(f.cashier, "/api/v1/orders", cart(line(f.bag, 1))).WantProblem(400, "VALIDATION_FAILED")
	f.a.PostKey(f.cashier, "/api/v1/orders", "pendek", cart(line(f.bag, 1))).WantProblem(400, "VALIDATION_FAILED")
}

func TestCreateIsIdempotent(t *testing.T) {
	f := setup(t)
	first := f.a.PostKey(f.cashier, "/api/v1/orders", "kunci-yang-sama", cart(line(f.bag, 1))).Want(201)
	second := f.a.PostKey(f.cashier, "/api/v1/orders", "kunci-yang-sama", cart(line(f.bag, 1))).Want(201)
	var a, b orderBody
	first.JSON(&a)
	second.JSON(&b)
	if a.ID != b.ID || second.Header.Get("Idempotency-Replayed") != "true" {
		t.Fatalf("replay salah: %s %s %v", a.ID, b.ID, second.Header)
	}
	f.a.PostKey(f.cashier, "/api/v1/orders", "kunci-yang-sama", cart(line(f.bag, 2))).WantProblem(409, "IDEMPOTENCY_KEY_REUSED")
	if n := f.count(t, `SELECT count(*) FROM orders`); n != 1 {
		t.Fatalf("order %d", n)
	}
	f.a.PostKey(f.owner, "/api/v1/orders", "kunci-yang-sama", cart(line(f.bag, 1))).Want(201)
}

func TestReservationHoldsStockAndClearsCatalogCache(t *testing.T) {
	f := setup(t)
	stock := func() (reserved, available int) {
		var it struct {
			Reserved  int `json:"stock_reserved"`
			Available int `json:"stock_available"`
		}
		f.a.Get(f.cashier, "/api/v1/items/"+f.bag.String()).Want(200).JSON(&it)
		return it.Reserved, it.Available
	}
	var page struct {
		Data []struct {
			ID        string `json:"id"`
			Available int    `json:"stock_available"`
		} `json:"data"`
	}
	f.a.Get(f.cashier, "/api/v1/items").Want(200).JSON(&page)

	o := f.create(t, f.cashier, cart(line(f.bag, 4)))
	if r, a := stock(); r != 4 || a != 1 {
		t.Fatalf("stok tertahan %d, tersedia %d", r, a)
	}
	f.a.Get(f.cashier, "/api/v1/items").Want(200).JSON(&page)
	for _, it := range page.Data {
		if it.ID == f.bag.String() && it.Available != 1 {
			t.Fatalf("cache katalog harus dibersihkan, tersedia %d", it.Available)
		}
	}
	f.a.PostKey(f.cashier, "/api/v1/orders", f.key(), cart(line(f.bag, 2))).WantProblem(409, "INSUFFICIENT_STOCK")
	f.a.Exec(`UPDATE orders SET status = 'expired' WHERE id = $1`, o.ID)
	if r, a := stock(); r != 0 || a != 5 {
		t.Fatalf("reservasi harus lepas: %d %d", r, a)
	}
}

func TestOrderDeductsOutstandingRefund(t *testing.T) {
	f := setup(t)
	first := f.debt(t, f.sari, 30000)
	second := f.debt(t, f.sari, 70000)

	var p orderBody
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(f.bag, 1))).Want(200).JSON(&p)
	if p.Recipients[0].DeductionIDR != 80000 || p.Recipients[0].AmountIDR != 0 || p.Recipients[1].RecoveredIDR != 80000 {
		t.Fatalf("rincian potongan salah: %+v", p.Recipients)
	}
	if p.Recipients[1].AmountIDR != 100000 || p.Recipients[0].AmountLamports != "0" {
		t.Fatalf("toko harus menerima semua: %+v", p.Recipients)
	}

	o := f.create(t, f.cashier, cart(line(f.bag, 1)))
	if o.Recipients[0].DeductionIDR != 80000 || o.Recipients[1].RecoveredIDR != 80000 {
		t.Fatalf("order salah: %+v", o.Recipients)
	}
	settled := func(id uuid.UUID) int64 {
		var v int64
		f.a.Scalar(&v, `SELECT COALESCE(sum(amount_idr), 0)::bigint FROM deduction_settlements WHERE deduction_id = $1`, id)
		return v
	}
	if settled(first) != 30000 || settled(second) != 50000 {
		t.Fatalf("yang terlama harus lunas lebih dulu: %d %d", settled(first), settled(second))
	}

	var next orderBody
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(f.bag, 1))).Want(200).JSON(&next)
	if next.Recipients[0].DeductionIDR != 20000 {
		t.Fatalf("sisa potongan salah: %+v", next.Recipients[0])
	}

	f.a.Exec(`UPDATE orders SET status = 'expired' WHERE id = $1`, o.ID)
	f.a.Post(f.cashier, "/api/v1/orders/preview", cart(line(f.bag, 1))).Want(200).JSON(&next)
	if next.Recipients[0].DeductionIDR != 80000 {
		t.Fatalf("order kedaluwarsa harus membuka potongan lagi: %+v", next.Recipients[0])
	}
}

func TestReadScope(t *testing.T) {
	f := setup(t)
	mine := f.create(t, f.cashier, cart(line(f.bag, 1)))
	other := f.a.NewUser(f.store.ID, auth.RoleCashier)
	theirs := f.create(t, other, cart(line(f.jam, 1)))

	f.a.Get(f.cashier, "/api/v1/orders/"+mine.ID).Want(200)
	f.a.Get(f.cashier, "/api/v1/orders/"+theirs.ID).WantProblem(404, "NOT_FOUND")
	f.a.Get(f.cashier, "/api/v1/orders/"+theirs.ID+"/status").WantProblem(404, "NOT_FOUND")
	f.a.Get(f.owner, "/api/v1/orders/"+theirs.ID).Want(200)

	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	f.a.Get(f.cashier, "/api/v1/orders").Want(200).JSON(&list)
	if len(list.Data) != 1 || list.Data[0].ID != mine.ID {
		t.Fatalf("kasir hanya melihat ordernya: %+v", list)
	}
	f.a.Get(f.owner, "/api/v1/orders").Want(200).JSON(&list)
	if len(list.Data) != 2 {
		t.Fatalf("pemilik melihat semua: %+v", list)
	}

	store := f.a.NewStore()
	stranger := f.a.NewUser(store.ID, auth.RoleOwner)
	f.a.Get(stranger, "/api/v1/orders/"+mine.ID).WantProblem(404, "NOT_FOUND")
	f.a.Get(stranger, "/api/v1/orders/"+mine.ID+"/status").WantProblem(404, "NOT_FOUND")
	f.a.Get(stranger, "/api/v1/orders").Want(200).JSON(&list)
	if len(list.Data) != 0 {
		t.Fatalf("toko lain melihat order: %+v", list)
	}

	f.a.Get(f.sari, "/api/v1/orders").WantProblem(403, "FORBIDDEN")
	f.a.Post(f.sari, "/api/v1/orders/preview", cart(line(f.bag, 1))).WantProblem(403, "FORBIDDEN")
	f.a.Get(f.owner, "/api/v1/orders/abc").WantProblem(400, "VALIDATION_FAILED")
}

func TestListOrders(t *testing.T) {
	f := setup(t)
	for i := 0; i < 3; i++ {
		f.create(t, f.cashier, cart(line(f.bag, 1)))
	}
	last := f.create(t, f.cashier, cart(line(f.jam, 3)))
	f.a.Exec(`UPDATE orders SET status = 'expired' WHERE id = $1`, last.ID)

	type row struct {
		ID            string `json:"id"`
		TotalIDR      int64  `json:"total_idr"`
		TotalLamports string `json:"total_lamports"`
		CommissionIDR int64  `json:"commission_idr"`
		ItemCount     int    `json:"item_count"`
	}
	type page struct {
		Data       []row   `json:"data"`
		NextCursor *string `json:"next_cursor"`
	}
	seen := map[string]row{}
	path := "/api/v1/orders?limit=3"
	for {
		var p page
		f.a.Get(f.owner, path).Want(200).JSON(&p)
		for _, r := range p.Data {
			if _, dup := seen[r.ID]; dup {
				t.Fatalf("order %s muncul dua kali", r.ID)
			}
			seen[r.ID] = r
		}
		if p.NextCursor == nil {
			break
		}
		path = "/api/v1/orders?limit=3&cursor=" + url.QueryEscape(*p.NextCursor)
	}
	if len(seen) != 4 {
		t.Fatalf("order %d", len(seen))
	}
	got := seen[last.ID]
	if got.TotalIDR != 150000 || got.CommissionIDR != 22500 || got.ItemCount != 3 || got.TotalLamports != "60000000" {
		t.Fatalf("ringkasan salah: %+v", got)
	}

	var p page
	f.a.Get(f.owner, "/api/v1/orders?status=expired").Want(200).JSON(&p)
	if len(p.Data) != 1 {
		t.Fatalf("filter status %d", len(p.Data))
	}
	future := url.QueryEscape(time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	past := url.QueryEscape(time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
	f.a.Get(f.owner, "/api/v1/orders?from="+future).Want(200).JSON(&p)
	if len(p.Data) != 0 {
		t.Fatalf("filter from %d", len(p.Data))
	}
	f.a.Get(f.owner, "/api/v1/orders?to="+past).Want(200).JSON(&p)
	if len(p.Data) != 0 {
		t.Fatalf("filter to %d", len(p.Data))
	}
	f.a.Get(f.owner, "/api/v1/orders?from="+past+"&to="+future).Want(200).JSON(&p)
	if len(p.Data) != 4 {
		t.Fatalf("filter periode %d", len(p.Data))
	}
	f.a.Get(f.owner, "/api/v1/orders?status=selesai").WantProblem(400, "VALIDATION_FAILED")
	f.a.Get(f.owner, "/api/v1/orders?from=kemarin").WantProblem(400, "VALIDATION_FAILED")
}

func TestOrderStatus(t *testing.T) {
	f := setup(t)
	o := f.create(t, f.cashier, cart(line(f.bag, 1)))
	var v struct {
		Status      string  `json:"status"`
		Signature   *string `json:"signature"`
		ExplorerURL *string `json:"explorer_url"`
	}
	r := f.a.Get(f.cashier, "/api/v1/orders/"+o.ID+"/status").Want(200)
	r.JSON(&v)
	if v.Status != "pending" || v.Signature != nil {
		t.Fatalf("status salah: %+v", v)
	}
	if !strings.Contains(r.Header.Get("Cache-Control"), "no-store") {
		t.Fatalf("status tidak boleh di-cache: %s", r.Header.Get("Cache-Control"))
	}
	sig := testkit.NewSignature()
	f.a.Exec(`UPDATE orders SET status = 'confirmed', signature = $2, payer_wallet = $3, confirmed_at = now() WHERE id = $1`,
		o.ID, sig, testkit.NewWallet().Address)
	f.a.Get(f.cashier, "/api/v1/orders/"+o.ID+"/status").Want(200).JSON(&v)
	if v.Status != "confirmed" || v.ExplorerURL == nil || !strings.Contains(*v.ExplorerURL, sig) {
		t.Fatalf("status baru salah: %+v", v)
	}
}

func TestLastStockGoesToExactlyOneBuyer(t *testing.T) {
	f := setup(t)
	last := f.a.NewItem(f.sari.ConsignorID, 100000, 2000, 3)
	const buyers = 12
	statuses := make([]int, buyers)
	var wg sync.WaitGroup
	for i := range buyers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := f.a.PostKey(f.cashier, "/api/v1/orders", fmt.Sprintf("kunci-paralel-%d", i), cart(line(last, 1)))
			statuses[i] = r.Status
		}()
	}
	wg.Wait()
	counts := map[int]int{}
	for _, s := range statuses {
		counts[s]++
	}
	if counts[201] != 3 || counts[409] != buyers-3 {
		t.Fatalf("hasil paralel salah: %v", counts)
	}
	if n := f.count(t, `SELECT COALESCE(sum(quantity), 0)::int FROM order_lines WHERE item_id = $1`, last); n != 3 {
		t.Fatalf("stok tertahan %d", n)
	}
}

func TestDuplicateRequestsCreateOneOrder(t *testing.T) {
	f := setup(t)
	last := f.a.NewItem(f.sari.ConsignorID, 100000, 2000, 1)
	const requests = 8
	ids := make([]string, requests)
	statuses := make([]int, requests)
	var wg sync.WaitGroup
	for i := range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := f.a.PostKey(f.cashier, "/api/v1/orders", "kunci-kembar", cart(line(last, 1)))
			statuses[i] = r.Status
			var o orderBody
			r.JSON(&o)
			ids[i] = o.ID
		}()
	}
	wg.Wait()
	for i := range ids {
		if statuses[i] != 201 || ids[i] != ids[0] {
			t.Fatalf("permintaan kembar harus mendapat order yang sama: %v %v", statuses, ids)
		}
	}
	if n := f.count(t, `SELECT count(*) FROM orders`); n != 1 {
		t.Fatalf("order %d", n)
	}
}

func TestParallelOrdersDoNotSettleDeductionTwice(t *testing.T) {
	f := setup(t)
	deduction := f.debt(t, f.sari, 50000)
	items := make([]uuid.UUID, 6)
	for i := range items {
		items[i] = f.a.NewItem(f.sari.ConsignorID, 100000, 2000, 5)
	}
	var wg sync.WaitGroup
	for i, it := range items {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.a.PostKey(f.cashier, "/api/v1/orders", fmt.Sprintf("kunci-potongan-%d", i), cart(line(it, 1)))
		}()
	}
	wg.Wait()
	if n := f.count(t, `SELECT count(*) FROM orders WHERE status = 'pending'`); n != len(items) {
		t.Fatalf("order %d", n)
	}
	if n := f.count(t, `SELECT COALESCE(sum(amount_idr), 0)::int FROM deduction_settlements WHERE deduction_id = $1`, deduction); n != 50000 {
		t.Fatalf("potongan terlunasi %d, seharusnya 50000", n)
	}
}

func TestOppositeCartsDoNotDeadlock(t *testing.T) {
	f := setup(t)
	f.a.Exec(`UPDATE items SET stock_on_hand = 100`)
	var wg sync.WaitGroup
	statuses := make([]int, 20)
	for i := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := cart(line(f.bag, 1), line(f.jam, 1))
			if i%2 == 0 {
				body = cart(line(f.jam, 1), line(f.bag, 1))
			}
			statuses[i] = f.a.PostKey(f.cashier, "/api/v1/orders", fmt.Sprintf("kunci-silang-%d", i), body).Status
		}()
	}
	wg.Wait()
	for _, s := range statuses {
		if s != 201 {
			t.Fatalf("status %v", statuses)
		}
	}
}
