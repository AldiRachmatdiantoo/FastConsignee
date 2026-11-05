// Package app menyusun semua fitur menjadi satu API.
package app

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/consignor"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/item"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/order"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/payment"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/cache"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/config"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/middleware"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/refund"
)

// Deps adalah semua dependensi luar.
type Deps struct {
	Config   config.Config
	DB       *database.DB
	Solana   solana.Client
	Cache    cache.Cache
	Verifier auth.Verifier
	Now      func() time.Time
}

// App adalah API yang sudah tersusun.
type App struct {
	Handler  http.Handler
	Mux      *http.ServeMux
	Payments *payment.Worker
	Refunds  *refund.Worker
}

type routes struct {
	mux    *http.ServeMux
	guard  *auth.Guard
	public *publicLimiter
}

// login memasang pemeriksaan JWT saja.
func (r routes) login(pattern string, h http.HandlerFunc, m ...middleware.Middleware) {
	r.mux.Handle(pattern, middleware.Chain(h, append([]middleware.Middleware{r.guard.Authenticated}, m...)...))
}

// member memasang JWT, penjaga wallet, dan peran.
func (r routes) member(pattern string, h http.HandlerFunc, roles ...auth.Role) {
	r.login(pattern, h, auth.RequireWallet, auth.RequireRole(roles...))
}

// New menyusun route dan middleware.
func New(d Deps) *App {
	if d.Now == nil {
		d.Now = time.Now
	}
	cfg := d.Config
	mux := http.NewServeMux()

	authRepo := auth.NewRepository(d.DB)
	authService := auth.NewService(authRepo, d.DB, d.Now, cfg.WalletDomain(), cfg.NonceTTL)
	authHandler := auth.NewHandler(authService)

	accounts := middleware.NewLimiter(cfg.RateLimitAccount, d.Now)
	ips := middleware.NewLimiter(cfg.RateLimitIP, d.Now)
	orders := middleware.NewLimiter(cfg.RateLimitOrder, d.Now)
	r := routes{
		mux:    mux,
		guard:  auth.NewGuard(d.Verifier, authService, accounts, ips, cfg.TrustedProxyHops),
		public: &publicLimiter{ips: ips, orders: orders, proxyHops: cfg.TrustedProxyHops},
	}

	mux.HandleFunc("GET /healthz", health(d.DB))
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		api.Error(w, req, apperr.New(apperr.NotFound, "Endpoint tidak ditemukan."))
	})

	r.login("GET /api/v1/me", authHandler.Me)
	r.login("POST /api/v1/wallet/nonce", authHandler.CreateNonce, auth.RequireRole(auth.RoleOwner, auth.RoleConsignor))
	r.login("POST /api/v1/wallet/link", authHandler.LinkWallet, auth.RequireRole(auth.RoleOwner, auth.RoleConsignor))

	itemService := item.NewService(item.NewRepository(d.DB), d.DB, d.Cache, cfg.CatalogCacheTTL)
	itemHandler := item.NewHandler(itemService)
	r.member("GET /api/v1/items", itemHandler.List, auth.RoleCashier, auth.RoleOwner)
	r.member("POST /api/v1/items", itemHandler.Create, auth.RoleOwner)
	r.member("GET /api/v1/items/{item_id}", itemHandler.Get, auth.RoleCashier, auth.RoleOwner)
	r.member("PATCH /api/v1/items/{item_id}", itemHandler.Update, auth.RoleOwner)

	consignorHandler := consignor.NewHandler(consignor.NewService(consignor.NewRepository(d.DB), d.DB))
	r.member("GET /api/v1/consignors", consignorHandler.List, auth.RoleOwner)
	r.member("POST /api/v1/consignors", consignorHandler.Create, auth.RoleOwner)
	r.member("GET /api/v1/consignors/{consignor_id}", consignorHandler.Get, auth.RoleOwner)
	r.member("PATCH /api/v1/consignors/{consignor_id}", consignorHandler.Update, auth.RoleOwner)

	orderRepo := order.NewRepository(d.DB)
	rates := order.NewMarketRate(cfg.CoinGeckoURL, cfg.FixedIDRPerSOL, cfg.RateTTL, d.Cache, d.Now)
	orderService := order.NewService(orderRepo, d.DB, rates, d.Solana, d.Cache, cfg.OrderTTL, d.Now)
	orderHandler := order.NewHandler(orderService, cfg.PublicAPIURL)
	r.member("POST /api/v1/orders/preview", orderHandler.Preview, auth.RoleCashier, auth.RoleOwner)
	r.member("POST /api/v1/orders", orderHandler.Create, auth.RoleCashier, auth.RoleOwner)
	r.member("GET /api/v1/orders", orderHandler.List, auth.RoleCashier, auth.RoleOwner)
	r.member("GET /api/v1/orders/{order_id}", orderHandler.Get, auth.RoleCashier, auth.RoleOwner)
	r.member("GET /api/v1/orders/{order_id}/status", orderHandler.Status, auth.RoleCashier, auth.RoleOwner)

	paymentRepo := payment.NewRepository(d.DB)
	paymentHandler := payment.NewHandler(payment.NewService(paymentRepo, orderRepo, d.Solana, cfg.IconURL, d.Now), orderRepo, cfg.PublicAPIURL)
	mux.HandleFunc("GET /api/v1/public/orders/{order_id}", r.public.limited(paymentHandler.PublicOrder))
	mux.HandleFunc("GET /api/v1/solana-pay/orders/{order_id}", r.public.solanaPay(paymentHandler.Metadata))
	mux.HandleFunc("POST /api/v1/solana-pay/orders/{order_id}", r.public.solanaPay(paymentHandler.Transaction))

	refundRepo := refund.NewRepository(d.DB)
	refundHandler := refund.NewHandler(refund.NewService(refundRepo, orderRepo, d.DB, d.Solana, cfg.RefundTTL, d.Now))
	r.member("POST /api/v1/orders/{order_id}/refunds", refundHandler.Create, auth.RoleOwner)
	r.member("GET /api/v1/refunds", refundHandler.List, auth.RoleOwner)
	r.member("GET /api/v1/refunds/{refund_id}", refundHandler.Get, auth.RoleOwner)
	r.member("POST /api/v1/refunds/{refund_id}/signature", refundHandler.Signature, auth.RoleOwner)

	return &App{
		Mux:      mux,
		Payments: payment.NewWorker(paymentRepo, d.DB, d.Solana, d.Cache, cfg.WorkerBatch, cfg.OrderGrace, d.Now),
		Refunds:  refund.NewWorker(refundRepo, d.DB, d.Solana, cfg.WorkerBatch, cfg.OrderGrace, d.Now),
		Handler: middleware.Chain(mux,
			middleware.RequestID,
			middleware.Logger,
			middleware.Recover,
			middleware.SecurityHeaders,
			middleware.CORS(cfg.FrontendURL),
		),
	}
}

func health(db *database.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.Ping(ctx); err != nil {
			slog.Error("health check gagal", "error", err)
			api.Error(w, r, apperr.New(apperr.ServiceUnavailable, "Database tidak bisa dihubungi."))
			return
		}
		api.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// publicLimiter membatasi endpoint tanpa login per IP dan per order.
type publicLimiter struct {
	ips       *middleware.Limiter
	orders    *middleware.Limiter
	proxyHops int
}

// check menulis header RateLimit-* dari batas yang paling ketat.
func (p *publicLimiter) check(w http.ResponseWriter, r *http.Request) middleware.Decision {
	d := p.ips.Allow(middleware.ClientIP(r, p.proxyHops))
	o := p.orders.Allow(r.PathValue("order_id"))
	if !o.Allowed || (d.Allowed && o.Remaining < d.Remaining) {
		d = o
	}
	middleware.RateHeaders(w, d)
	return d
}

// limited membungkus handler publik yang memakai format error problem+json.
func (p *publicLimiter) limited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d := p.check(w, r); !d.Allowed {
			middleware.Reject(w, r, d)
			return
		}
		next(w, r)
	}
}

// solanaPay membungkus handler Solana Pay, yang memakai format error `{"message": ...}`.
func (p *publicLimiter) solanaPay(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if d := p.check(w, r); !d.Allowed {
			w.Header().Set("Retry-After", strconv.Itoa(max(d.RetryAfterSeconds, 1)))
			payment.WriteError(w, http.StatusTooManyRequests, "Terlalu banyak permintaan. Coba lagi sebentar lagi.")
			return
		}
		next(w, r)
	}
}

// RunWorkers menjalankan worker sampai ctx selesai. Panic di satu putaran hanya menghentikan putaran itu.
func (a *App) RunWorkers(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.tick(ctx)
		}
	}
}

func (a *App) tick(ctx context.Context) {
	defer func() {
		if v := recover(); v != nil {
			slog.Error("worker panic", "value", v)
		}
	}()
	if err := a.Payments.Tick(ctx); err != nil {
		slog.Error("worker pembayaran gagal", "error", err)
	}
	if err := a.Refunds.Tick(ctx); err != nil {
		slog.Error("worker refund gagal", "error", err)
	}
}
