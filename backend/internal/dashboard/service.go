package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/item"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/cache"
)

// Service membaca data dasbor.
type Service struct {
	repo     *Repository
	items    item.Repository
	cache    cache.Cache
	cacheTTL time.Duration
	now      func() time.Time
}

// NewService membuat service dasbor.
func NewService(repo *Repository, items item.Repository, c cache.Cache, cacheTTL time.Duration, now func() time.Time) *Service {
	return &Service{repo: repo, items: items, cache: c, cacheTTL: cacheTTL, now: now}
}

// Sales membaca penjualan satu penitip.
func (s *Service) Sales(ctx context.Context, consignorID uuid.UUID, period Period, p api.Page) ([]Sale, error) {
	return s.repo.sales(ctx, consignorID, period, p)
}

// Deductions membaca potongan satu penitip. Hasilnya tidak di-cache, karena menyangkut uang.
func (s *Service) Deductions(ctx context.Context, consignorID uuid.UUID, status *string, p api.Page) ([]Deduction, int64, error) {
	rows, err := s.repo.deductions(ctx, consignorID, status, p)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.outstandingTotal(ctx, consignorID)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// Stock membaca stok satu penitip dari sumber yang sama dengan katalog kasir.
func (s *Service) Stock(ctx context.Context, storeID, consignorID uuid.UUID, p api.Page) ([]item.Item, error) {
	return s.items.List(ctx, storeID, item.Filter{ConsignorID: &consignorID}, p)
}

func stamp(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format(time.RFC3339)
}

// Summary membaca ringkasan toko, dengan cache singkat.
func (s *Service) Summary(ctx context.Context, storeID uuid.UUID, period Period) (Summary, error) {
	key := fmt.Sprintf("dashboard:%s:%s:%s", storeID, stamp(period.From), stamp(period.To))
	if v, ok := s.cache.Get(key); ok {
		return v.(Summary), nil
	}
	out, err := s.repo.summary(ctx, storeID, period)
	if err != nil {
		return Summary{}, err
	}
	out.GeneratedAt = s.now().UTC()
	s.cache.Set(key, out, s.cacheTTL)
	return out, nil
}
