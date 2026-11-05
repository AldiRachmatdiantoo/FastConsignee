package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/app"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/cache"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/config"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
)

const (
	jwtSecret = "rahasia-khusus-test-yang-cukup-panjang-untuk-hs256"
	jwtIssuer = "https://test.supabase.co/auth/v1"
	jwtAud    = "authenticated"
)

// Clock adalah jam yang bisa dimajukan test.
type Clock struct {
	mu  sync.Mutex
	now time.Time
}

// Now membaca waktu sekarang.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance memajukan jam.
func (c *Clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// App adalah API lengkap di atas Postgres sungguhan dan RPC tiruan.
type App struct {
	T        testing.TB
	DB       *database.DB
	API      *app.App
	Solana   *FakeSolana
	Clock    *Clock
	Config   config.Config
	Contract *Contract
	Cache    *cache.Memory
}

var (
	contractOnce sync.Once
	contract     *Contract
	contractErr  error
)

// LoadedContract membaca kontrak sekali per proses test.
func LoadedContract(t testing.TB) *Contract {
	t.Helper()
	contractOnce.Do(func() { contract, contractErr = LoadContract(ContractPath()) })
	if contractErr != nil {
		t.Fatalf("baca kontrak: %v", contractErr)
	}
	return contract
}

// Option mengubah konfigurasi App sebelum dibuat.
type Option func(*config.Config)

// NewApp membuat API untuk satu test.
func NewApp(t testing.TB, opts ...Option) *App {
	t.Helper()
	db := NewDB(t)
	clock := &Clock{now: time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC)}
	cfg := config.Config{
		PublicAPIURL:      "https://api.test",
		FrontendURL:       "https://app.test",
		IconURL:           "https://app.test/icon.png",
		JWTIssuer:         jwtIssuer,
		JWTAudience:       jwtAud,
		OrderTTL:          5 * time.Minute,
		OrderGrace:        2 * time.Minute,
		RefundTTL:         5 * time.Minute,
		NonceTTL:          5 * time.Minute,
		WorkerBatch:       50,
		RateLimitAccount:  100000,
		RateLimitIP:       100000,
		RateLimitOrder:    100000,
		CatalogCacheTTL:   5 * time.Second,
		DashboardCacheTTL: 30 * time.Second,
		RateTTL:           45 * time.Second,
		FixedIDRPerSOL:    2500000,
	}
	for _, o := range opts {
		o(&cfg)
	}
	fake := NewFakeSolana()
	memory := cache.NewMemory(clock.Now)
	a := app.New(app.Deps{
		Config:   cfg,
		DB:       db,
		Solana:   fake,
		Cache:    memory,
		Verifier: auth.NewHMACVerifier(jwtSecret, jwtIssuer, jwtAud, clock.Now),
		Now:      clock.Now,
	})
	return &App{T: t, DB: db, API: a, Solana: fake, Clock: clock, Config: cfg, Contract: LoadedContract(t), Cache: memory}
}

// Exec menjalankan SQL fixture.
func (a *App) Exec(sql string, args ...any) {
	a.T.Helper()
	if _, err := a.DB.Pool.Exec(context.Background(), sql, args...); err != nil {
		a.T.Fatalf("fixture: %v\n%s", err, sql)
	}
}

// Scalar membaca satu nilai.
func (a *App) Scalar(dst any, sql string, args ...any) {
	a.T.Helper()
	if err := a.DB.Pool.QueryRow(context.Background(), sql, args...).Scan(dst); err != nil {
		a.T.Fatalf("query: %v\n%s", err, sql)
	}
}

// User adalah pengguna fixture.
type User struct {
	ID          uuid.UUID
	StoreID     uuid.UUID
	Subject     string
	Email       string
	ConsignorID uuid.UUID
	Wallet      Wallet
}

// Store adalah toko fixture.
type Store struct {
	ID     uuid.UUID
	Wallet Wallet
}

// NewStore membuat toko. Wallet-nya langsung tertaut.
func (a *App) NewStore() Store {
	a.T.Helper()
	s := Store{ID: uuid.New(), Wallet: NewWallet()}
	a.Exec(`INSERT INTO stores (id, name, wallet_address, wallet_linked_at) VALUES ($1, 'Toko Titip Bersama', $2, now())`, s.ID, s.Wallet.Address)
	return s
}

// NewStoreWithoutWallet membuat toko yang belum menautkan wallet.
func (a *App) NewStoreWithoutWallet() Store {
	a.T.Helper()
	s := Store{ID: uuid.New()}
	a.Exec(`INSERT INTO stores (id, name) VALUES ($1, 'Toko Baru')`, s.ID)
	return s
}

// NewUser membuat akun yang sudah pernah login.
func (a *App) NewUser(storeID uuid.UUID, role auth.Role) User {
	a.T.Helper()
	u := User{ID: uuid.New(), StoreID: storeID, Subject: uuid.NewString()}
	u.Email = strings.ToLower(u.ID.String()) + "@test.id"
	a.Exec(`INSERT INTO users (id, store_id, role, email, name, auth_subject) VALUES ($1, $2, $3, $4, 'Pengguna', $5)`,
		u.ID, storeID, string(role), u.Email, u.Subject)
	return u
}

// NewConsignor membuat penitip dengan wallet terbukti.
func (a *App) NewConsignor(storeID uuid.UUID) User {
	a.T.Helper()
	u := a.NewUser(storeID, auth.RoleConsignor)
	u.ConsignorID = uuid.New()
	u.Wallet = NewWallet()
	a.Exec(`INSERT INTO consignors (id, store_id, user_id, name, wallet_address, wallet_verified_at) VALUES ($1, $2, $3, 'Penitip', $4, now())`,
		u.ConsignorID, storeID, u.ID, u.Wallet.Address)
	return u
}

// NewItem membuat barang.
func (a *App) NewItem(consignorID uuid.UUID, price int64, bps, stock int) uuid.UUID {
	a.T.Helper()
	id := uuid.New()
	a.Exec(`INSERT INTO items (id, consignor_id, name, price_idr, commission_bps, stock_on_hand) VALUES ($1, $2, 'Barang', $3, $4, $5)`,
		id, consignorID, price, bps, stock)
	return id
}

// TokenClaims adalah isi JWT test.
type TokenClaims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Issuer        string
	Audience      string
	ExpiresIn     time.Duration
	Method        jwt.SigningMethod
	Secret        string
}

// SignToken membuat JWT test.
func (a *App) SignToken(c TokenClaims) string {
	a.T.Helper()
	if c.Issuer == "" {
		c.Issuer = jwtIssuer
	}
	if c.Audience == "" {
		c.Audience = jwtAud
	}
	if c.ExpiresIn == 0 {
		c.ExpiresIn = time.Hour
	}
	if c.Method == nil {
		c.Method = jwt.SigningMethodHS256
	}
	if c.Secret == "" {
		c.Secret = jwtSecret
	}
	now := a.Clock.Now()
	tok := jwt.NewWithClaims(c.Method, jwt.MapClaims{
		"sub":           c.Subject,
		"email":         c.Email,
		"iss":           c.Issuer,
		"aud":           c.Audience,
		"iat":           now.Unix(),
		"exp":           now.Add(c.ExpiresIn).Unix(),
		"user_metadata": map[string]any{"email_verified": c.EmailVerified},
	})
	s, err := tok.SignedString([]byte(c.Secret))
	if err != nil {
		a.T.Fatalf("tanda tangani JWT: %v", err)
	}
	return s
}

// Token membuat JWT yang sah untuk pengguna.
func (a *App) Token(u User) string {
	a.T.Helper()
	return a.SignToken(TokenClaims{Subject: u.Subject, Email: u.Email, EmailVerified: true})
}

// Response adalah jawaban API.
type Response struct {
	t      testing.TB
	Status int
	Header http.Header
	Body   []byte
}

// JSON membaca isi jawaban.
func (r *Response) JSON(dst any) {
	r.t.Helper()
	if err := json.Unmarshal(r.Body, dst); err != nil {
		r.t.Fatalf("baca JSON: %v\n%s", err, r.Body)
	}
}

// Want memastikan status jawaban.
func (r *Response) Want(status int) *Response {
	r.t.Helper()
	if r.Status != status {
		r.t.Fatalf("status %d, seharusnya %d. Isi: %s", r.Status, status, r.Body)
	}
	return r
}

// WantProblem memastikan status dan kode error.
func (r *Response) WantProblem(status int, code string) *Response {
	r.t.Helper()
	r.Want(status)
	var p struct {
		Code string `json:"code"`
	}
	r.JSON(&p)
	if p.Code != code {
		r.t.Fatalf("kode %s, seharusnya %s. Isi: %s", p.Code, code, r.Body)
	}
	return r
}

// Request adalah permintaan ke API.
type Request struct {
	Method  string
	Path    string
	Body    any
	Token   string
	Headers map[string]string
	// Raw melewati validasi kontrak, untuk path yang memang tidak ada di kontrak.
	Raw bool
}

// Do mengirim permintaan dan memvalidasi jawabannya terhadap kontrak.
func (a *App) Do(req Request) *Response {
	a.T.Helper()
	var body *bytes.Reader
	switch b := req.Body.(type) {
	case nil:
		body = bytes.NewReader(nil)
	case string:
		body = bytes.NewReader([]byte(b))
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			a.T.Fatal(err)
		}
		body = bytes.NewReader(raw)
	}
	r := httptest.NewRequest(req.Method, req.Path, body)
	r.RemoteAddr = "203.0.113.10:4321"
	if req.Token != "" {
		r.Header.Set("Authorization", "Bearer "+req.Token)
	}
	for k, v := range req.Headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	a.API.Handler.ServeHTTP(w, r)
	res := &Response{t: a.T, Status: w.Code, Header: w.Header(), Body: w.Body.Bytes()}
	if !req.Raw {
		path := strings.SplitN(req.Path, "?", 2)[0]
		if err := a.Contract.Validate(req.Method, path, res.Status, res.Header, res.Body); err != nil {
			a.T.Fatalf("kontrak: %v", err)
		}
	}
	return res
}

// Get mengirim GET sebagai pengguna.
func (a *App) Get(u User, path string) *Response {
	a.T.Helper()
	return a.Do(Request{Method: http.MethodGet, Path: path, Token: a.Token(u)})
}

// Post mengirim POST sebagai pengguna.
func (a *App) Post(u User, path string, body any) *Response {
	a.T.Helper()
	return a.Do(Request{Method: http.MethodPost, Path: path, Token: a.Token(u), Body: body})
}

// Patch mengirim PATCH sebagai pengguna.
func (a *App) Patch(u User, path string, body any) *Response {
	a.T.Helper()
	return a.Do(Request{Method: http.MethodPatch, Path: path, Token: a.Token(u), Body: body})
}
