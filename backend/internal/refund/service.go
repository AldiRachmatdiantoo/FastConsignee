package refund

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/order"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

// errDuplicateKey berarti permintaan lain dengan kunci sama selesai lebih dulu.
var errDuplicateKey = errors.New("idempotency key sudah dipakai")

// Service memegang aturan refund.
type Service struct {
	repo   *Repository
	orders *order.Repository
	tx     database.TxRunner
	solana solana.Client
	ttl    time.Duration
	now    func() time.Time
}

// NewService membuat service refund.
func NewService(repo *Repository, orders *order.Repository, tx database.TxRunner, client solana.Client,
	ttl time.Duration, now func() time.Time) *Service {
	return &Service{repo: repo, orders: orders, tx: tx, solana: client, ttl: ttl, now: now}
}

func notFound() error {
	return apperr.New(apperr.NotFound, "Refund tidak ditemukan.")
}

// Input adalah permintaan refund yang sudah lolos validasi format.
type Input struct {
	OrderID uuid.UUID
	LineID  uuid.UUID
	Reason  *string
}

func (in Input) hash() string {
	reason := ""
	if in.Reason != nil {
		reason = *in.Reason
	}
	sum := sha256.Sum256([]byte(in.OrderID.String() + "|" + in.LineID.String() + "|" + reason))
	return hex.EncodeToString(sum[:])
}

// Draft adalah refund beserta transaksi yang harus ditandatangani wallet toko.
type Draft struct {
	Refund      order.Refund
	Transaction string
}

func (s *Service) load(ctx context.Context, p auth.Principal, in Input) (*order.Order, order.Line, error) {
	o, err := s.orders.Get(ctx, order.ScopeOf(p), in.OrderID)
	if err != nil {
		return nil, order.Line{}, err
	}
	if o == nil {
		return nil, order.Line{}, apperr.New(apperr.NotFound, "Order tidak ditemukan.")
	}
	for _, l := range o.Lines {
		if l.ID == in.LineID {
			return o, l, nil
		}
	}
	return nil, order.Line{}, apperr.New(apperr.NotFound, "Barang ini tidak ada di order.")
}

// nextUnit menghitung refund berikutnya pada satu baris.
func nextUnit(o *order.Order, l order.Line) (Unit, error) {
	if o.Status != order.StatusConfirmed {
		return Unit{}, apperr.New(apperr.OrderNotRefundable, "Hanya order yang sudah dibayar yang bisa di-refund.")
	}
	if l.RefundedQuantity >= l.Quantity {
		return Unit{}, apperr.New(apperr.AlreadyRefunded, "Semua unit barang ini sudah di-refund.")
	}
	u, err := ValueOf(l, o.Rate.IDRPerSOL, l.RefundedQuantity+1)
	if err != nil {
		return Unit{}, err
	}
	if u.AmountLamports <= 0 {
		return Unit{}, apperr.New(apperr.OrderNotRefundable, "Nilai refund unit ini 0.")
	}
	return u, nil
}

func (s *Service) transaction(ctx context.Context, f order.Refund) (string, error) {
	blockhash, err := s.solana.LatestBlockhash(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %w", apperr.New(apperr.ServiceUnavailable, "Jaringan Solana tidak bisa dihubungi."), err)
	}
	return solana.BuildTransfers(f.SourceWallet, blockhash, f.Reference, []solana.Transfer{{
		From: f.SourceWallet, To: f.DestinationWallet, Lamports: uint64(f.AmountLamports),
	}})
}

func (s *Service) draft(ctx context.Context, storeID, id uuid.UUID) (Draft, error) {
	f, err := s.Get(ctx, storeID, id)
	if err != nil {
		return Draft{}, err
	}
	encoded, err := s.transaction(ctx, f)
	if err != nil {
		return Draft{}, err
	}
	return Draft{Refund: f, Transaction: encoded}, nil
}

func (s *Service) replay(ctx context.Context, p auth.Principal, key, hash string) (*Draft, error) {
	id, stored, err := s.repo.idempotent(ctx, p.UserID, key)
	if id == nil || err != nil {
		return nil, err
	}
	if stored != hash {
		return nil, apperr.New(apperr.IdempotencyKeyReused, "Idempotency-Key ini sudah dipakai untuk refund lain.")
	}
	d, err := s.draft(ctx, p.StoreID, *id)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// Create menyimpan refund satu unit. Blockhash dibaca setelah transaksi database selesai, jadi
// transaksi itu tidak pernah menunggu RPC.
func (s *Service) Create(ctx context.Context, p auth.Principal, key string, in Input) (d Draft, replayed bool, err error) {
	hash := in.hash()
	if old, err := s.replay(ctx, p, key, hash); old != nil || err != nil {
		if err != nil {
			return Draft{}, false, err
		}
		return *old, true, nil
	}
	if _, _, err := s.load(ctx, p, in); err != nil {
		return Draft{}, false, err
	}
	wallet, err := s.repo.storeWallet(ctx, p.StoreID)
	if err != nil {
		return Draft{}, false, err
	}
	if wallet == nil {
		return Draft{}, false, apperr.New(apperr.WalletNotLinked, "Toko belum menautkan wallet.")
	}
	reference, err := solana.NewReference()
	if err != nil {
		return Draft{}, false, err
	}
	id := uuid.New()
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.repo.lockLine(ctx, in.LineID); err != nil {
			return err
		}
		if existing, _, err := s.repo.idempotent(ctx, p.UserID, key); err != nil {
			return err
		} else if existing != nil {
			return errDuplicateKey
		}
		o, line, err := s.load(ctx, p, in)
		if err != nil {
			return err
		}
		unit, err := nextUnit(o, line)
		if err != nil {
			return err
		}
		running, err := s.repo.running(ctx, line.ID)
		if err != nil {
			return err
		}
		if running {
			return apperr.New(apperr.RefundInProgress, "Masih ada refund berjalan untuk barang ini.")
		}
		return s.repo.insert(ctx, newRefund{
			ID: id, LineID: line.ID, CreatedBy: p.UserID, Unit: unit, Source: *wallet, Reference: reference,
			Reason: in.Reason, Key: key, Hash: hash, ExpiresAt: s.now().Add(s.ttl),
		})
	})
	switch {
	case errors.Is(err, errDuplicateKey), database.IsUniqueViolation(err, "refunds_created_by_idempotency_key_key"):
		old, err := s.replay(ctx, p, key, hash)
		if err != nil {
			return Draft{}, false, err
		}
		return *old, true, nil
	case database.IsUniqueViolation(err, "refunds_one_running_per_line_idx"):
		return Draft{}, false, apperr.New(apperr.RefundInProgress, "Masih ada refund berjalan untuk barang ini.")
	case err != nil:
		return Draft{}, false, err
	}
	created, err := s.draft(ctx, p.StoreID, id)
	return created, false, err
}

// Get membaca satu refund.
func (s *Service) Get(ctx context.Context, storeID, id uuid.UUID) (order.Refund, error) {
	f, err := s.repo.get(ctx, storeID, id)
	if err != nil {
		return order.Refund{}, err
	}
	if f == nil {
		return order.Refund{}, notFound()
	}
	return *f, nil
}

// List membaca riwayat refund toko.
func (s *Service) List(ctx context.Context, storeID uuid.UUID, f Filter, p api.Page) ([]order.Refund, error) {
	return s.repo.list(ctx, storeID, f, p)
}

// Submit mencatat signature dari wallet toko. Hasilnya tetap divalidasi worker.
func (s *Service) Submit(ctx context.Context, storeID, id uuid.UUID, signature string) (order.Refund, error) {
	if _, err := s.Get(ctx, storeID, id); err != nil {
		return order.Refund{}, err
	}
	ok, err := s.repo.submit(ctx, id, signature)
	if database.IsUniqueViolation(err, "refunds_signature_key") {
		return order.Refund{}, apperr.Validation(apperr.FieldError{Field: "signature", Message: "Signature ini sudah dipakai refund lain."})
	}
	if err != nil {
		return order.Refund{}, err
	}
	if !ok {
		return order.Refund{}, apperr.New(apperr.RefundNotAwaitingSignature, "Refund ini tidak sedang menunggu tanda tangan.")
	}
	return s.Get(ctx, storeID, id)
}
