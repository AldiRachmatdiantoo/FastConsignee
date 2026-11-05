// Package app menyusun semua fitur menjadi satu API.
package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/cache"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/config"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/middleware"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
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
	Handler http.Handler
	Mux     *http.ServeMux
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

	return &App{
		Mux: mux,
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
