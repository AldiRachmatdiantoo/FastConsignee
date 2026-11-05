// Package config membaca konfigurasi dari environment.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Config berisi semua pengaturan API. Secret hanya datang dari environment.
type Config struct {
	Port         string
	DatabaseURL  string
	PublicAPIURL string
	FrontendURL  string
	IconURL      string
	AutoMigrate  bool

	JWTSecret   string
	JWKSURL     string
	JWTIssuer   string
	JWTAudience string

	SolanaRPCURL string

	CoinGeckoURL   string
	FixedIDRPerSOL int64
	RateTTL        time.Duration

	OrderTTL       time.Duration
	OrderGrace     time.Duration
	RefundTTL      time.Duration
	NonceTTL       time.Duration
	WorkerInterval time.Duration
	WorkerBatch    int

	RateLimitAccount  int
	RateLimitIP       int
	RateLimitOrder    int
	TrustedProxyHops  int
	CatalogCacheTTL   time.Duration
	DashboardCacheTTL time.Duration
}

// Load membaca Config lewat getenv, supaya mudah diuji.
func Load(getenv func(string) string) (Config, error) {
	r := reader{getenv: getenv}
	c := Config{
		Port:         r.str("PORT", "8080"),
		DatabaseURL:  r.str("DATABASE_URL", ""),
		PublicAPIURL: strings.TrimRight(r.str("PUBLIC_API_URL", "http://localhost:8080"), "/"),
		FrontendURL:  strings.TrimRight(r.str("FRONTEND_URL", "http://localhost:3000"), "/"),
		AutoMigrate:  r.boolean("AUTO_MIGRATE", true),

		JWTSecret:   r.str("SUPABASE_JWT_SECRET", ""),
		JWKSURL:     r.str("SUPABASE_JWKS_URL", ""),
		JWTIssuer:   r.str("SUPABASE_JWT_ISSUER", ""),
		JWTAudience: r.str("SUPABASE_JWT_AUDIENCE", "authenticated"),

		SolanaRPCURL: r.str("SOLANA_RPC_URL", "https://api.devnet.solana.com"),

		CoinGeckoURL:   r.str("COINGECKO_URL", "https://api.coingecko.com/api/v3/simple/price?ids=solana&vs_currencies=idr"),
		FixedIDRPerSOL: r.integer("FIXED_IDR_PER_SOL", 0),
		RateTTL:        r.duration("RATE_CACHE_TTL", 45*time.Second),

		OrderTTL:       r.duration("ORDER_TTL", 5*time.Minute),
		OrderGrace:     r.duration("ORDER_GRACE", 2*time.Minute),
		RefundTTL:      r.duration("REFUND_TTL", 5*time.Minute),
		NonceTTL:       r.duration("WALLET_NONCE_TTL", 5*time.Minute),
		WorkerInterval: r.duration("WORKER_INTERVAL", 2*time.Second),
		WorkerBatch:    int(r.integer("WORKER_BATCH", 50)),

		RateLimitAccount:  int(r.integer("RATE_LIMIT_ACCOUNT_PER_MINUTE", 120)),
		RateLimitIP:       int(r.integer("RATE_LIMIT_IP_PER_MINUTE", 60)),
		RateLimitOrder:    int(r.integer("RATE_LIMIT_ORDER_PER_MINUTE", 30)),
		TrustedProxyHops:  int(r.integer("TRUSTED_PROXY_HOPS", 0)),
		CatalogCacheTTL:   r.duration("CATALOG_CACHE_TTL", 5*time.Second),
		DashboardCacheTTL: r.duration("DASHBOARD_CACHE_TTL", 30*time.Second),
	}
	c.IconURL = r.str("STORE_ICON_URL", c.FrontendURL+"/icon.png")
	if r.err != nil {
		return Config{}, r.err
	}
	return c, c.validate()
}

func (c Config) validate() error {
	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL wajib diisi"))
	}
	if c.JWTSecret == "" && c.JWKSURL == "" {
		errs = append(errs, errors.New("isi SUPABASE_JWT_SECRET atau SUPABASE_JWKS_URL"))
	}
	if c.JWTIssuer == "" {
		errs = append(errs, errors.New("SUPABASE_JWT_ISSUER wajib diisi"))
	}
	if u, err := url.Parse(c.PublicAPIURL); err != nil || u.Host == "" {
		errs = append(errs, errors.New("PUBLIC_API_URL tidak valid"))
	}
	if strings.Contains(strings.ToLower(c.SolanaRPCURL), "mainnet") {
		errs = append(errs, errors.New("SOLANA_RPC_URL harus mengarah ke Devnet"))
	}
	return errors.Join(errs...)
}

// WalletDomain adalah domain yang tertulis di pesan penautan wallet.
func (c Config) WalletDomain() string {
	u, err := url.Parse(c.FrontendURL)
	if err != nil || u.Host == "" {
		return c.FrontendURL
	}
	return u.Host
}

type reader struct {
	getenv func(string) string
	err    error
}

func (r *reader) str(key, def string) string {
	if v := strings.TrimSpace(r.getenv(key)); v != "" {
		return v
	}
	return def
}

func (r *reader) integer(key string, def int64) int64 {
	v := strings.TrimSpace(r.getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		r.err = errors.Join(r.err, fmt.Errorf("%s harus berupa angka", key))
		return def
	}
	return n
}

func (r *reader) boolean(key string, def bool) bool {
	v := strings.TrimSpace(r.getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		r.err = errors.Join(r.err, fmt.Errorf("%s harus true atau false", key))
		return def
	}
	return b
}

func (r *reader) duration(key string, def time.Duration) time.Duration {
	v := strings.TrimSpace(r.getenv(key))
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		r.err = errors.Join(r.err, fmt.Errorf("%s harus berupa durasi, misalnya 5m", key))
		return def
	}
	return d
}
