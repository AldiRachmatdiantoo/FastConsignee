package order

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/item"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/cache"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

const (
	createAttempts = 3
	rentCacheKey   = "solana:rent"
	rentCacheTTL   = time.Hour
)

var (
	// errPlanChanged berarti pembagian berubah setelah saldo minimum diperiksa di luar transaksi.
	errPlanChanged = errors.New("pembagian berubah")
	// errDuplicateKey berarti permintaan lain dengan kunci sama selesai lebih dulu.
	errDuplicateKey = errors.New("idempotency key sudah dipakai")
)

// Service memegang aturan keranjang dan order.
type Service struct {
	repo     *Repository
	tx       database.TxRunner
	rates    RateProvider
	solana   solana.Client
	cache    cache.Cache
	orderTTL time.Duration
	now      func() time.Time
}

// NewService membuat service order.
func NewService(repo *Repository, tx database.TxRunner, rates RateProvider, client solana.Client, c cache.Cache,
	orderTTL time.Duration, now func() time.Time) *Service {
	return &Service{repo: repo, tx: tx, rates: rates, solana: client, cache: c, orderTTL: orderTTL, now: now}
}

func notFound() error {
	return apperr.New(apperr.NotFound, "Order tidak ditemukan.")
}

func unavailable(err error) error {
	return fmt.Errorf("%w: %w", apperr.New(apperr.ServiceUnavailable, "Jaringan Solana tidak bisa dihubungi."), err)
}

func itemIDs(cart Cart) []uuid.UUID {
	ids := make([]uuid.UUID, len(cart.Lines))
	for i, l := range cart.Lines {
		ids[i] = l.ItemID
	}
	return ids
}

// plan menghitung keranjang dari data terbaru. Dengan rate nil, kurs dibaca setelah keranjang valid.
// Fungsi yang sama dipakai untuk rincian dan di dalam transaksi order.
func (s *Service) plan(ctx context.Context, storeID uuid.UUID, cart Cart, rate *Rate) (Plan, error) {
	ids := itemIDs(cart)
	items, err := s.repo.cartItems(ctx, storeID, ids)
	if err != nil {
		return Plan{}, err
	}
	for _, id := range ids {
		if _, ok := items[id]; !ok {
			return Plan{}, apperr.New(apperr.NotFound, "Barang tidak ditemukan.").WithDetails(map[string]any{"item_id": id})
		}
	}
	reserved, err := s.repo.reserved(ctx, ids)
	if err != nil {
		return Plan{}, err
	}
	if err := checkStock(cart, items, reserved); err != nil {
		return Plan{}, err
	}
	p := Plan{}
	consignors := []item.ConsignorRef{}
	shares := map[uuid.UUID]int64{}
	wallets := map[uuid.UUID]*string{}
	var commission int64
	for _, l := range cart.Lines {
		it := items[l.ItemID]
		if _, seen := shares[it.Consignor.ID]; !seen {
			consignors = append(consignors, it.Consignor)
			shares[it.Consignor.ID] = 0
			wallets[it.Consignor.ID] = it.Wallet
		}
	}
	if len(consignors) > MaxConsignors {
		return Plan{}, apperr.New(apperr.TooManyConsignors, "Satu order memuat paling banyak 5 penitip.").
			WithDetails(map[string]any{"consignors": len(consignors), "maximum": MaxConsignors})
	}
	for _, l := range cart.Lines {
		it := items[l.ItemID]
		discount, bearer := int64(0), ""
		if l.Discount != nil {
			discount, bearer = l.Discount.AmountIDR, l.Discount.BorneBy
		}
		amounts, err := ComputeLine(it.PriceIDR, l.Quantity, it.CommissionBps, discount, bearer)
		if err != nil {
			return Plan{}, apperr.New(apperr.DiscountTooLarge, "Diskon melebihi bagian penanggungnya.").
				WithDetails(map[string]any{"item_id": it.ID})
		}
		p.Lines = append(p.Lines, Line{
			ItemID: it.ID, ItemName: it.Name, Consignor: it.Consignor, Quantity: l.Quantity,
			UnitPriceIDR: it.PriceIDR, CommissionBps: it.CommissionBps, Discount: l.Discount, Amounts: amounts,
		})
		p.Totals.SubtotalIDR += amounts.Gross
		p.Totals.DiscountIDR += amounts.Discount
		shares[it.Consignor.ID] += amounts.ConsignorShare
		commission += amounts.Commission
	}
	if p.Totals.SubtotalIDR == p.Totals.DiscountIDR {
		return Plan{}, apperr.New(apperr.DiscountTooLarge, "Total order harus lebih dari 0.")
	}
	missing := []map[string]any{}
	for _, c := range consignors {
		if wallets[c.ID] == nil {
			missing = append(missing, map[string]any{"consignor_id": c.ID, "name": c.Name})
		}
	}
	if len(missing) > 0 {
		return Plan{}, apperr.New(apperr.RecipientWalletMissing, "Penitip belum punya wallet.").
			WithDetails(map[string]any{"consignors": missing})
	}
	store, err := s.repo.store(ctx, storeID)
	if err != nil {
		return Plan{}, err
	}
	if store.Wallet == nil {
		return Plan{}, apperr.New(apperr.StoreWalletNotLinked, "Toko belum menautkan wallet.")
	}
	if rate == nil {
		current, err := s.rates.Current(ctx)
		if err != nil {
			return Plan{}, err
		}
		rate = &current
	}
	p.Rate = *rate

	consignorIDs := make([]uuid.UUID, len(consignors))
	for i, c := range consignors {
		consignorIDs[i] = c.ID
	}
	outstanding, err := s.repo.outstanding(ctx, consignorIDs)
	if err != nil {
		return Plan{}, err
	}
	owed := map[uuid.UUID]int64{}
	for _, o := range outstanding {
		owed[o.ConsignorID] += o.RemainingIDR
	}
	inputs := make([]Share, len(consignors))
	for i, c := range consignors {
		inputs[i] = Share{ConsignorID: c.ID, Name: c.Name, Wallet: *wallets[c.ID], ShareIDR: shares[c.ID], OutstandingIDR: owed[c.ID]}
	}
	p.Split, err = BuildSplit(inputs, StoreShare{Name: store.Name, Wallet: *store.Wallet, ShareIDR: commission}, rate.IDRPerSOL)
	if errors.Is(err, ErrAmountTooLarge) {
		return Plan{}, apperr.Validation(apperr.FieldError{Field: "lines", Message: "Total order terlalu besar."})
	}
	if err != nil {
		return Plan{}, err
	}
	taken := map[uuid.UUID]int64{}
	for _, r := range p.Split.Recipients {
		if r.ConsignorID != nil {
			taken[*r.ConsignorID] = r.DeductionIDR
		}
	}
	p.Settlements = Allocate(outstanding, taken)
	return p, nil
}

func checkStock(cart Cart, items map[uuid.UUID]cartItem, reserved map[uuid.UUID]int32) error {
	short := []map[string]any{}
	for _, l := range cart.Lines {
		it := items[l.ItemID]
		available := int32(0)
		if it.Active {
			available = max(it.StockOnHand-reserved[it.ID], 0)
		}
		if l.Quantity > available {
			short = append(short, map[string]any{"item_id": it.ID, "requested": l.Quantity, "available": available})
		}
	}
	if len(short) == 0 {
		return nil
	}
	return apperr.New(apperr.InsufficientStock, "Stok beberapa barang tidak cukup.").WithDetails(map[string]any{"items": short})
}

// checkRent menolak transfer ke wallet kosong yang lebih kecil dari minimum rent, karena jaringan
// akan menggagalkan seluruh transaksi. Dipanggil di luar transaksi database.
func (s *Service) checkRent(ctx context.Context, split Split) error {
	minimum, err := s.rent(ctx)
	if err != nil {
		return err
	}
	for _, r := range split.Recipients {
		if r.AmountLamports == 0 || uint64(r.AmountLamports) >= minimum { // #nosec G115 -- BuildSplit tidak menghasilkan nilai negatif
			continue
		}
		balance, err := s.solana.Balance(ctx, r.Wallet)
		if err != nil {
			return unavailable(err)
		}
		if balance == 0 {
			return apperr.New(apperr.BelowRentMinimum, "Bagian penerima terlalu kecil untuk wallet yang masih kosong.").
				WithDetails(map[string]any{
					"wallet_address":   r.Wallet,
					"amount_lamports":  strconv.FormatInt(r.AmountLamports, 10),
					"minimum_lamports": strconv.FormatUint(minimum, 10),
				})
		}
	}
	return nil
}

func (s *Service) rent(ctx context.Context) (uint64, error) {
	if v, ok := s.cache.Get(rentCacheKey); ok {
		return v.(uint64), nil
	}
	minimum, err := s.solana.RentExemptMinimum(ctx)
	if err != nil {
		return 0, unavailable(err)
	}
	s.cache.Set(rentCacheKey, minimum, rentCacheTTL)
	return minimum, nil
}

// Preview menghitung rincian tanpa menyimpan apa pun.
func (s *Service) Preview(ctx context.Context, p auth.Principal, cart Cart) (Plan, error) {
	plan, err := s.plan(ctx, p.StoreID, cart, nil)
	if err != nil {
		return Plan{}, err
	}
	if err := s.checkRent(ctx, plan.Split); err != nil {
		return Plan{}, err
	}
	return plan, nil
}

func requestHash(cart Cart) (string, error) {
	b, err := json.Marshal(cart)
	if err != nil {
		return "", fmt.Errorf("hash keranjang: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// replay mengembalikan order lama kalau kunci yang sama pernah dipakai.
func (s *Service) replay(ctx context.Context, p auth.Principal, key, hash string) (*Order, error) {
	id, stored, err := s.repo.idempotent(ctx, p.UserID, key)
	if id == nil || err != nil {
		return nil, err
	}
	if stored != hash {
		return nil, apperr.New(apperr.IdempotencyKeyReused, "Idempotency-Key ini sudah dipakai untuk keranjang lain.")
	}
	o, err := s.Get(ctx, p, *id)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// Create menyimpan order. Saldo wallet diperiksa di luar transaksi, jadi transaksinya menghitung ulang
// di bawah kunci dan mengulang kalau hasilnya berubah. Nilai replayed true berarti order lama.
func (s *Service) Create(ctx context.Context, p auth.Principal, key string, cart Cart) (o Order, replayed bool, err error) {
	hash, err := requestHash(cart)
	if err != nil {
		return Order{}, false, err
	}
	if old, err := s.replay(ctx, p, key, hash); old != nil || err != nil {
		if err != nil {
			return Order{}, false, err
		}
		return *old, true, nil
	}
	var rate *Rate
	for range createAttempts {
		plan, err := s.plan(ctx, p.StoreID, cart, rate)
		if err != nil {
			// Permintaan kembar bisa menghabiskan stok tepat sebelum baris ini.
			if old, replayErr := s.replay(ctx, p, key, hash); old != nil && replayErr == nil {
				return *old, true, nil
			}
			return Order{}, false, err
		}
		rate = &plan.Rate
		if err := s.checkRent(ctx, plan.Split); err != nil {
			return Order{}, false, err
		}
		id, err := s.commit(ctx, p, key, hash, cart, plan)
		switch {
		case errors.Is(err, errPlanChanged):
			continue
		case errors.Is(err, errDuplicateKey), database.IsUniqueViolation(err, "orders_created_by_idempotency_key_key"):
			old, err := s.replay(ctx, p, key, hash)
			if err != nil {
				return Order{}, false, err
			}
			return *old, true, nil
		case err != nil:
			return Order{}, false, err
		}
		s.cache.DeletePrefix(item.CatalogPrefix(p.StoreID))
		created, err := s.Get(ctx, p, id)
		return created, false, err
	}
	return Order{}, false, apperr.New(apperr.ServiceUnavailable, "Keranjang terus berubah. Coba lagi.")
}

func (s *Service) commit(ctx context.Context, p auth.Principal, key, hash string, cart Cart, planned Plan) (uuid.UUID, error) {
	reference, err := solana.NewReference()
	if err != nil {
		return uuid.Nil, err
	}
	id := uuid.New()
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.lockItems(ctx, itemIDs(cart)); err != nil {
			return err
		}
		if existing, _, err := s.repo.idempotent(ctx, p.UserID, key); err != nil {
			return err
		} else if existing != nil {
			return errDuplicateKey
		}
		consignors := make([]uuid.UUID, 0, len(planned.Split.Recipients))
		for _, r := range planned.Split.Recipients {
			if r.ConsignorID != nil {
				consignors = append(consignors, *r.ConsignorID)
			}
		}
		if err := s.repo.lockDeductions(ctx, consignors); err != nil {
			return err
		}
		fresh, err := s.plan(ctx, p.StoreID, cart, &planned.Rate)
		if err != nil {
			return err
		}
		if !fresh.Split.Same(planned.Split) {
			return errPlanChanged
		}
		return s.repo.insert(ctx, newOrder{
			ID: id, CreatedBy: p.UserID, Reference: reference, Key: key, Hash: hash,
			ExpiresAt: s.now().Add(s.orderTTL), Plan: fresh,
		})
	})
	return id, err
}

// Get membaca detail order.
func (s *Service) Get(ctx context.Context, p auth.Principal, id uuid.UUID) (Order, error) {
	o, err := s.repo.Get(ctx, ScopeOf(p), id)
	if err != nil {
		return Order{}, err
	}
	if o == nil {
		return Order{}, notFound()
	}
	return *o, nil
}

// List membaca riwayat order.
func (s *Service) List(ctx context.Context, p auth.Principal, f Filter, page api.Page) ([]Summary, error) {
	return s.repo.list(ctx, ScopeOf(p), f, page)
}

// Status membaca status order dari database, tanpa cache.
func (s *Service) Status(ctx context.Context, p auth.Principal, id uuid.UUID) (StatusView, error) {
	v, err := s.repo.status(ctx, ScopeOf(p), id)
	if err != nil {
		return StatusView{}, err
	}
	if v == nil {
		return StatusView{}, notFound()
	}
	return *v, nil
}
