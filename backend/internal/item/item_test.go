package item_test

import (
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

type itemBody struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	PriceIDR           int64  `json:"price_idr"`
	StockOnHand        int    `json:"stock_on_hand"`
	StockReserved      int    `json:"stock_reserved"`
	StockAvailable     int    `json:"stock_available"`
	Active             bool   `json:"active"`
	CertificateEnabled bool   `json:"certificate_enabled"`
}

type listBody struct {
	Data       []itemBody `json:"data"`
	NextCursor *string    `json:"next_cursor"`
}

type fixture struct {
	a         *testkit.App
	store     testkit.Store
	owner     testkit.User
	cashier   testkit.User
	consignor testkit.User
}

func setup(t *testing.T) fixture {
	t.Helper()
	a := testkit.NewApp(t)
	store := a.NewStore()
	return fixture{
		a: a, store: store,
		owner:     a.NewUser(store.ID, auth.RoleOwner),
		cashier:   a.NewUser(store.ID, auth.RoleCashier),
		consignor: a.NewConsignor(store.ID),
	}
}

func (f fixture) create(t *testing.T, name string) itemBody {
	t.Helper()
	var out itemBody
	f.a.Post(f.owner, "/api/v1/items", map[string]any{
		"name": name, "price_idr": 50000, "commission_bps": 1500,
		"consignor_id": f.consignor.ConsignorID, "stock_on_hand": 10,
	}).Want(201).JSON(&out)
	return out
}

func TestCreateAndGetItem(t *testing.T) {
	f := setup(t)
	r := f.a.Post(f.owner, "/api/v1/items", map[string]any{
		"name": "Selai nanas", "price_idr": 50000, "commission_bps": 1500,
		"consignor_id": f.consignor.ConsignorID, "stock_on_hand": 10, "certificate_enabled": true,
	}).Want(201)
	var created itemBody
	r.JSON(&created)
	if r.Header.Get("Location") != "/api/v1/items/"+created.ID {
		t.Fatalf("Location salah: %s", r.Header.Get("Location"))
	}
	if !created.Active || !created.CertificateEnabled || created.StockAvailable != 10 {
		t.Fatalf("data salah: %+v", created)
	}
	var got itemBody
	f.a.Get(f.cashier, "/api/v1/items/"+created.ID).Want(200).JSON(&got)
	if got.Name != "Selai nanas" {
		t.Fatalf("nama %s", got.Name)
	}
}

func TestCreateRejectsBadInput(t *testing.T) {
	f := setup(t)
	good := func() map[string]any {
		return map[string]any{
			"name": "A", "price_idr": 1, "commission_bps": 0,
			"consignor_id": f.consignor.ConsignorID, "stock_on_hand": 0,
		}
	}
	cases := map[string]func(map[string]any){
		"tanpa nama":       func(m map[string]any) { delete(m, "name") },
		"nama kosong":      func(m map[string]any) { m["name"] = "" },
		"harga 0":          func(m map[string]any) { m["price_idr"] = 0 },
		"harga kelewat":    func(m map[string]any) { m["price_idr"] = 10_000_000_001 },
		"komisi negatif":   func(m map[string]any) { m["commission_bps"] = -1 },
		"komisi kelewat":   func(m map[string]any) { m["commission_bps"] = 10001 },
		"stok negatif":     func(m map[string]any) { m["stock_on_hand"] = -1 },
		"tanpa penitip":    func(m map[string]any) { delete(m, "consignor_id") },
		"penitip bukan id": func(m map[string]any) { m["consignor_id"] = "abc" },
		"kolom asing":      func(m map[string]any) { m["stock_reserved"] = 1 },
		"harga pecahan":    func(m map[string]any) { m["price_idr"] = 1.5 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := good()
			mutate(m)
			f.a.Post(f.owner, "/api/v1/items", m).WantProblem(400, "VALIDATION_FAILED")
		})
	}
	f.a.Post(f.owner, "/api/v1/items", good()).Want(201)
}

func TestCreateRejectsConsignorFromOtherStore(t *testing.T) {
	f := setup(t)
	other := f.a.NewStore()
	stranger := f.a.NewConsignor(other.ID)
	f.a.Post(f.owner, "/api/v1/items", map[string]any{
		"name": "A", "price_idr": 1, "commission_bps": 0,
		"consignor_id": stranger.ConsignorID, "stock_on_hand": 0,
	}).WantProblem(404, "NOT_FOUND")
}

func TestListPaginatesWithoutGapsOrDuplicates(t *testing.T) {
	f := setup(t)
	for i := 0; i < 5; i++ {
		f.create(t, fmt.Sprintf("Barang %d", i))
	}
	seen := map[string]bool{}
	path := "/api/v1/items?limit=2"
	pages := 0
	for {
		var page listBody
		f.a.Get(f.cashier, path).Want(200).JSON(&page)
		pages++
		for _, i := range page.Data {
			if seen[i.ID] {
				t.Fatalf("barang %s muncul dua kali", i.ID)
			}
			seen[i.ID] = true
		}
		if page.NextCursor == nil {
			break
		}
		path = "/api/v1/items?limit=2&cursor=" + url.QueryEscape(*page.NextCursor)
	}
	if len(seen) != 5 || pages != 3 {
		t.Fatalf("barang %d, halaman %d", len(seen), pages)
	}
}

func TestListFilters(t *testing.T) {
	f := setup(t)
	f.create(t, "Tas 100% rajut")
	hidden := f.create(t, "Topi")
	second := f.a.NewConsignor(f.store.ID)
	f.a.NewItem(second.ConsignorID, 1000, 0, 1)
	f.a.Patch(f.owner, "/api/v1/items/"+hidden.ID, map[string]any{"active": false}).Want(200)

	count := func(q string) int {
		var page listBody
		f.a.Get(f.cashier, "/api/v1/items?"+q).Want(200).JSON(&page)
		return len(page.Data)
	}
	if n := count(""); n != 2 {
		t.Fatalf("bawaan hanya barang aktif, dapat %d", n)
	}
	if n := count("active=false"); n != 1 {
		t.Fatalf("barang nonaktif %d", n)
	}
	if n := count("q=rajut"); n != 1 {
		t.Fatalf("pencarian %d", n)
	}
	if n := count("q=" + url.QueryEscape("100%")); n != 1 {
		t.Fatalf("persen harus dicari apa adanya, dapat %d", n)
	}
	if n := count("q=" + url.QueryEscape("%")); n != 1 {
		t.Fatalf("persen bukan wildcard, dapat %d", n)
	}
	if n := count("consignor_id=" + second.ConsignorID.String()); n != 1 {
		t.Fatalf("filter penitip %d", n)
	}
	f.a.Get(f.cashier, "/api/v1/items?consignor_id=abc").WantProblem(400, "VALIDATION_FAILED")
	f.a.Get(f.cashier, "/api/v1/items?active=mungkin").WantProblem(400, "VALIDATION_FAILED")
	f.a.Get(f.cashier, "/api/v1/items?limit=0").WantProblem(400, "VALIDATION_FAILED")
}

func TestStoresAreIsolated(t *testing.T) {
	f := setup(t)
	mine := f.create(t, "Punya saya")
	other := f.a.NewStore()
	stranger := f.a.NewUser(other.ID, auth.RoleOwner)

	var page listBody
	f.a.Get(stranger, "/api/v1/items").Want(200).JSON(&page)
	if len(page.Data) != 0 {
		t.Fatal("toko lain tidak boleh melihat barang ini")
	}
	f.a.Get(stranger, "/api/v1/items/"+mine.ID).WantProblem(404, "NOT_FOUND")
	f.a.Patch(stranger, "/api/v1/items/"+mine.ID, map[string]any{"price_idr": 1}).WantProblem(404, "NOT_FOUND")
}

func TestRolesForItems(t *testing.T) {
	f := setup(t)
	it := f.create(t, "Barang")
	f.a.Post(f.cashier, "/api/v1/items", map[string]any{}).WantProblem(403, "FORBIDDEN")
	f.a.Patch(f.cashier, "/api/v1/items/"+it.ID, map[string]any{"price_idr": 1}).WantProblem(403, "FORBIDDEN")
	f.a.Get(f.consignor, "/api/v1/items").WantProblem(403, "FORBIDDEN")
}

func TestOwnerWithoutWalletIsGated(t *testing.T) {
	a := testkit.NewApp(t)
	store := a.NewStoreWithoutWallet()
	owner := a.NewUser(store.ID, auth.RoleOwner)
	a.Get(owner, "/api/v1/items").WantProblem(403, "WALLET_NOT_LINKED")
	a.Get(owner, "/api/v1/me").Want(200)
}

func TestUpdateItem(t *testing.T) {
	f := setup(t)
	it := f.create(t, "Lama")
	var got itemBody
	f.a.Patch(f.owner, "/api/v1/items/"+it.ID, map[string]any{"name": "Baru", "price_idr": 75000}).Want(200).JSON(&got)
	if got.Name != "Baru" || got.PriceIDR != 75000 || got.StockOnHand != 10 {
		t.Fatalf("hasil salah: %+v", got)
	}
	f.a.Patch(f.owner, "/api/v1/items/"+it.ID, map[string]any{}).WantProblem(400, "VALIDATION_FAILED")
	f.a.Patch(f.owner, "/api/v1/items/"+it.ID, map[string]any{"stock_reserved": 1}).WantProblem(400, "VALIDATION_FAILED")
	f.a.Patch(f.owner, "/api/v1/items/bukan-id", map[string]any{"name": "x"}).WantProblem(400, "VALIDATION_FAILED")
	f.a.Patch(f.owner, "/api/v1/items/"+uuid.NewString(), map[string]any{"name": "x"}).WantProblem(404, "NOT_FOUND")
}

func reserve(t *testing.T, f fixture, itemID string, qty int) {
	t.Helper()
	order := uuid.New()
	f.a.Exec(`INSERT INTO orders (id, created_by, reference, idr_per_sol, rate_quoted_at, idempotency_key, request_hash, expires_at)
		VALUES ($1, $2, $3, 2500000, now(), $4, 'h', now())`, order, f.cashier.ID, uuid.NewString(), uuid.NewString())
	f.a.Exec(`INSERT INTO order_lines (id, order_id, item_id, quantity, unit_price_idr, commission_bps)
		VALUES ($1, $2, $3, $4, 1000, 0)`, uuid.New(), order, itemID, qty)
}

func TestStockCountsPendingOrdersOnly(t *testing.T) {
	f := setup(t)
	it := f.create(t, "Barang")
	reserve(t, f, it.ID, 3)
	var got itemBody
	f.a.Get(f.cashier, "/api/v1/items/"+it.ID).Want(200).JSON(&got)
	if got.StockReserved != 3 || got.StockAvailable != 7 || got.StockOnHand != 10 {
		t.Fatalf("stok salah: %+v", got)
	}
	f.a.Exec(`UPDATE orders SET status = 'expired'`)
	f.a.Get(f.cashier, "/api/v1/items/"+it.ID).Want(200).JSON(&got)
	if got.StockReserved != 0 || got.StockAvailable != 10 {
		t.Fatalf("status order saja harus melepas reservasi: %+v", got)
	}
}

func TestStockCannotDropBelowReserved(t *testing.T) {
	f := setup(t)
	it := f.create(t, "Barang")
	reserve(t, f, it.ID, 4)
	f.a.Patch(f.owner, "/api/v1/items/"+it.ID, map[string]any{"stock_on_hand": 3}).WantProblem(409, "STOCK_BELOW_RESERVED")
	f.a.Patch(f.owner, "/api/v1/items/"+it.ID, map[string]any{"stock_on_hand": 4}).Want(200)
}

func TestCatalogCacheIsInvalidatedOnWrite(t *testing.T) {
	f := setup(t)
	it := f.create(t, "Lama")
	read := func() string {
		var page listBody
		f.a.Get(f.cashier, "/api/v1/items").Want(200).JSON(&page)
		return page.Data[0].Name
	}
	if read() != "Lama" {
		t.Fatal("katalog awal salah")
	}
	f.a.Exec(`UPDATE items SET name = 'Diubah di luar API' WHERE id = $1`, it.ID)
	if read() != "Lama" {
		t.Fatal("katalog seharusnya terbaca dari cache")
	}
	f.a.Clock.Advance(6 * time.Second)
	if read() != "Diubah di luar API" {
		t.Fatal("cache seharusnya kedaluwarsa")
	}
	f.a.Patch(f.owner, "/api/v1/items/"+it.ID, map[string]any{"name": "Baru"}).Want(200)
	if read() != "Baru" {
		t.Fatal("perubahan lewat API harus menghapus cache")
	}
}
