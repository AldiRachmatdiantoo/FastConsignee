package refund_test

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
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

type orderBody struct {
	ID    string `json:"id"`
	Lines []struct {
		ID               string `json:"id"`
		RefundedQuantity int    `json:"refunded_quantity"`
	} `json:"lines"`
	Refunds []struct {
		ID string `json:"id"`
	} `json:"refunds"`
	Payment struct {
		Reference string `json:"reference"`
	} `json:"payment"`
}

type draftBody struct {
	ID                string `json:"id"`
	OrderID           string `json:"order_id"`
	Status            string `json:"status"`
	AmountIDR         int64  `json:"amount_idr"`
	AmountLamports    string `json:"amount_lamports"`
	SourceWallet      string `json:"source_wallet"`
	DestinationWallet string `json:"destination_wallet"`
	Reference         string `json:"reference"`
	Deduction         struct {
		AmountIDR int64 `json:"amount_idr"`
	} `json:"deduction"`
	Reason      *string `json:"reason"`
	Signature   *string `json:"signature"`
	ExplorerURL *string `json:"explorer_url"`
	Transaction string  `json:"transaction"`
}

type fixture struct {
	a       *testkit.App
	store   testkit.Store
	owner   testkit.User
	cashier testkit.User
	sari    testkit.User
	bag     uuid.UUID
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
	}
	f.bag = a.NewItem(f.sari.ConsignorID, 100000, 2000, 50)
	return f
}

func (f *fixture) key() string {
	f.keys++
	return fmt.Sprintf("kunci-refund-%d", f.keys)
}

// paid membuat order lunas lewat alur lengkap.
func (f *fixture) paid(t *testing.T, qty int) orderBody {
	t.Helper()
	var o orderBody
	f.a.PostKey(f.cashier, "/api/v1/orders", f.key(), map[string]any{
		"lines": []any{map[string]any{"item_id": f.bag, "quantity": qty}},
	}).Want(201).JSON(&o)
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
	return o
}

func (f *fixture) refund(t *testing.T, o orderBody) draftBody {
	t.Helper()
	var d draftBody
	f.a.PostKey(f.owner, "/api/v1/orders/"+o.ID+"/refunds", f.key(), map[string]any{"order_line_id": o.Lines[0].ID}).Want(201).JSON(&d)
	return d
}

// send membuat transaksi refund terlihat di jaringan, seperti setelah wallet toko mengirimnya.
func (f *fixture) send(t *testing.T, d draftBody) *solana.Transaction {
	t.Helper()
	tx, _, err := solana.DecodeBase64(d.Transaction)
	if err != nil {
		t.Fatal(err)
	}
	tx.Signature = testkit.NewSignature()
	f.a.Solana.Publish(d.Reference, tx)
	return tx
}

func (f *fixture) read(t *testing.T, id string) draftBody {
	t.Helper()
	var d draftBody
	f.a.Get(f.owner, "/api/v1/refunds/"+id).Want(200).JSON(&d)
	return d
}

func (f *fixture) deductions(t *testing.T) int {
	t.Helper()
	var n int
	f.a.Scalar(&n, `SELECT count(*) FROM deductions`)
	return n
}

func TestCreateRefundBuildsStoreTransaction(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 1)
	r := f.a.PostKey(f.owner, "/api/v1/orders/"+o.ID+"/refunds", f.key(), map[string]any{
		"order_line_id": o.Lines[0].ID, "reason": "Barang rusak",
	}).Want(201)
	var d draftBody
	r.JSON(&d)
	if r.Header.Get("Location") != "/api/v1/refunds/"+d.ID {
		t.Fatalf("Location salah: %s", r.Header.Get("Location"))
	}
	if d.Status != "awaiting_signature" || d.AmountIDR != 100000 || d.AmountLamports != "40000000" || d.Deduction.AmountIDR != 80000 {
		t.Fatalf("refund salah: %+v", d)
	}
	if d.SourceWallet != f.store.Wallet.Address || d.DestinationWallet != f.buyer.Address {
		t.Fatalf("wallet salah: %+v", d)
	}
	if d.Reason == nil || *d.Reason != "Barang rusak" {
		t.Fatalf("alasan salah: %+v", d.Reason)
	}
	tx, _, err := solana.DecodeBase64(d.Transaction)
	if err != nil {
		t.Fatal(err)
	}
	want := []solana.Transfer{{To: f.buyer.Address, Lamports: 40000000}}
	if !solana.Matches(tx, d.Reference, f.store.Wallet.Address, want) {
		t.Fatalf("transaksi salah: %+v", tx)
	}
	if f.deductions(t) != 0 {
		t.Fatal("potongan baru tercatat setelah refund selesai")
	}
}

func TestRefundUsesLockedRate(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 1)
	f.a.Cache.DeletePrefix("rate:")
	f.a.Exec(`UPDATE items SET price_idr = 1`)
	d := f.refund(t, o)
	if d.AmountLamports != "40000000" || d.AmountIDR != 100000 {
		t.Fatalf("refund harus memakai nilai order: %+v", d)
	}
}

func TestRefundValidation(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 1)
	path := "/api/v1/orders/" + o.ID + "/refunds"
	body := map[string]any{"order_line_id": o.Lines[0].ID}
	f.a.Post(f.owner, path, body).WantProblem(400, "VALIDATION_FAILED")
	f.a.PostKey(f.owner, path, f.key(), map[string]any{}).WantProblem(400, "VALIDATION_FAILED")
	f.a.PostKey(f.owner, path, f.key(), map[string]any{"order_line_id": "abc"}).WantProblem(400, "VALIDATION_FAILED")
	f.a.PostKey(f.owner, path, f.key(), map[string]any{
		"order_line_id": o.Lines[0].ID, "reason": strings.Repeat("a", 501),
	}).WantProblem(400, "VALIDATION_FAILED")
	f.a.PostKey(f.owner, path, f.key(), map[string]any{"order_line_id": o.Lines[0].ID, "amount_idr": 1}).WantProblem(400, "VALIDATION_FAILED")
	f.a.PostKey(f.owner, "/api/v1/orders/abc/refunds", f.key(), body).WantProblem(400, "VALIDATION_FAILED")
	f.a.PostKey(f.owner, "/api/v1/orders/"+uuid.NewString()+"/refunds", f.key(), body).WantProblem(404, "NOT_FOUND")
	f.a.PostKey(f.owner, path, f.key(), map[string]any{"order_line_id": uuid.NewString()}).WantProblem(404, "NOT_FOUND")
}

func TestRefundRequiresConfirmedOrder(t *testing.T) {
	f := setup(t)
	var o orderBody
	f.a.PostKey(f.cashier, "/api/v1/orders", f.key(), map[string]any{
		"lines": []any{map[string]any{"item_id": f.bag, "quantity": 1}},
	}).Want(201).JSON(&o)
	f.a.PostKey(f.owner, "/api/v1/orders/"+o.ID+"/refunds", f.key(), map[string]any{"order_line_id": o.Lines[0].ID}).
		WantProblem(409, "ORDER_NOT_REFUNDABLE")
}

func TestRefundRolesAndStores(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 1)
	d := f.refund(t, o)
	body := map[string]any{"order_line_id": o.Lines[0].ID}
	f.a.PostKey(f.cashier, "/api/v1/orders/"+o.ID+"/refunds", f.key(), body).WantProblem(403, "FORBIDDEN")
	f.a.Get(f.cashier, "/api/v1/refunds").WantProblem(403, "FORBIDDEN")
	f.a.Get(f.sari, "/api/v1/refunds/"+d.ID).WantProblem(403, "FORBIDDEN")

	other := f.a.NewStore()
	stranger := f.a.NewUser(other.ID, auth.RoleOwner)
	f.a.PostKey(stranger, "/api/v1/orders/"+o.ID+"/refunds", f.key(), body).WantProblem(404, "NOT_FOUND")
	f.a.Get(stranger, "/api/v1/refunds/"+d.ID).WantProblem(404, "NOT_FOUND")
	f.a.Post(stranger, "/api/v1/refunds/"+d.ID+"/signature", map[string]any{"signature": testkit.NewSignature()}).WantProblem(404, "NOT_FOUND")
	var list struct {
		Data []draftBody `json:"data"`
	}
	f.a.Get(stranger, "/api/v1/refunds").Want(200).JSON(&list)
	if len(list.Data) != 0 {
		t.Fatalf("toko lain melihat refund: %+v", list)
	}
}

func TestRefundIsIdempotent(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 2)
	path := "/api/v1/orders/" + o.ID + "/refunds"
	body := map[string]any{"order_line_id": o.Lines[0].ID}
	var first, second draftBody
	f.a.PostKey(f.owner, path, "kunci-sama-refund", body).Want(201).JSON(&first)
	r := f.a.PostKey(f.owner, path, "kunci-sama-refund", body).Want(201)
	r.JSON(&second)
	if first.ID != second.ID || r.Header.Get("Idempotency-Replayed") != "true" || second.Transaction == "" {
		t.Fatalf("replay salah: %s %s %v", first.ID, second.ID, r.Header)
	}
	f.a.PostKey(f.owner, path, "kunci-sama-refund", map[string]any{"order_line_id": o.Lines[0].ID, "reason": "lain"}).
		WantProblem(409, "IDEMPOTENCY_KEY_REUSED")
}

func TestOnlyOneRefundRunsPerLine(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 2)
	f.refund(t, o)
	f.a.PostKey(f.owner, "/api/v1/orders/"+o.ID+"/refunds", f.key(), map[string]any{"order_line_id": o.Lines[0].ID}).
		WantProblem(409, "REFUND_IN_PROGRESS")
}

func TestParallelRefundsCreateOne(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 5)
	statuses := make([]int, 10)
	var wg sync.WaitGroup
	for i := range statuses {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i] = f.a.PostKey(f.owner, "/api/v1/orders/"+o.ID+"/refunds", fmt.Sprintf("kunci-paralel-%d", i),
				map[string]any{"order_line_id": o.Lines[0].ID}).Status
		}()
	}
	wg.Wait()
	counts := map[int]int{}
	for _, s := range statuses {
		counts[s]++
	}
	if counts[201] != 1 || counts[409] != 9 {
		t.Fatalf("hasil paralel salah: %v", counts)
	}
}

func TestWorkerCompletesRefundAndRecordsDeduction(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 1)
	d := f.refund(t, o)
	tx := f.send(t, d)
	f.a.Tick()
	got := f.read(t, d.ID)
	if got.Status != "completed" || got.Signature == nil || *got.Signature != tx.Signature {
		t.Fatalf("refund salah: %+v", got)
	}
	if got.ExplorerURL == nil || !strings.Contains(*got.ExplorerURL, tx.Signature) {
		t.Fatalf("tautan salah: %+v", got)
	}
	if f.deductions(t) != 1 {
		t.Fatal("potongan harus tercatat")
	}
	var detail orderBody
	f.a.Get(f.owner, "/api/v1/orders/"+o.ID).Want(200).JSON(&detail)
	if detail.Lines[0].RefundedQuantity != 1 || len(detail.Refunds) != 1 {
		t.Fatalf("detail order salah: %+v", detail)
	}
	f.a.PostKey(f.owner, "/api/v1/orders/"+o.ID+"/refunds", f.key(), map[string]any{"order_line_id": o.Lines[0].ID}).
		WantProblem(409, "ALREADY_REFUNDED")
}

func TestRefundDoesNotRestock(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 1)
	f.send(t, f.refund(t, o))
	f.a.Tick()
	var stock int
	f.a.Scalar(&stock, `SELECT stock_on_hand FROM items WHERE id = $1`, f.bag)
	if stock != 49 {
		t.Fatalf("stok %d", stock)
	}
}

func TestFullRefundOfALineEqualsWhatBuyerPaid(t *testing.T) {
	f := setup(t)
	odd := f.a.NewItem(f.sari.ConsignorID, 33333, 1500, 10)
	var o orderBody
	f.a.PostKey(f.cashier, "/api/v1/orders", f.key(), map[string]any{
		"lines": []any{map[string]any{"item_id": odd, "quantity": 3, "discount": map[string]any{"amount_idr": 1, "borne_by": "store"}}},
	}).Want(201).JSON(&o)
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

	var idr, deduction int64
	for range 3 {
		d := f.refund(t, o)
		idr += d.AmountIDR
		deduction += d.Deduction.AmountIDR
		f.send(t, d)
		f.a.Tick()
	}
	if idr != 99998 {
		t.Fatalf("total refund %d", idr)
	}
	if deduction != 84999 {
		t.Fatalf("total potongan %d", deduction)
	}
	f.a.PostKey(f.owner, "/api/v1/orders/"+o.ID+"/refunds", f.key(), map[string]any{"order_line_id": o.Lines[0].ID}).
		WantProblem(409, "ALREADY_REFUNDED")
}

func TestDeductionIsRecoveredFromNextSale(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 1)
	f.send(t, f.refund(t, o))
	f.a.Tick()

	var p struct {
		Recipients []struct {
			DeductionIDR int64 `json:"deduction_idr"`
			RecoveredIDR int64 `json:"recovered_idr"`
			AmountIDR    int64 `json:"amount_idr"`
		} `json:"recipients"`
	}
	f.a.Post(f.cashier, "/api/v1/orders/preview", map[string]any{"lines": []any{map[string]any{"item_id": f.bag, "quantity": 2}}}).Want(200).JSON(&p)
	if p.Recipients[0].DeductionIDR != 80000 || p.Recipients[0].AmountIDR != 80000 {
		t.Fatalf("penitip: %+v", p.Recipients[0])
	}
	if p.Recipients[1].RecoveredIDR != 80000 || p.Recipients[1].AmountIDR != 120000 {
		t.Fatalf("toko: %+v", p.Recipients[1])
	}
}

func TestSignatureEndpoint(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 1)
	d := f.refund(t, o)
	path := "/api/v1/refunds/" + d.ID + "/signature"
	f.a.Post(f.owner, path, map[string]any{"signature": "abc"}).WantProblem(400, "VALIDATION_FAILED")
	f.a.Post(f.owner, "/api/v1/refunds/"+uuid.NewString()+"/signature", map[string]any{"signature": testkit.NewSignature()}).WantProblem(404, "NOT_FOUND")

	tx, _, err := solana.DecodeBase64(d.Transaction)
	if err != nil {
		t.Fatal(err)
	}
	tx.Signature = testkit.NewSignature()
	var got draftBody
	f.a.Post(f.owner, path, map[string]any{"signature": tx.Signature}).Want(202).JSON(&got)
	if got.Status != "confirming" || got.Signature == nil {
		t.Fatalf("refund salah: %+v", got)
	}
	f.a.Post(f.owner, path, map[string]any{"signature": testkit.NewSignature()}).WantProblem(409, "REFUND_NOT_AWAITING_SIGNATURE")

	f.a.Tick()
	if s := f.read(t, d.ID).Status; s != "confirming" {
		t.Fatalf("belum terlihat di jaringan: %s", s)
	}
	f.a.Solana.AddTransaction(tx)
	f.a.Tick()
	if s := f.read(t, d.ID).Status; s != "completed" {
		t.Fatalf("worker harus memakai signature yang dilaporkan: %s", s)
	}
}

func TestSignatureCannotBeSharedBetweenRefunds(t *testing.T) {
	f := setup(t)
	a := f.refund(t, f.paid(t, 1))
	b := f.refund(t, f.paid(t, 1))
	sig := testkit.NewSignature()
	f.a.Post(f.owner, "/api/v1/refunds/"+a.ID+"/signature", map[string]any{"signature": sig}).Want(202)
	f.a.Post(f.owner, "/api/v1/refunds/"+b.ID+"/signature", map[string]any{"signature": sig}).WantProblem(400, "VALIDATION_FAILED")
}

func TestWorkerRejectsWrongRefundTransactions(t *testing.T) {
	cases := map[string]func(f *fixture, tx *solana.Transaction){
		"jumlah lain": func(_ *fixture, tx *solana.Transaction) { tx.Transfers[0].Lamports++ },
		"tujuan lain": func(_ *fixture, tx *solana.Transaction) { tx.Transfers[0].To = testkit.NewWallet().Address },
		"gagal":       func(_ *fixture, tx *solana.Transaction) { tx.Failed = true },
		"bukan toko": func(_ *fixture, tx *solana.Transaction) {
			w := testkit.NewWallet().Address
			tx.FeePayer, tx.Transfers[0].From = w, w
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := setup(t)
			d := f.refund(t, f.paid(t, 1))
			tx, _, err := solana.DecodeBase64(d.Transaction)
			if err != nil {
				t.Fatal(err)
			}
			tx.Signature = testkit.NewSignature()
			mutate(f, tx)
			f.a.Solana.Publish(d.Reference, tx)
			f.a.Tick()
			if s := f.read(t, d.ID).Status; s != "failed" {
				t.Fatalf("status %s", s)
			}
			if f.deductions(t) != 0 {
				t.Fatal("refund gagal tidak boleh mencatat potongan")
			}
		})
	}
}

func TestFailedRefundCanBeRetried(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 1)
	d := f.refund(t, o)
	tx, _, err := solana.DecodeBase64(d.Transaction)
	if err != nil {
		t.Fatal(err)
	}
	tx.Signature, tx.Failed = testkit.NewSignature(), true
	f.a.Solana.Publish(d.Reference, tx)
	f.a.Tick()

	next := f.refund(t, o)
	if next.ID == d.ID || next.AmountIDR != 100000 {
		t.Fatalf("percobaan baru salah: %+v", next)
	}
	f.send(t, next)
	f.a.Tick()
	if s := f.read(t, next.ID).Status; s != "completed" {
		t.Fatalf("status %s", s)
	}
	if f.deductions(t) != 1 {
		t.Fatalf("potongan %d", f.deductions(t))
	}
}

func TestRefundExpiresAfterGrace(t *testing.T) {
	f := setup(t)
	o := f.paid(t, 1)
	d := f.refund(t, o)
	f.a.Clock.Advance(6 * time.Minute)
	f.a.Tick()
	if s := f.read(t, d.ID).Status; s != "awaiting_signature" {
		t.Fatalf("masih dalam masa tenggang: %s", s)
	}
	f.a.Clock.Advance(time.Minute)
	f.a.Tick()
	if s := f.read(t, d.ID).Status; s != "expired" {
		t.Fatalf("status %s", s)
	}
	if f.deductions(t) != 0 {
		t.Fatal("refund kedaluwarsa tidak boleh mencatat potongan")
	}
	f.refund(t, o)
}

func TestRefundWorkerSurvivesRPCFailure(t *testing.T) {
	f := setup(t)
	d := f.refund(t, f.paid(t, 1))
	f.a.Clock.Advance(time.Hour)
	f.a.Solana.SetErr(errors.New("RPC mati"))
	f.a.Tick()
	f.a.Solana.SetErr(nil)
	if s := f.read(t, d.ID).Status; s != "awaiting_signature" {
		t.Fatalf("status %s", s)
	}
}

func TestParallelWorkersCompleteOnce(t *testing.T) {
	f := setup(t)
	d := f.refund(t, f.paid(t, 1))
	f.send(t, d)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			f.a.Tick()
		}()
	}
	wg.Wait()
	if s := f.read(t, d.ID).Status; s != "completed" {
		t.Fatalf("status %s", s)
	}
	if f.deductions(t) != 1 {
		t.Fatalf("potongan %d", f.deductions(t))
	}
}

func TestZeroShareRefundLeavesNoDeduction(t *testing.T) {
	f := setup(t)
	full := f.a.NewItem(f.sari.ConsignorID, 100000, 10000, 5)
	f.bag = full
	d := f.refund(t, f.paid(t, 1))
	if d.Deduction.AmountIDR != 0 {
		t.Fatalf("potongan %d", d.Deduction.AmountIDR)
	}
	f.send(t, d)
	f.a.Tick()
	if s := f.read(t, d.ID).Status; s != "completed" {
		t.Fatalf("status %s", s)
	}
	if f.deductions(t) != 0 {
		t.Fatal("bagian penitip 0 tidak menghasilkan potongan")
	}
}

func TestListRefunds(t *testing.T) {
	f := setup(t)
	done := f.refund(t, f.paid(t, 1))
	f.send(t, done)
	f.a.Tick()
	f.refund(t, f.paid(t, 1))
	f.refund(t, f.paid(t, 1))

	seen := map[string]bool{}
	path := "/api/v1/refunds?limit=2"
	for {
		var page struct {
			Data       []draftBody `json:"data"`
			NextCursor *string     `json:"next_cursor"`
		}
		f.a.Get(f.owner, path).Want(200).JSON(&page)
		for _, r := range page.Data {
			if seen[r.ID] {
				t.Fatalf("refund %s muncul dua kali", r.ID)
			}
			seen[r.ID] = true
		}
		if page.NextCursor == nil {
			break
		}
		path = "/api/v1/refunds?limit=2&cursor=" + url.QueryEscape(*page.NextCursor)
	}
	if len(seen) != 3 {
		t.Fatalf("refund %d", len(seen))
	}
	var page struct {
		Data []draftBody `json:"data"`
	}
	f.a.Get(f.owner, "/api/v1/refunds?status=completed").Want(200).JSON(&page)
	if len(page.Data) != 1 || page.Data[0].ID != done.ID {
		t.Fatalf("filter status salah: %+v", page.Data)
	}
	future := url.QueryEscape(time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	f.a.Get(f.owner, "/api/v1/refunds?from="+future).Want(200).JSON(&page)
	if len(page.Data) != 0 {
		t.Fatalf("filter from %d", len(page.Data))
	}
	f.a.Get(f.owner, "/api/v1/refunds?status=selesai").WantProblem(400, "VALIDATION_FAILED")
	f.a.Get(f.owner, "/api/v1/refunds/abc").WantProblem(400, "VALIDATION_FAILED")
}
