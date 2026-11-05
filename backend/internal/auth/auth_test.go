package auth_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/config"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

func TestHealthz(t *testing.T) {
	a := testkit.NewApp(t)
	var body struct {
		Status string `json:"status"`
	}
	a.Do(testkit.Request{Method: http.MethodGet, Path: "/healthz"}).Want(200).JSON(&body)
	if body.Status != "ok" {
		t.Fatalf("status %q", body.Status)
	}
}

func TestUnknownRouteIsProblem(t *testing.T) {
	a := testkit.NewApp(t)
	r := a.Do(testkit.Request{Method: http.MethodGet, Path: "/tidak/ada", Raw: true}).WantProblem(404, "NOT_FOUND")
	if r.Header.Get("Content-Type") != "application/problem+json" {
		t.Fatalf("content type %s", r.Header.Get("Content-Type"))
	}
}

func TestCorsOnlyAllowsFrontendForPrivateRoutes(t *testing.T) {
	a := testkit.NewApp(t)
	r := a.Do(testkit.Request{Method: http.MethodOptions, Path: "/api/v1/me", Raw: true, Headers: map[string]string{
		"Origin": "https://app.test", "Access-Control-Request-Method": "GET",
	}})
	if r.Status != 204 || r.Header.Get("Access-Control-Allow-Origin") != "https://app.test" {
		t.Fatalf("frontend seharusnya diizinkan: %d %v", r.Status, r.Header)
	}
	r = a.Do(testkit.Request{Method: http.MethodOptions, Path: "/api/v1/me", Raw: true, Headers: map[string]string{
		"Origin": "https://jahat.test", "Access-Control-Request-Method": "GET",
	}})
	if r.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("origin lain tidak boleh diizinkan")
	}
}

func TestMeRequiresValidToken(t *testing.T) {
	a := testkit.NewApp(t)
	store := a.NewStore()
	u := a.NewUser(store.ID, auth.RoleCashier)
	good := testkit.TokenClaims{Subject: u.Subject, Email: u.Email, EmailVerified: true}

	cases := map[string]string{
		"tanpa token":    "",
		"bukan jwt":      "abc.def.ghi",
		"secret salah":   a.SignToken(with(good, func(c *testkit.TokenClaims) { c.Secret = "secret-penyerang-yang-juga-cukup-panjang-sekali" })),
		"issuer salah":   a.SignToken(with(good, func(c *testkit.TokenClaims) { c.Issuer = "https://palsu.test" })),
		"audience salah": a.SignToken(with(good, func(c *testkit.TokenClaims) { c.Audience = "anon" })),
		"kedaluwarsa":    a.SignToken(with(good, func(c *testkit.TokenClaims) { c.ExpiresIn = -time.Hour })),
		"algoritma lain": a.SignToken(with(good, func(c *testkit.TokenClaims) { c.Method = jwt.SigningMethodHS512 })),
		"algoritma none": noneToken(t, u.Subject),
		"subject kosong": a.SignToken(with(good, func(c *testkit.TokenClaims) { c.Subject = "" })),
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			r := a.Do(testkit.Request{Method: http.MethodGet, Path: "/api/v1/me", Token: token}).WantProblem(401, "UNAUTHENTICATED")
			if r.Header.Get("WWW-Authenticate") == "" {
				t.Fatal("401 wajib memuat WWW-Authenticate")
			}
		})
	}
	a.Do(testkit.Request{Method: http.MethodGet, Path: "/api/v1/me", Token: a.SignToken(good)}).Want(200)
}

func with(c testkit.TokenClaims, f func(*testkit.TokenClaims)) testkit.TokenClaims {
	f(&c)
	return c
}

func noneToken(t *testing.T, subject string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{"sub": subject})
	s, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMeShowsProfileByRole(t *testing.T) {
	a := testkit.NewApp(t)
	store := a.NewStore()
	owner := a.NewUser(store.ID, auth.RoleOwner)
	cashier := a.NewUser(store.ID, auth.RoleCashier)
	consignor := a.NewConsignor(store.ID)

	var me struct {
		Role           string  `json:"role"`
		ConsignorID    *string `json:"consignor_id"`
		WalletRequired bool    `json:"wallet_required"`
		Wallet         *struct {
			Address string `json:"address"`
		} `json:"wallet"`
	}
	a.Get(owner, "/api/v1/me").Want(200).JSON(&me)
	if me.Role != "store_owner" || !me.WalletRequired || me.Wallet == nil || me.Wallet.Address != store.Wallet.Address {
		t.Fatalf("profil pemilik toko salah: %+v", me)
	}
	me.Wallet = nil
	a.Get(cashier, "/api/v1/me").Want(200).JSON(&me)
	if me.WalletRequired || me.Wallet != nil {
		t.Fatalf("kasir tidak butuh wallet: %+v", me)
	}
	a.Get(consignor, "/api/v1/me").Want(200).JSON(&me)
	if me.ConsignorID == nil || *me.ConsignorID != consignor.ConsignorID.String() {
		t.Fatalf("consignor_id salah: %+v", me)
	}
}

func TestUnverifiedConsignorWalletShowsAsNull(t *testing.T) {
	a := testkit.NewApp(t)
	store := a.NewStore()
	c := a.NewConsignor(store.ID)
	a.Exec(`UPDATE consignors SET wallet_verified_at = NULL WHERE id = $1`, c.ConsignorID)
	var me struct {
		Wallet *struct{} `json:"wallet"`
	}
	a.Get(c, "/api/v1/me").Want(200).JSON(&me)
	if me.Wallet != nil {
		t.Fatal("wallet unverified harus tampil null")
	}
}

func TestFirstLoginMatchesVerifiedEmailOnce(t *testing.T) {
	a := testkit.NewApp(t)
	store := a.NewStore()
	id := uuid.New()
	a.Exec(`INSERT INTO users (id, store_id, role, email, name) VALUES ($1, $2, 'cashier', 'kasir@toko.id', 'Kasir')`, id, store.ID)
	me := testkit.Request{Method: http.MethodGet, Path: "/api/v1/me"}

	me.Token = a.SignToken(testkit.TokenClaims{Subject: "google-1", Email: "kasir@toko.id", EmailVerified: false})
	a.Do(me).WantProblem(403, "ROLE_NOT_ASSIGNED")

	me.Token = a.SignToken(testkit.TokenClaims{Subject: "google-1", Email: "KASIR@toko.id", EmailVerified: true})
	a.Do(me).Want(200)

	me.Token = a.SignToken(testkit.TokenClaims{Subject: "google-2", Email: "kasir@toko.id", EmailVerified: true})
	a.Do(me).WantProblem(403, "ROLE_NOT_ASSIGNED")
}

func TestUnknownAccountIsRejected(t *testing.T) {
	a := testkit.NewApp(t)
	token := a.SignToken(testkit.TokenClaims{Subject: "asing", Email: "asing@x.id", EmailVerified: true})
	a.Do(testkit.Request{Method: http.MethodGet, Path: "/api/v1/me", Token: token}).WantProblem(403, "ROLE_NOT_ASSIGNED")
}

func TestRoleChangeAppliesImmediately(t *testing.T) {
	a := testkit.NewApp(t)
	store := a.NewStore()
	u := a.NewUser(store.ID, auth.RoleOwner)
	a.Get(u, "/api/v1/me").Want(200)
	a.Exec(`DELETE FROM users WHERE id = $1`, u.ID)
	a.Get(u, "/api/v1/me").WantProblem(403, "ROLE_NOT_ASSIGNED")
}

func link(t *testing.T, a *testkit.App, u testkit.User, w testkit.Wallet) *testkit.Response {
	t.Helper()
	var n struct {
		Nonce   string `json:"nonce"`
		Message string `json:"message"`
	}
	a.Post(u, "/api/v1/wallet/nonce", map[string]string{"address": w.Address}).Want(201).JSON(&n)
	return a.Post(u, "/api/v1/wallet/link", map[string]string{
		"address": w.Address, "nonce": n.Nonce, "signature": w.Sign(n.Message),
	})
}

func TestOwnerLinksWalletWithSignature(t *testing.T) {
	a := testkit.NewApp(t)
	store := a.NewStoreWithoutWallet()
	owner := a.NewUser(store.ID, auth.RoleOwner)
	w := testkit.NewWallet()
	link(t, a, owner, w).Want(200)
	var got string
	a.Scalar(&got, `SELECT wallet_address FROM stores WHERE id = $1`, store.ID)
	if got != w.Address {
		t.Fatalf("wallet toko %s", got)
	}
	replacement := testkit.NewWallet()
	link(t, a, owner, replacement).Want(200)
	a.Scalar(&got, `SELECT wallet_address FROM stores WHERE id = $1`, store.ID)
	if got != replacement.Address {
		t.Fatal("pemanggilan ulang seharusnya mengganti wallet")
	}
}

func TestConsignorProvesRegisteredWallet(t *testing.T) {
	a := testkit.NewApp(t)
	store := a.NewStore()
	c := a.NewConsignor(store.ID)
	a.Exec(`UPDATE consignors SET wallet_verified_at = NULL WHERE id = $1`, c.ConsignorID)
	link(t, a, c, c.Wallet).Want(200)
	var verified bool
	a.Scalar(&verified, `SELECT wallet_verified_at IS NOT NULL FROM consignors WHERE id = $1`, c.ConsignorID)
	if !verified {
		t.Fatal("wallet seharusnya terbukti")
	}
}

func TestLinkRejectsBadProof(t *testing.T) {
	a := testkit.NewApp(t)
	store := a.NewStoreWithoutWallet()
	owner := a.NewUser(store.ID, auth.RoleOwner)
	other := a.NewUser(store.ID, auth.RoleOwner)
	w := testkit.NewWallet()
	nonce := func() (string, string) {
		var n struct {
			Nonce   string `json:"nonce"`
			Message string `json:"message"`
		}
		a.Post(owner, "/api/v1/wallet/nonce", map[string]string{"address": w.Address}).Want(201).JSON(&n)
		return n.Nonce, n.Message
	}
	linkWith := func(u testkit.User, address, n, sig string) *testkit.Response {
		return a.Post(u, "/api/v1/wallet/link", map[string]string{"address": address, "nonce": n, "signature": sig})
	}

	t.Run("tanda tangan wallet lain", func(t *testing.T) {
		n, msg := nonce()
		linkWith(owner, w.Address, n, testkit.NewWallet().Sign(msg)).WantProblem(422, "SIGNATURE_INVALID")
	})
	t.Run("pesan lain", func(t *testing.T) {
		n, _ := nonce()
		linkWith(owner, w.Address, n, w.Sign("pesan lain")).WantProblem(422, "SIGNATURE_INVALID")
	})
	t.Run("nonce tidak dikenal", func(t *testing.T) {
		_, msg := nonce()
		linkWith(owner, w.Address, "abcdef", w.Sign(msg)).WantProblem(422, "NONCE_INVALID")
	})
	t.Run("nonce milik akun lain", func(t *testing.T) {
		n, msg := nonce()
		linkWith(other, w.Address, n, w.Sign(msg)).WantProblem(422, "NONCE_INVALID")
	})
	t.Run("alamat berbeda dengan nonce", func(t *testing.T) {
		n, msg := nonce()
		x := testkit.NewWallet()
		linkWith(owner, x.Address, n, x.Sign(msg)).WantProblem(422, "NONCE_INVALID")
	})
	t.Run("nonce kedaluwarsa", func(t *testing.T) {
		n, msg := nonce()
		a.Clock.Advance(6 * time.Minute)
		linkWith(owner, w.Address, n, w.Sign(msg)).WantProblem(422, "NONCE_INVALID")
	})
	t.Run("nonce dipakai dua kali", func(t *testing.T) {
		n, msg := nonce()
		linkWith(owner, w.Address, n, w.Sign(msg)).Want(200)
		linkWith(owner, w.Address, n, w.Sign(msg)).WantProblem(422, "NONCE_INVALID")
	})
	t.Run("format salah", func(t *testing.T) {
		a.Post(owner, "/api/v1/wallet/nonce", map[string]string{"address": "bukan-alamat"}).WantProblem(400, "VALIDATION_FAILED")
		a.Post(owner, "/api/v1/wallet/nonce", map[string]any{"address": w.Address, "x": 1}).WantProblem(400, "VALIDATION_FAILED")
		linkWith(owner, w.Address, "", "x").WantProblem(400, "VALIDATION_FAILED")
	})
}

func TestWalletCannotBeSharedAcrossOwners(t *testing.T) {
	a := testkit.NewApp(t)
	storeA, storeB := a.NewStore(), a.NewStoreWithoutWallet()
	ownerB := a.NewUser(storeB.ID, auth.RoleOwner)
	link(t, a, ownerB, storeA.Wallet).WantProblem(409, "WALLET_ALREADY_LINKED")

	c := a.NewConsignor(storeA.ID)
	link(t, a, ownerB, c.Wallet).WantProblem(409, "WALLET_ALREADY_LINKED")
}

func TestCashierCannotLinkWallet(t *testing.T) {
	a := testkit.NewApp(t)
	store := a.NewStore()
	cashier := a.NewUser(store.ID, auth.RoleCashier)
	a.Post(cashier, "/api/v1/wallet/nonce", map[string]string{"address": testkit.NewWallet().Address}).WantProblem(403, "FORBIDDEN")
}

func TestAccountRateLimitReturnsHeaders(t *testing.T) {
	a := testkit.NewApp(t, func(c *config.Config) { c.RateLimitAccount = 3 })
	store := a.NewStore()
	u := a.NewUser(store.ID, auth.RoleCashier)
	for i := 0; i < 3; i++ {
		r := a.Get(u, "/api/v1/me").Want(200)
		if want := fmt.Sprint(2 - i); r.Header.Get("RateLimit-Remaining") != want {
			t.Fatalf("sisa %s, seharusnya %s", r.Header.Get("RateLimit-Remaining"), want)
		}
	}
	r := a.Get(u, "/api/v1/me").WantProblem(429, "RATE_LIMITED")
	if r.Header.Get("Retry-After") == "" {
		t.Fatal("429 wajib memuat Retry-After")
	}
	other := a.NewUser(store.ID, auth.RoleCashier)
	a.Get(other, "/api/v1/me").Want(200)
}

func TestInvalidTokensAreLimitedPerIP(t *testing.T) {
	a := testkit.NewApp(t, func(c *config.Config) { c.RateLimitIP = 2 })
	req := testkit.Request{Method: http.MethodGet, Path: "/api/v1/me", Token: "palsu"}
	a.Do(req).Want(401)
	a.Do(req).Want(401)
	a.Do(req).WantProblem(429, "RATE_LIMITED")
}

func TestRequestIDIsSafe(t *testing.T) {
	a := testkit.NewApp(t)
	r := a.Do(testkit.Request{Method: http.MethodGet, Path: "/healthz", Headers: map[string]string{"X-Request-Id": "abc\"def<script>"}})
	if got := r.Header.Get("X-Request-Id"); got == "abc\"def<script>" || got == "" {
		t.Fatalf("request ID tidak disaring: %q", got)
	}
	r = a.Do(testkit.Request{Method: http.MethodGet, Path: "/healthz", Headers: map[string]string{"X-Request-Id": "trace-12345"}})
	if r.Header.Get("X-Request-Id") != "trace-12345" {
		t.Fatal("request ID yang aman seharusnya dipakai")
	}
}
