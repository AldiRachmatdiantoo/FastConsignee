package item

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/cache"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
)

// Service memegang aturan barang dan cache katalog.
type Service struct {
	repo     Repository
	tx       database.TxRunner
	cache    cache.Cache
	cacheTTL time.Duration
}

// NewService membuat service barang.
func NewService(repo Repository, tx database.TxRunner, c cache.Cache, cacheTTL time.Duration) *Service {
	return &Service{repo: repo, tx: tx, cache: c, cacheTTL: cacheTTL}
}

// CatalogPrefix adalah awalan cache katalog satu toko.
func CatalogPrefix(storeID uuid.UUID) string {
	return "catalog:" + storeID.String() + ":"
}

func notFound() error {
	return apperr.New(apperr.NotFound, "Item not found.")
}

// List membaca katalog. Halaman pertama tanpa pencarian di-cache sebentar.
func (s *Service) List(ctx context.Context, storeID uuid.UUID, f Filter, p api.Page) ([]Item, error) {
	key := ""
	if p.Cursor == nil && f.Query == nil && f.ConsignorID == nil {
		key = fmt.Sprintf("%s%v:%d", CatalogPrefix(storeID), f.Active != nil && *f.Active, p.Limit)
		if f.Active == nil {
			key = ""
		}
	}
	if key != "" {
		if v, ok := s.cache.Get(key); ok {
			return v.([]Item), nil
		}
	}
	items, err := s.repo.List(ctx, storeID, f, p)
	if err != nil {
		return nil, err
	}
	if key != "" {
		s.cache.Set(key, items, s.cacheTTL)
	}
	return items, nil
}

// Get membaca satu barang.
func (s *Service) Get(ctx context.Context, storeID, id uuid.UUID) (Item, error) {
	i, err := s.repo.Get(ctx, storeID, id)
	if err != nil {
		return Item{}, err
	}
	if i == nil {
		return Item{}, notFound()
	}
	return *i, nil
}

// Create menyimpan barang baru.
func (s *Service) Create(ctx context.Context, storeID uuid.UUID, c Create) (Item, error) {
	ok, err := s.repo.ConsignorInStore(ctx, storeID, c.ConsignorID)
	if err != nil {
		return Item{}, err
	}
	if !ok {
		return Item{}, apperr.New(apperr.NotFound, "Consignor not found.")
	}
	id := uuid.New()
	if err := s.repo.Insert(ctx, id, c); err != nil {
		return Item{}, err
	}
	s.cache.DeletePrefix(CatalogPrefix(storeID))
	return s.Get(ctx, storeID, id)
}

// Update mengubah barang. Stok tidak boleh turun di bawah jumlah yang tertahan order.
func (s *Service) Update(ctx context.Context, storeID, id uuid.UUID, u Update) (Item, error) {
	var out Item
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		i, err := s.repo.Lock(ctx, storeID, id)
		if err != nil {
			return err
		}
		if i == nil {
			return notFound()
		}
		apply(i, u)
		if i.StockOnHand < i.StockReserved {
			return apperr.New(apperr.StockBelowReserved, "Stock cannot be lower than the stock held by orders.").
				WithDetails(map[string]any{"stock_reserved": i.StockReserved})
		}
		if err := s.repo.Save(ctx, *i); err != nil {
			return err
		}
		out = *i
		return nil
	})
	if err != nil {
		return Item{}, err
	}
	s.cache.DeletePrefix(CatalogPrefix(storeID))
	return s.Get(ctx, storeID, out.ID)
}

func apply(i *Item, u Update) {
	if u.Name != nil {
		i.Name = *u.Name
	}
	if u.PriceIDR != nil {
		i.PriceIDR = *u.PriceIDR
	}
	if u.CommissionBps != nil {
		i.CommissionBps = *u.CommissionBps
	}
	if u.StockOnHand != nil {
		i.StockOnHand = *u.StockOnHand
	}
	if u.Active != nil {
		i.Active = *u.Active
	}
	if u.CertificateEnabled != nil {
		i.CertificateEnabled = *u.CertificateEnabled
	}
}
