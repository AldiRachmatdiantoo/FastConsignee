package seed_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/seed"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

const owner = "pemilik@test.id"

func count(t *testing.T, a *testkit.App, sql string, args ...any) int {
	t.Helper()
	var n int
	a.Scalar(&n, sql, args...)
	return n
}

func TestSeedLoadsDemoData(t *testing.T) {
	a := testkit.NewApp(t)
	out, err := seed.Run(context.Background(), a.DB, seed.Options{OwnerEmail: " Pemilik@Test.id "})
	if err != nil {
		t.Fatal(err)
	}
	if out.Users != 5 || out.Consignors != 3 || out.Items != 9 {
		t.Fatalf("hasil seed salah: %+v", out)
	}
	if n := count(t, a, `SELECT count(*) FROM users WHERE store_id = $1 AND email = $2 AND role = 'store_owner'`, out.StoreID, owner); n != 1 {
		t.Fatalf("pemilik %d", n)
	}
	rows, err := a.DB.Pool.Query(context.Background(), `SELECT wallet_address, wallet_verified_at IS NULL FROM consignors WHERE store_id = $1`, out.StoreID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var (
			wallet     string
			unverified bool
		)
		if err := rows.Scan(&wallet, &unverified); err != nil {
			t.Fatal(err)
		}
		if !solana.ValidAddress(wallet) || !unverified || seen[wallet] {
			t.Fatalf("wallet salah: %q %v", wallet, unverified)
		}
		seen[wallet] = true
	}
	if len(seen) != 3 {
		t.Fatalf("wallet %d", len(seen))
	}
}

func TestSeedIsIdempotent(t *testing.T) {
	a := testkit.NewApp(t)
	first, err := seed.Run(context.Background(), a.DB, seed.Options{OwnerEmail: owner})
	if err != nil {
		t.Fatal(err)
	}
	var wallet string
	a.Scalar(&wallet, `SELECT min(wallet_address) FROM consignors`)

	second, err := seed.Run(context.Background(), a.DB, seed.Options{OwnerEmail: owner})
	if err != nil {
		t.Fatal(err)
	}
	if second != (seed.Result{StoreID: first.StoreID}) {
		t.Fatalf("seed kedua membuat data baru: %+v", second)
	}
	var after string
	a.Scalar(&after, `SELECT min(wallet_address) FROM consignors`)
	if wallet != after {
		t.Fatal("seed kedua mengganti wallet")
	}
	if n := count(t, a, `SELECT count(*) FROM items`); n != 9 {
		t.Fatalf("barang %d", n)
	}
}

func TestSeedUsesChosenAccounts(t *testing.T) {
	a := testkit.NewApp(t)
	wallets := []string{testkit.NewWallet().Address, testkit.NewWallet().Address, testkit.NewWallet().Address}
	emails := []string{"A@test.id", "b@test.id", "c@test.id"}
	_, err := seed.Run(context.Background(), a.DB, seed.Options{
		OwnerEmail: owner, CashierEmail: "Kasir@test.id", ConsignorEmails: emails, ConsignorWallets: wallets,
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := count(t, a, `SELECT count(*) FROM users WHERE email = 'kasir@test.id' AND role = 'cashier'`); n != 1 {
		t.Fatalf("kasir %d", n)
	}
	for i, w := range wallets {
		n := count(t, a, `SELECT count(*) FROM consignors c JOIN users u ON u.id = c.user_id WHERE u.email = $1 AND c.wallet_address = $2`,
			strings.ToLower(emails[i]), w)
		if n != 1 {
			t.Fatalf("penitip %d tidak cocok", i)
		}
	}
}

func TestSeedRejectsBadOptions(t *testing.T) {
	a := testkit.NewApp(t)
	for name, o := range map[string]seed.Options{
		"tanpa pemilik": {},
		"jumlah email":  {OwnerEmail: owner, ConsignorEmails: []string{"a@test.id"}},
		"jumlah wallet": {OwnerEmail: owner, ConsignorWallets: []string{testkit.NewWallet().Address}},
		"wallet rusak":  {OwnerEmail: owner, ConsignorWallets: []string{"0", "0", "0"}},
	} {
		if _, err := seed.Run(context.Background(), a.DB, o); err == nil {
			t.Errorf("%s seharusnya ditolak", name)
		}
	}
	if n := count(t, a, `SELECT count(*) FROM stores`); n != 0 {
		t.Fatalf("toko %d", n)
	}
}

func TestSeedRollsBackOnConflict(t *testing.T) {
	a := testkit.NewApp(t)
	other := a.NewStore()
	stranger := a.NewUser(other.ID, "cashier")
	_, err := seed.Run(context.Background(), a.DB, seed.Options{OwnerEmail: owner, CashierEmail: stranger.Email})
	if err == nil {
		t.Fatal("email toko lain seharusnya ditolak")
	}
	if n := count(t, a, `SELECT count(*) FROM stores`); n != 1 {
		t.Fatalf("seed gagal harus dibatalkan seluruhnya: %d toko", n)
	}
}

// Data seed harus bisa langsung dipakai: pemilik login Google, menautkan wallet, lalu kasir menjual.
func TestSeededStoreCanSell(t *testing.T) {
	a := testkit.NewApp(t)
	out, err := seed.Run(context.Background(), a.DB, seed.Options{OwnerEmail: owner})
	if err != nil {
		t.Fatal(err)
	}
	subject := uuid.NewString()
	token := a.SignToken(testkit.TokenClaims{Subject: subject, Email: owner, EmailVerified: true})
	a.Do(testkit.Request{Method: "GET", Path: "/api/v1/me", Token: token}).Want(200)

	a.Exec(`UPDATE stores SET wallet_address = $2, wallet_linked_at = now() WHERE id = $1`, out.StoreID, testkit.NewWallet().Address)
	var page struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	a.Do(testkit.Request{Method: "GET", Path: "/api/v1/items?limit=100", Token: token}).Want(200).JSON(&page)
	if len(page.Data) != 9 {
		t.Fatalf("katalog %d", len(page.Data))
	}
	lines := make([]map[string]any, 0, len(page.Data))
	for _, it := range page.Data {
		lines = append(lines, map[string]any{"item_id": it.ID, "quantity": 1})
	}
	cashier := a.SignToken(testkit.TokenClaims{Subject: uuid.NewString(), Email: "kasir@fastconsignee.test", EmailVerified: true})
	var order struct {
		TotalIDR   int64 `json:"total_idr"`
		Recipients []struct {
			Wallet string `json:"wallet_address"`
		} `json:"recipients"`
	}
	a.Do(testkit.Request{
		Method: "POST", Path: "/api/v1/orders", Token: cashier,
		Headers: map[string]string{"Idempotency-Key": "kunci-seed-pertama"},
		Body:    map[string]any{"lines": lines},
	}).Want(201).JSON(&order)
	if order.TotalIDR != 1653000 || len(order.Recipients) != 4 {
		t.Fatalf("order salah: %+v", order)
	}
}
