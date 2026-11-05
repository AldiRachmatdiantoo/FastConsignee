package auth

import (
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/middleware"
)

// Guard melindungi route yang butuh login.
type Guard struct {
	verifier  Verifier
	service   *Service
	accounts  *middleware.Limiter
	ips       *middleware.Limiter
	proxyHops int
}

// NewGuard membuat penjaga route.
func NewGuard(v Verifier, s *Service, accounts, ips *middleware.Limiter, proxyHops int) *Guard {
	return &Guard{verifier: v, service: s, accounts: accounts, ips: ips, proxyHops: proxyHops}
}

func bearer(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

// Authenticated memverifikasi JWT, menerapkan rate limit, dan memuat Principal.
// Permintaan tanpa token yang sah dibatasi per IP, supaya tebakan token tetap terbatas.
func (g *Guard) Authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := g.identify(r)
		if err != nil {
			d := g.ips.Allow(middleware.ClientIP(r, g.proxyHops))
			middleware.RateHeaders(w, d)
			if !d.Allowed {
				middleware.Reject(w, r, d)
				return
			}
			api.Error(w, r, apperr.New(apperr.Unauthenticated, "Login diperlukan."))
			return
		}
		d := g.accounts.Allow(id.Subject)
		middleware.RateHeaders(w, d)
		if !d.Allowed {
			middleware.Reject(w, r, d)
			return
		}
		p, err := g.service.Resolve(r.Context(), id)
		if err != nil {
			api.Error(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
	})
}

func (g *Guard) identify(r *http.Request) (Identity, error) {
	token, ok := bearer(r)
	if !ok {
		return Identity{}, ErrInvalidToken
	}
	id, err := g.verifier.Verify(r.Context(), token)
	if err != nil {
		return Identity{}, errors.Join(ErrInvalidToken, err)
	}
	return id, nil
}

// RequireWallet menolak pemilik toko dan penitip yang belum menautkan wallet.
func RequireWallet(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := MustFromContext(r.Context())
		if p.WalletRequired() && !p.HasWallet() {
			api.Error(w, r, apperr.New(apperr.WalletNotLinked, "Tautkan wallet lebih dulu."))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole hanya meloloskan peran tertentu.
func RequireRole(roles ...Role) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !slices.Contains(roles, MustFromContext(r.Context()).Role) {
				api.Error(w, r, apperr.New(apperr.Forbidden, "Peran ini tidak boleh mengakses data tersebut."))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
