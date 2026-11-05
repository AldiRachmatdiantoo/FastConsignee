package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/cache"
)

const (
	rateCacheKey = "rate:sol-idr"
	rateTimeout  = 3 * time.Second
	maxIDRPerSOL = 1_000_000_000_000
)

// RateProvider memberi kurs SOL ke Rupiah.
type RateProvider interface {
	Current(ctx context.Context) (Rate, error)
}

// MarketRate membaca kurs pasar dan menyimpannya sebentar di cache. Kalau pasar tidak bisa dibaca,
// nilai cadangan dipakai supaya kasir tetap jalan.
type MarketRate struct {
	url    string
	fixed  int64
	ttl    time.Duration
	cache  cache.Cache
	client *http.Client
	now    func() time.Time
}

// NewMarketRate membuat penyedia kurs. url kosong berarti selalu memakai nilai cadangan.
func NewMarketRate(url string, fixed int64, ttl time.Duration, c cache.Cache, now func() time.Time) *MarketRate {
	return &MarketRate{url: url, fixed: fixed, ttl: ttl, cache: c, client: &http.Client{Timeout: rateTimeout}, now: now}
}

// Current mengembalikan kurs terbaru.
func (m *MarketRate) Current(ctx context.Context) (Rate, error) {
	if v, ok := m.cache.Get(rateCacheKey); ok {
		return v.(Rate), nil
	}
	value, err := m.fetch(ctx)
	if err != nil {
		if m.url != "" {
			slog.Warn("kurs pasar tidak terbaca", "error", err)
		}
		if m.fixed <= 0 {
			return Rate{}, apperr.New(apperr.RateUnavailable, "Kurs SOL tidak tersedia. Coba lagi sebentar.")
		}
		value = m.fixed
	}
	rate := Rate{IDRPerSOL: value, QuotedAt: m.now().UTC()}
	m.cache.Set(rateCacheKey, rate, m.ttl)
	return rate, nil
}

func (m *MarketRate) fetch(ctx context.Context) (int64, error) {
	if m.url == "" {
		return 0, errors.New("URL kurs kosong")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil) // #nosec G704 -- URL datang dari konfigurasi server
	if err != nil {
		return 0, fmt.Errorf("susun permintaan: %w", err)
	}
	res, err := m.client.Do(req) // #nosec G704 -- URL datang dari konfigurasi server
	if err != nil {
		return 0, fmt.Errorf("baca kurs: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("kurs menjawab status %d", res.StatusCode)
	}
	var body struct {
		Solana struct {
			IDR float64 `json:"idr"`
		} `json:"solana"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("baca isi kurs: %w", err)
	}
	value := int64(math.Round(body.Solana.IDR))
	if value < 1 || value > maxIDRPerSOL {
		return 0, fmt.Errorf("kurs di luar batas: %v", body.Solana.IDR)
	}
	return value, nil
}
