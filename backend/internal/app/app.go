// Package app menyusun semua fitur menjadi satu API.
package app

import (
	"context"
	"log/slog"
	"net/http"
	"time"

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
	Config config.Config
	DB     *database.DB
	Solana solana.Client
	Cache  cache.Cache
	Now    func() time.Time
}

// App adalah API yang sudah tersusun.
type App struct {
	Handler http.Handler
	Mux     *http.ServeMux
}

// New menyusun route dan middleware.
func New(d Deps) *App {
	if d.Now == nil {
		d.Now = time.Now
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health(d.DB))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		api.Error(w, r, apperr.New(apperr.NotFound, "Endpoint tidak ditemukan."))
	})
	return &App{
		Mux: mux,
		Handler: middleware.Chain(mux,
			middleware.RequestID,
			middleware.Logger,
			middleware.Recover,
			middleware.SecurityHeaders,
			middleware.CORS(d.Config.FrontendURL),
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
