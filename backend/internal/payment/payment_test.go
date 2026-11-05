package payment_test

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/config"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

type orderBody struct {
	ID         string `json:"id"`
	Status     string `json:"status"`
	Recipients []struct {
		WalletAddress  string `json:"wallet_address"`
		AmountLamports string `json:"amount_lamports"`
	} `json:"recipients"`
	Lines []struct {
		Certificate *struct {
			Status string `json:"status"`
		} `json:"certificate"`
	} `json:"lines"`
	Payment struct {
		Reference   string  `json:"reference"`
		PayerWallet *string `json:"payer_wallet"`
		Signature   *string `json:"signature"`
		ExplorerURL *string `json:"explorer_url"`
		FailureCode *string `json:"failure_code"`
	} `json:"payment"`
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

func setup(t *testing.T, opts ...testkit.Option) *fixture {
	t.Helper()
	a := testkit.NewApp(t, opts...)
	store := a.NewStore()
	f := &fixture{
		a: a, store: store, buyer: testkit.NewWallet(),
		owner:   a.NewUser(store.ID, auth.RoleOwner),
		cashier: a.NewUser(store.ID, auth.RoleCashier),
		sari:    a.NewConsignor(store.ID),
		budi:    a.NewConsignor(store.ID),
	}
	f.bag = a.NewItem(f.sari.ConsignorID, 100000, 2000, 5)
	f.jam = a.NewItem(f.budi.ConsignorID, 50000, 1500, 5)
	return f
}

func (f *fixture) order(t *testing.T, ids ...uuid.UUID) orderBody {
	t.Helper()
	lines := make([]map[string]any, len(ids))
	for i, id := range ids {
		lines[i] = map[string]any{"item_id": id, "quantity": 1}
	}
	f.keys++
	var o orderBody
	f.a.PostKey(f.cashier, "/api/v1/orders", "kunci-bayar-"+strings.Repeat("x", f.keys), map[string]any{"lines": lines}).Want(201).JSON(&o)
	return o
}

func (f *fixture) get(t *testing.T, id string) orderBody {
	t.Helper()
	var o orderBody
	f.a.Get(f.owner, "/api/v1/orders/"+id).Want(200).JSON(&o)
	return o
}

// build meminta transaksi dari API seperti wallet sungguhan.
func (f *fixture) build(t *testing.T, id string) *solana.Transaction {
	t.Helper()
	var res struct {
		Transaction string `json:"transaction"`
		Message     string `json:"message"`
	}
	f.a.Public("POST", "/api/v1/solana-pay/orders/"+id, map[string]any{"account": f.buyer.Address}).Want(200).JSON(&res)
	tx, _, err := solana.DecodeBase64(res.Transaction)
	if err != nil {
		t.Fatal(err)
	}
	tx.Signature = testkit.NewSignature()
	return tx
}

// pay menyusun transaksi lewat API lalu membuatnya terlihat di jaringan.
func (f *fixture) pay(t *testing.T, o orderBody) *solana.Transaction {
	t.Helper()
	tx := f.build(t, o.ID)
	f.a.Solana.Publish(o.Payment.Reference, tx)
	return tx
}

func (f *fixture) stock(t *testing.T, id uuid.UUID) (onHand, reserved int) {
	t.Helper()
	var it struct {
		OnHand   int `json:"stock_on_hand"`
		Reserved int `json:"stock_reserved"`
	}
	f.a.Get(f.owner, "/api/v1/items/"+id.String()).Want(200).JSON(&it)
	return it.OnHand, it.Reserved
}

func TestPublicOrderHidesPrivateData(t *testing.T) {
	f := setup(t)
	f.a.Exec(`UPDATE consignors SET name = 'Nama Rahasia Penitip' WHERE id = $1`, f.sari.ConsignorID)
	o := f.order(t, f.bag)
	r := f.a.Public("GET", "/api/v1/public/orders/"+o.ID, nil).Want(200)
	var p struct {
		Status    string `json:"status"`
		StoreName string `json:"store_name"`
		Network   string `json:"network"`
		TotalIDR  int64  `json:"total_idr"`
		Lines     []struct {
			LineTotalIDR int64 `json:"line_total_idr"`
		} `json:"lines"`
	}
	r.JSON(&p)
	if p.Network != "devnet" || p.StoreName != "Toko Titip Bersama" || p.TotalIDR != 100000 || p.Lines[0].LineTotalIDR != 100000 {
		t.Fatalf("layar bayar salah: %+v", p)
	}
	body := string(r.Body)
	for _, secret := range []string{"Nama Rahasia Penitip", f.sari.Wallet.Address, f.store.Wallet.Address, f.sari.Email, f.cashier.Email} {
		if strings.Contains(body, secret) {
			t.Fatalf("data pribadi bocor: %s", secret)
		}
	}
	f.pay(t, o)
	f.a.Tick()
	r = f.a.Public("GET", "/api/v1/public/orders/"+o.ID, nil).Want(200)
	if strings.Contains(string(r.Body), f.buyer.Address) {
		t.Fatal("wallet pembayar tidak boleh tampil di tautan publik")
	}
	var done struct {
		Status      string  `json:"status"`
		ExplorerURL *string `json:"explorer_url"`
	}
	r.JSON(&done)
	if done.Status != "confirmed" || done.ExplorerURL == nil || !strings.Contains(*done.ExplorerURL, "cluster=devnet") {
		t.Fatalf("bukti pembayaran salah: %+v", done)
	}
	f.a.Public("GET", "/api/v1/public/orders/"+uuid.NewString(), nil).WantProblem(404, "NOT_FOUND")
	f.a.Public("GET", "/api/v1/public/orders/abc", nil).WantProblem(400, "VALIDATION_FAILED")
}

func TestSolanaPayMetadata(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	path := "/api/v1/solana-pay/orders/" + o.ID
	var m struct {
		Label string `json:"label"`
		Icon  string `json:"icon"`
	}
	f.a.Public("GET", path, nil).Want(200).JSON(&m)
	if m.Label != "Toko Titip Bersama" || m.Icon != "https://app.test/icon.png" {
		t.Fatalf("metadata salah: %+v", m)
	}
	f.a.Exec(`UPDATE stores SET icon_url = 'https://cdn.test/toko.png' WHERE id = $1`, f.store.ID)
	f.a.Public("GET", path, nil).Want(200).JSON(&m)
	if m.Icon != "https://cdn.test/toko.png" {
		t.Fatalf("ikon toko harus dipakai: %s", m.Icon)
	}
	var e struct {
		Message string `json:"message"`
	}
	f.a.Public("GET", "/api/v1/solana-pay/orders/"+uuid.NewString(), nil).Want(404).JSON(&e)
	if e.Message == "" {
		t.Fatal("error Solana Pay harus punya message")
	}
	f.a.Public("GET", "/api/v1/solana-pay/orders/abc", nil).Want(400)
}

func TestSolanaPayAllowsAnyWalletOrigin(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	path := "/api/v1/solana-pay/orders/" + o.ID
	r := f.a.Do(testkit.Request{Method: "OPTIONS", Path: path, Raw: true, Headers: map[string]string{
		"Origin": "https://wallet.example", "Access-Control-Request-Method": "POST",
	}}).Want(204)
	if r.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("preflight: %v", r.Header)
	}
	r = f.a.Do(testkit.Request{Method: "GET", Path: path, Headers: map[string]string{"Origin": "https://wallet.example"}}).Want(200)
	if r.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("GET: %v", r.Header)
	}
}

func TestSolanaPayBuildsSplitTransaction(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag, f.jam)
	tx := f.build(t, o.ID)
	if tx.FeePayer != f.buyer.Address {
		t.Fatalf("fee payer %s", tx.FeePayer)
	}
	want := []solana.Transfer{
		{To: f.sari.Wallet.Address, Lamports: 32000000},
		{To: f.budi.Wallet.Address, Lamports: 17000000},
		{To: f.store.Wallet.Address, Lamports: 11000000},
	}
	if !solana.Matches(tx, o.Payment.Reference, f.buyer.Address, want) {
		t.Fatalf("transaksi salah: %+v", tx)
	}
}

func TestSolanaPaySkipsZeroRecipients(t *testing.T) {
	f := setup(t)
	free := f.a.NewItem(f.sari.ConsignorID, 100000, 0, 5)
	o := f.order(t, free)
	tx := f.build(t, o.ID)
	if len(tx.Transfers) != 1 || tx.Transfers[0].To != f.sari.Wallet.Address {
		t.Fatalf("toko tanpa komisi tidak boleh mendapat transfer: %+v", tx.Transfers)
	}
	f.a.Solana.Publish(o.Payment.Reference, tx)
	f.a.Tick()
	if s := f.get(t, o.ID).Status; s != "confirmed" {
		t.Fatalf("status %s", s)
	}
}

func TestSolanaPayIgnoresExtraFieldsAndRejectsBadAccount(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	path := "/api/v1/solana-pay/orders/" + o.ID
	f.a.Public("POST", path, map[string]any{"account": f.buyer.Address, "extra": "boleh"}).Want(200)
	for _, body := range []any{map[string]any{}, map[string]any{"account": "abc"}, "bukan json"} {
		f.a.Public("POST", path, body).Want(400)
	}
	f.a.Public("POST", "/api/v1/solana-pay/orders/abc", map[string]any{"account": f.buyer.Address}).Want(400)
	f.a.Public("POST", "/api/v1/solana-pay/orders/"+uuid.NewString(), map[string]any{"account": f.buyer.Address}).Want(404)
}

func TestSolanaPayRejectsClosedOrders(t *testing.T) {
	f := setup(t)
	body := map[string]any{"account": f.buyer.Address}

	paid := f.order(t, f.bag)
	f.pay(t, paid)
	f.a.Tick()
	f.a.Public("POST", "/api/v1/solana-pay/orders/"+paid.ID, body).Want(409)

	failed := f.order(t, f.bag)
	f.a.Exec(`UPDATE orders SET status = 'failed', failure_code = 'TRANSACTION_FAILED' WHERE id = $1`, failed.ID)
	f.a.Public("POST", "/api/v1/solana-pay/orders/"+failed.ID, body).Want(409)

	expired := f.order(t, f.bag)
	f.a.Exec(`UPDATE orders SET status = 'expired' WHERE id = $1`, expired.ID)
	f.a.Public("POST", "/api/v1/solana-pay/orders/"+expired.ID, body).Want(410)

	late := f.order(t, f.bag)
	f.a.Clock.Advance(5 * time.Minute)
	f.a.Public("POST", "/api/v1/solana-pay/orders/"+late.ID, body).Want(410)
	if s := f.get(t, late.ID).Status; s != "pending" {
		t.Fatalf("order tetap pending selama masa tenggang: %s", s)
	}
}

func TestSolanaPayReportsRPCFailure(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	f.a.Solana.SetErr(errors.New("RPC mati"))
	var e struct {
		Message string `json:"message"`
	}
	f.a.Public("POST", "/api/v1/solana-pay/orders/"+o.ID, map[string]any{"account": f.buyer.Address}).Want(503).JSON(&e)
	if e.Message == "" || strings.Contains(e.Message, "RPC mati") {
		t.Fatalf("pesan salah: %q", e.Message)
	}
}

func TestSolanaPayRateLimit(t *testing.T) {
	f := setup(t, func(c *config.Config) { c.RateLimitOrder = 2 })
	o := f.order(t, f.bag)
	path := "/api/v1/solana-pay/orders/" + o.ID
	f.a.Public("GET", path, nil).Want(200)
	f.a.Public("GET", path, nil).Want(200)
	r := f.a.Public("GET", path, nil).Want(429)
	var e struct {
		Message string `json:"message"`
	}
	r.JSON(&e)
	if e.Message == "" || r.Header.Get("Retry-After") == "" {
		t.Fatalf("429 salah: %s %v", r.Body, r.Header)
	}
	f.a.Public("GET", "/api/v1/public/orders/"+o.ID, nil).WantProblem(429, "RATE_LIMITED")
}

func TestWorkerConfirmsPayment(t *testing.T) {
	f := setup(t)
	f.a.Exec(`UPDATE items SET certificate_enabled = true WHERE id = $1`, f.bag)
	o := f.order(t, f.bag, f.jam)
	if h, r := f.stock(t, f.bag); h != 5 || r != 1 {
		t.Fatalf("stok sebelum bayar: %d %d", h, r)
	}
	tx := f.pay(t, o)
	f.a.Tick()

	got := f.get(t, o.ID)
	if got.Status != "confirmed" || got.Payment.Signature == nil || *got.Payment.Signature != tx.Signature {
		t.Fatalf("order salah: %+v", got)
	}
	if got.Payment.PayerWallet == nil || *got.Payment.PayerWallet != f.buyer.Address {
		t.Fatalf("pembayar salah: %+v", got.Payment)
	}
	if got.Payment.ExplorerURL == nil || !strings.Contains(*got.Payment.ExplorerURL, tx.Signature) {
		t.Fatalf("tautan salah: %+v", got.Payment)
	}
	if h, r := f.stock(t, f.bag); h != 4 || r != 0 {
		t.Fatalf("stok sesudah bayar: %d %d", h, r)
	}
	if got.Lines[0].Certificate == nil || got.Lines[0].Certificate.Status != "pending" || got.Lines[1].Certificate != nil {
		t.Fatalf("sertifikat salah: %+v", got.Lines)
	}
	f.a.Tick()
	if h, _ := f.stock(t, f.bag); h != 4 {
		t.Fatalf("putaran berikutnya tidak boleh memotong stok lagi: %d", h)
	}
}

func TestWorkerClearsCatalogCache(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	read := func() int {
		var page struct {
			Data []struct {
				ID     string `json:"id"`
				OnHand int    `json:"stock_on_hand"`
			} `json:"data"`
		}
		f.a.Get(f.cashier, "/api/v1/items").Want(200).JSON(&page)
		for _, it := range page.Data {
			if it.ID == f.bag.String() {
				return it.OnHand
			}
		}
		t.Fatal("barang tidak ada di katalog")
		return 0
	}
	if read() != 5 {
		t.Fatal("stok awal salah")
	}
	f.pay(t, o)
	f.a.Tick()
	if n := read(); n != 4 {
		t.Fatalf("katalog masih memakai cache lama: %d", n)
	}
}

func TestWorkerRejectsMismatch(t *testing.T) {
	cases := map[string]func(f *fixture, tx *solana.Transaction){
		"jumlah kurang":   func(_ *fixture, tx *solana.Transaction) { tx.Transfers[0].Lamports-- },
		"penerima lain":   func(_ *fixture, tx *solana.Transaction) { tx.Transfers[0].To = testkit.NewWallet().Address },
		"transfer hilang": func(_ *fixture, tx *solana.Transaction) { tx.Transfers = tx.Transfers[:1] },
		"pengirim bukan fee payer": func(_ *fixture, tx *solana.Transaction) {
			tx.Transfers[0].From = testkit.NewWallet().Address
		},
		"transfer tambahan": func(f *fixture, tx *solana.Transaction) {
			tx.Transfers = append(tx.Transfers, solana.Transfer{From: f.buyer.Address, To: f.sari.Wallet.Address, Lamports: 1})
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			o := f.order(t, f.bag)
			tx := f.build(t, o.ID)
			mutate(f, tx)
			f.a.Solana.Publish(o.Payment.Reference, tx)
			f.a.Tick()
			got := f.get(t, o.ID)
			if got.Status != "failed" || got.Payment.FailureCode == nil || *got.Payment.FailureCode != "VALIDATION_MISMATCH" {
				t.Fatalf("order salah: %+v", got)
			}
			if got.Payment.Signature == nil || *got.Payment.Signature != tx.Signature {
				t.Fatalf("signature harus tersimpan untuk penelusuran: %+v", got.Payment)
			}
			if h, r := f.stock(t, f.bag); h != 5 || r != 0 {
				t.Fatalf("reservasi harus lepas tanpa memotong stok: %d %d", h, r)
			}
		})
	}
}

func TestWorkerRejectsMissingReference(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	tx := f.build(t, o.ID)
	keys := tx.AccountKeys[:0]
	for _, k := range tx.AccountKeys {
		if k != o.Payment.Reference {
			keys = append(keys, k)
		}
	}
	tx.AccountKeys = keys
	f.a.Solana.Publish(o.Payment.Reference, tx)
	f.a.Tick()
	if s := f.get(t, o.ID).Status; s != "failed" {
		t.Fatalf("status %s", s)
	}
}

func TestWorkerMarksFailedTransaction(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	tx := f.build(t, o.ID)
	tx.Failed = true
	f.a.Solana.Publish(o.Payment.Reference, tx)
	f.a.Tick()
	got := f.get(t, o.ID)
	if got.Status != "failed" || got.Payment.FailureCode == nil || *got.Payment.FailureCode != "TRANSACTION_FAILED" {
		t.Fatalf("order salah: %+v", got)
	}
}

func TestWorkerPrefersValidTransactionOverJunk(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	junk := f.build(t, o.ID)
	junk.Transfers[0].Lamports = 1
	f.a.Solana.Publish(o.Payment.Reference, junk)
	good := f.pay(t, o)
	f.a.Tick()
	got := f.get(t, o.ID)
	if got.Status != "confirmed" || *got.Payment.Signature != good.Signature {
		t.Fatalf("transaksi sah harus menang: %+v", got)
	}
}

func TestWorkerExpiresOnlyAfterGrace(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	f.a.Clock.Advance(6 * time.Minute)
	f.a.Tick()
	if s := f.get(t, o.ID).Status; s != "pending" {
		t.Fatalf("masih dalam masa tenggang: %s", s)
	}
	f.a.Clock.Advance(time.Minute)
	f.a.Tick()
	if s := f.get(t, o.ID).Status; s != "expired" {
		t.Fatalf("status %s", s)
	}
	if h, r := f.stock(t, f.bag); h != 5 || r != 0 {
		t.Fatalf("stok %d %d", h, r)
	}
}

func TestWorkerConfirmsDuringGrace(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	tx := f.build(t, o.ID)
	f.a.Clock.Advance(6 * time.Minute)
	f.a.Solana.Publish(o.Payment.Reference, tx)
	f.a.Tick()
	if s := f.get(t, o.ID).Status; s != "confirmed" {
		t.Fatalf("status %s", s)
	}
}

func TestWorkerDoesNotExpireWhileRPCIsDown(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	f.a.Clock.Advance(time.Hour)
	f.a.Solana.SetErr(errors.New("RPC mati"))
	f.a.Tick()
	f.a.Solana.SetErr(nil)
	if s := f.get(t, o.ID).Status; s != "pending" {
		t.Fatalf("status %s", s)
	}
	f.a.Tick()
	if s := f.get(t, o.ID).Status; s != "expired" {
		t.Fatalf("status %s", s)
	}
}

func TestWorkerWaitsForUnseenTransaction(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	tx := f.build(t, o.ID)
	f.a.Solana.PublishSignature(o.Payment.Reference, tx.Signature)
	f.a.Tick()
	if s := f.get(t, o.ID).Status; s != "pending" {
		t.Fatalf("status %s", s)
	}
	f.a.Clock.Advance(8 * time.Minute)
	f.a.Tick()
	if s := f.get(t, o.ID).Status; s != "expired" {
		t.Fatalf("transaksi yang tidak pernah terbaca tidak boleh menahan stok: %s", s)
	}
}

func TestWorkerTreatsUnreadableTransactionAsMismatch(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	tx := f.build(t, o.ID)
	f.a.Solana.PublishMalformed(o.Payment.Reference, tx.Signature)
	f.a.Tick()
	got := f.get(t, o.ID)
	if got.Status != "failed" || *got.Payment.FailureCode != "VALIDATION_MISMATCH" {
		t.Fatalf("order salah: %+v", got)
	}
}

func TestParallelWorkersConfirmOnce(t *testing.T) {
	f := setup(t)
	f.a.Exec(`UPDATE items SET certificate_enabled = true WHERE id = $1`, f.bag)
	o := f.order(t, f.bag)
	f.pay(t, o)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.a.Tick()
		}()
	}
	wg.Wait()
	if h, r := f.stock(t, f.bag); h != 4 || r != 0 {
		t.Fatalf("stok %d %d", h, r)
	}
	var n int
	f.a.Scalar(&n, `SELECT count(*) FROM certificates`)
	if n != 1 {
		t.Fatalf("sertifikat %d", n)
	}
}

func TestOneTransactionCannotPayTwoOrders(t *testing.T) {
	f := setup(t)
	first := f.order(t, f.bag)
	second := f.order(t, f.bag)
	tx := f.build(t, first.ID)
	tx.AccountKeys = append(tx.AccountKeys, second.Payment.Reference)
	f.a.Solana.Publish(first.Payment.Reference, tx)
	f.a.Solana.Publish(second.Payment.Reference, tx)
	f.a.Tick()
	a, b := f.get(t, first.ID), f.get(t, second.ID)
	statuses := map[string]int{a.Status: 1}
	statuses[b.Status]++
	if statuses["confirmed"] != 1 || statuses["failed"] != 1 {
		t.Fatalf("status %s dan %s", a.Status, b.Status)
	}
}

func TestExpiredOrderReopensDeduction(t *testing.T) {
	f := setup(t)
	it := f.a.NewItem(f.sari.ConsignorID, 40000, 0, 1)
	orderID, lineID, refund := uuid.New(), uuid.New(), uuid.New()
	f.a.Exec(`INSERT INTO orders (id, created_by, status, reference, idr_per_sol, rate_quoted_at, payer_wallet, signature,
		idempotency_key, request_hash, expires_at, confirmed_at)
		VALUES ($1, $2, 'confirmed', $3, 2500000, now(), $4, $5, 'kunci-lama-1', 'h', now(), now())`,
		orderID, f.cashier.ID, uuid.NewString(), f.buyer.Address, testkit.NewSignature())
	f.a.Exec(`INSERT INTO order_lines (id, order_id, item_id, quantity, unit_price_idr, commission_bps) VALUES ($1, $2, $3, 1, 40000, 0)`, lineID, orderID, it)
	f.a.Exec(`INSERT INTO refunds (id, order_line_id, created_by, status, amount_idr, amount_lamports, consignor_deduction_idr,
		source_wallet, reference, signature, idempotency_key, request_hash, expires_at, completed_at)
		VALUES ($1, $2, $3, 'completed', 40000, 1, 40000, $4, $5, $6, 'kunci-lama-2', 'h', now(), now())`,
		refund, lineID, f.owner.ID, f.store.Wallet.Address, uuid.NewString(), testkit.NewSignature())
	f.a.Exec(`INSERT INTO deductions (id, refund_id) VALUES ($1, $2)`, uuid.New(), refund)

	o := f.order(t, f.bag)
	f.a.Clock.Advance(8 * time.Minute)
	f.a.Tick()
	if s := f.get(t, o.ID).Status; s != "expired" {
		t.Fatalf("status %s", s)
	}
	var p struct {
		Recipients []struct {
			DeductionIDR int64 `json:"deduction_idr"`
		} `json:"recipients"`
	}
	f.a.Post(f.cashier, "/api/v1/orders/preview", map[string]any{"lines": []any{map[string]any{"item_id": f.bag, "quantity": 1}}}).Want(200).JSON(&p)
	if p.Recipients[0].DeductionIDR != 40000 {
		t.Fatalf("potongan harus terbuka lagi: %+v", p)
	}
}

func TestReadScopeForOrdersInPaymentFlow(t *testing.T) {
	f := setup(t)
	o := f.order(t, f.bag)
	f.pay(t, o)
	f.a.Tick()
	var v struct {
		Status string `json:"status"`
	}
	f.a.Get(f.cashier, "/api/v1/orders/"+o.ID+"/status").Want(200).JSON(&v)
	if v.Status != "confirmed" {
		t.Fatalf("status %s", v.Status)
	}
}
