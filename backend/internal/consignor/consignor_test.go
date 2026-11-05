package consignor_test

import (
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

type body struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	WalletAddress *string `json:"wallet_address"`
	WalletStatus  string  `json:"wallet_status"`
}

func setup(t *testing.T) (*testkit.App, testkit.Store, testkit.User) {
	t.Helper()
	a := testkit.NewApp(t)
	store := a.NewStore()
	return a, store, a.NewUser(store.ID, auth.RoleOwner)
}

func TestRegisterConsignorWithWallet(t *testing.T) {
	a, _, owner := setup(t)
	w := testkit.NewWallet()
	r := a.Post(owner, "/api/v1/consignors", map[string]any{
		"name": "Rajut Bu Sari", "email": "Sari@Rajut.ID", "wallet_address": w.Address,
	}).Want(201)
	var c body
	r.JSON(&c)
	if r.Header.Get("Location") != "/api/v1/consignors/"+c.ID {
		t.Fatalf("Location salah: %s", r.Header.Get("Location"))
	}
	if c.WalletStatus != "unverified" || c.Email != "sari@rajut.id" {
		t.Fatalf("data salah: %+v", c)
	}
}

func TestRegisterConsignorWithoutWallet(t *testing.T) {
	a, _, owner := setup(t)
	var c body
	a.Post(owner, "/api/v1/consignors", map[string]any{"name": "A", "email": "a@b.id"}).Want(201).JSON(&c)
	if c.WalletStatus != "unlinked" || c.WalletAddress != nil {
		t.Fatalf("data salah: %+v", c)
	}
}

func TestConsignorFirstLoginMatchesEmail(t *testing.T) {
	a, _, owner := setup(t)
	var c body
	a.Post(owner, "/api/v1/consignors", map[string]any{"name": "A", "email": "a@b.id"}).Want(201).JSON(&c)
	token := a.SignToken(testkit.TokenClaims{Subject: "google-a", Email: "a@b.id", EmailVerified: true})
	var me struct {
		Role        string `json:"role"`
		ConsignorID string `json:"consignor_id"`
	}
	a.Do(testkit.Request{Method: "GET", Path: "/api/v1/me", Token: token}).Want(200).JSON(&me)
	if me.Role != "consignor" || me.ConsignorID != c.ID {
		t.Fatalf("profil salah: %+v", me)
	}
}

func TestRegisterRejectsBadInput(t *testing.T) {
	a, _, owner := setup(t)
	cases := map[string]map[string]any{
		"tanpa nama":    {"email": "a@b.id"},
		"nama panjang":  {"name": strings.Repeat("a", 201), "email": "a@b.id"},
		"email salah":   {"name": "A", "email": "bukan email"},
		"wallet salah":  {"name": "A", "email": "a@b.id", "wallet_address": "abc"},
		"wallet base58": {"name": "A", "email": "a@b.id", "wallet_address": strings.Repeat("0", 32)},
		"kolom asing":   {"name": "A", "email": "a@b.id", "role": "store_owner"},
	}
	for name, b := range cases {
		t.Run(name, func(t *testing.T) {
			a.Post(owner, "/api/v1/consignors", b).WantProblem(400, "VALIDATION_FAILED")
		})
	}
}

func TestRegisterRejectsDuplicates(t *testing.T) {
	a, store, owner := setup(t)
	w := testkit.NewWallet()
	a.Post(owner, "/api/v1/consignors", map[string]any{"name": "A", "email": "a@b.id", "wallet_address": w.Address}).Want(201)
	a.Post(owner, "/api/v1/consignors", map[string]any{"name": "B", "email": "A@B.id"}).WantProblem(409, "CONSIGNOR_EMAIL_TAKEN")
	a.Post(owner, "/api/v1/consignors", map[string]any{"name": "B", "email": "b@b.id", "wallet_address": w.Address}).WantProblem(409, "WALLET_ALREADY_LINKED")
	a.Post(owner, "/api/v1/consignors", map[string]any{"name": "C", "email": "c@b.id", "wallet_address": store.Wallet.Address}).WantProblem(409, "WALLET_ALREADY_LINKED")

	var n int
	a.Scalar(&n, `SELECT count(*) FROM users WHERE email IN ('b@b.id', 'c@b.id')`)
	if n != 0 {
		t.Fatal("akun tidak boleh tersisa setelah pendaftaran gagal")
	}
}

func TestListGetAndRename(t *testing.T) {
	a, _, owner := setup(t)
	var ids []string
	for _, e := range []string{"a@b.id", "b@b.id", "c@b.id"} {
		var c body
		a.Post(owner, "/api/v1/consignors", map[string]any{"name": "Nama", "email": e}).Want(201).JSON(&c)
		ids = append(ids, c.ID)
	}
	seen := 0
	path := "/api/v1/consignors?limit=2"
	for {
		var page struct {
			Data       []body  `json:"data"`
			NextCursor *string `json:"next_cursor"`
		}
		a.Get(owner, path).Want(200).JSON(&page)
		seen += len(page.Data)
		if page.NextCursor == nil {
			break
		}
		path = "/api/v1/consignors?limit=2&cursor=" + url.QueryEscape(*page.NextCursor)
	}
	if seen != 3 {
		t.Fatalf("penitip %d", seen)
	}

	var c body
	a.Patch(owner, "/api/v1/consignors/"+ids[0], map[string]any{"name": "Nama baru"}).Want(200).JSON(&c)
	if c.Name != "Nama baru" {
		t.Fatalf("nama %s", c.Name)
	}
	a.Get(owner, "/api/v1/consignors/"+ids[0]).Want(200).JSON(&c)
	if c.Name != "Nama baru" {
		t.Fatalf("nama %s", c.Name)
	}
	a.Patch(owner, "/api/v1/consignors/"+ids[0], map[string]any{"wallet_address": testkit.NewWallet().Address}).WantProblem(400, "VALIDATION_FAILED")
	a.Get(owner, "/api/v1/consignors/abc").WantProblem(400, "VALIDATION_FAILED")
	a.Get(owner, "/api/v1/consignors/"+uuid.NewString()).WantProblem(404, "NOT_FOUND")
	a.Patch(owner, "/api/v1/consignors/"+uuid.NewString(), map[string]any{"name": "x"}).WantProblem(404, "NOT_FOUND")
}

func TestStoresAndRolesAreIsolated(t *testing.T) {
	a, store, owner := setup(t)
	c := a.NewConsignor(store.ID)
	other := a.NewStore()
	stranger := a.NewUser(other.ID, auth.RoleOwner)
	path := "/api/v1/consignors/" + c.ConsignorID.String()

	a.Get(owner, path).Want(200)
	a.Get(stranger, path).WantProblem(404, "NOT_FOUND")
	a.Patch(stranger, path, map[string]any{"name": "x"}).WantProblem(404, "NOT_FOUND")

	cashier := a.NewUser(store.ID, auth.RoleCashier)
	a.Get(cashier, "/api/v1/consignors").WantProblem(403, "FORBIDDEN")
	a.Get(c, "/api/v1/consignors").WantProblem(403, "FORBIDDEN")
}
