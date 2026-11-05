package platform_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/cache"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/config"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/middleware"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func validEnv() map[string]string {
	return map[string]string{
		"DATABASE_URL":        "postgres://localhost/db",
		"SUPABASE_JWT_SECRET": "rahasia",
		"SUPABASE_JWT_ISSUER": "https://x.supabase.co/auth/v1",
	}
}

func TestConfigDefaults(t *testing.T) {
	c, err := config.Load(env(validEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if c.OrderTTL != 5*time.Minute || c.OrderGrace != 2*time.Minute {
		t.Fatalf("durasi bawaan salah: %v %v", c.OrderTTL, c.OrderGrace)
	}
	if c.RateLimitAccount != 120 || c.RateLimitIP != 60 || c.RateLimitOrder != 30 {
		t.Fatal("rate limit bawaan tidak sesuai kontrak")
	}
	if c.WalletDomain() != "localhost:3000" {
		t.Fatalf("domain wallet salah: %s", c.WalletDomain())
	}
}

func TestConfigRejectsBadValues(t *testing.T) {
	cases := map[string]func(map[string]string){
		"tanpa database":  func(m map[string]string) { delete(m, "DATABASE_URL") },
		"tanpa kunci jwt": func(m map[string]string) { delete(m, "SUPABASE_JWT_SECRET") },
		"tanpa issuer":    func(m map[string]string) { delete(m, "SUPABASE_JWT_ISSUER") },
		"durasi salah":    func(m map[string]string) { m["ORDER_TTL"] = "lima" },
		"angka salah":     func(m map[string]string) { m["WORKER_BATCH"] = "banyak" },
		"mainnet":         func(m map[string]string) { m["SOLANA_RPC_URL"] = "https://api.mainnet-beta.solana.com" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			m := validEnv()
			mutate(m)
			if _, err := config.Load(env(m)); err == nil {
				t.Fatal("seharusnya ditolak")
			}
		})
	}
}

func TestEveryContractErrorCodeIsImplemented(t *testing.T) {
	c, err := testkit.LoadContract(testkit.ContractPath())
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, code := range apperr.Codes() {
		known[string(code)] = true
	}
	want := c.ErrorCodes()
	for _, code := range want {
		if !known[code] {
			t.Errorf("kode %s ada di kontrak tetapi tidak di apperr", code)
		}
	}
	if len(want) != len(known) {
		t.Errorf("kontrak punya %d kode, apperr punya %d", len(want), len(known))
	}
}

func TestLimiterBlocksAndRefills(t *testing.T) {
	now := time.Unix(1000, 0)
	l := middleware.NewLimiter(60, func() time.Time { return now })
	for i := 0; i < 60; i++ {
		if d := l.Allow("a"); !d.Allowed {
			t.Fatalf("permintaan %d seharusnya lolos", i)
		}
	}
	d := l.Allow("a")
	if d.Allowed || d.Remaining != 0 || d.RetryAfterSeconds < 1 {
		t.Fatalf("seharusnya ditolak: %+v", d)
	}
	if !l.Allow("b").Allowed {
		t.Fatal("kunci lain tidak boleh ikut terbatas")
	}
	now = now.Add(2 * time.Second)
	if !l.Allow("a").Allowed {
		t.Fatal("token seharusnya terisi lagi")
	}
}

func TestLimiterForgetsIdleKeys(t *testing.T) {
	now := time.Unix(1000, 0)
	l := middleware.NewLimiter(60, func() time.Time { return now })
	l.Allow("a")
	now = now.Add(11 * time.Minute)
	l.Allow("b")
	if l.Size() != 1 {
		t.Fatalf("kunci lama seharusnya terhapus, tersisa %d", l.Size())
	}
}

func TestClientIPIgnoresSpoofedHeaders(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:1234"
	r.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
	if got := middleware.ClientIP(r, 0); got != "10.0.0.1" {
		t.Fatalf("tanpa proxy tepercaya, header harus diabaikan: %s", got)
	}
	if got := middleware.ClientIP(r, 1); got != "2.2.2.2" {
		t.Fatalf("satu proxy tepercaya memakai entri terakhir: %s", got)
	}
	if got := middleware.ClientIP(r, 5); got != "10.0.0.1" {
		t.Fatalf("hop lebih banyak dari header memakai alamat koneksi: %s", got)
	}
}

func TestCacheExpiresAndDeletesByPrefix(t *testing.T) {
	now := time.Unix(1000, 0)
	c := cache.NewMemory(func() time.Time { return now })
	c.Set("catalog:a:1", 1, time.Minute)
	c.Set("catalog:b:1", 2, time.Minute)
	if v, ok := c.Get("catalog:a:1"); !ok || v != 1 {
		t.Fatal("nilai seharusnya ada")
	}
	c.DeletePrefix("catalog:a:")
	if _, ok := c.Get("catalog:a:1"); ok {
		t.Fatal("nilai seharusnya terhapus")
	}
	now = now.Add(time.Minute)
	if _, ok := c.Get("catalog:b:1"); ok {
		t.Fatal("nilai seharusnya kedaluwarsa")
	}
}

func TestDecodeRejectsUnknownFieldsAndTrailingData(t *testing.T) {
	type body struct {
		Name string `json:"name"`
	}
	cases := map[string]string{
		"kolom asing": `{"name":"a","x":1}`,
		"dua objek":   `{"name":"a"}{"name":"b"}`,
		"bukan json":  `halo`,
		"kosong":      ``,
		"tipe salah":  `{"name":1}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(raw))
			var b body
			err := api.Decode(httptest.NewRecorder(), r, &b)
			if !apperr.Is(err, apperr.ValidationFailed) {
				t.Fatalf("seharusnya VALIDATION_FAILED: %v", err)
			}
		})
	}
}

func TestDecodeRejectsOversizedBody(t *testing.T) {
	raw := `{"name":"` + strings.Repeat("a", 2<<20) + `"}`
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(raw))
	var b struct {
		Name string `json:"name"`
	}
	if err := api.Decode(httptest.NewRecorder(), r, &b); !apperr.Is(err, apperr.ValidationFailed) {
		t.Fatalf("seharusnya ditolak: %v", err)
	}
}

func TestPageParsing(t *testing.T) {
	for _, q := range []string{"limit=0", "limit=101", "limit=x", "cursor=%25%25"} {
		r := httptest.NewRequest(http.MethodGet, "/?"+q, nil)
		if _, err := api.ParsePage(r); !apperr.Is(err, apperr.ValidationFailed) {
			t.Errorf("%s seharusnya ditolak", q)
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	p, err := api.ParsePage(r)
	if err != nil || p.Limit != 20 || p.Cursor != nil {
		t.Fatalf("bawaan salah: %+v %v", p, err)
	}
}

func TestSolanaAddressValidation(t *testing.T) {
	if !solana.ValidAddress("7xKXtg2CW87d97TXJSDpbD5jBkheTqA83TZRuJosgAsU") {
		t.Fatal("alamat valid ditolak")
	}
	for _, a := range []string{"", "abc", "0OIl0OIl0OIl0OIl0OIl0OIl0OIl0OIl"} {
		if solana.ValidAddress(a) {
			t.Errorf("alamat %q seharusnya ditolak", a)
		}
	}
}
