package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/item"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/cache"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

const (
	failureTransaction = "TRANSACTION_FAILED"
	failureMismatch    = "VALIDATION_MISMATCH"
)

// Worker mencari transaksi order dan mengubah statusnya. RPC selalu dipanggil di luar transaksi
// database, dan aman berjalan di banyak instance.
type Worker struct {
	repo   *Repository
	tx     database.TxRunner
	solana solana.Client
	cache  cache.Cache
	batch  int
	grace  time.Duration
	now    func() time.Time
}

// NewWorker membuat worker pembayaran.
func NewWorker(repo *Repository, tx database.TxRunner, client solana.Client, c cache.Cache, batch int,
	grace time.Duration, now func() time.Time) *Worker {
	return &Worker{repo: repo, tx: tx, solana: client, cache: c, batch: batch, grace: grace, now: now}
}

// Tick memproses satu putaran. Kegagalan satu order tidak menghentikan order lain.
func (w *Worker) Tick(ctx context.Context) error {
	list, err := w.repo.candidates(ctx, w.batch)
	if err != nil {
		return err
	}
	for _, c := range list {
		if err := w.process(ctx, c); err != nil {
			slog.Warn("order belum bisa diproses", "order_id", c.ID, "error", err)
		}
	}
	return nil
}

type rejection struct {
	code      string
	signature string
	payer     *string
}

func (w *Worker) process(ctx context.Context, c candidate) error {
	signatures, err := w.solana.FindSignatures(ctx, c.Reference)
	if err != nil {
		// Tanpa jawaban RPC, order tidak boleh dianggap kedaluwarsa.
		return fmt.Errorf("cari transaksi: %w", err)
	}
	want, err := w.repo.transfers(ctx, c.ID)
	if err != nil {
		return err
	}
	var rejected *rejection
	unseen := false
	for _, s := range signatures {
		if s.Failed {
			rejected = &rejection{code: failureTransaction, signature: s.Signature}
			continue
		}
		tx, err := w.solana.GetTransaction(ctx, s.Signature)
		switch {
		case errors.Is(err, solana.ErrNotFound):
			unseen = true
			continue
		case errors.Is(err, solana.ErrMalformed):
			rejected = &rejection{code: failureMismatch, signature: s.Signature}
			continue
		case err != nil:
			return fmt.Errorf("baca transaksi: %w", err)
		}
		if tx.Failed {
			rejected = &rejection{code: failureTransaction, signature: s.Signature, payer: &tx.FeePayer}
			continue
		}
		if !solana.Matches(tx, c.Reference, "", want) {
			rejected = &rejection{code: failureMismatch, signature: s.Signature, payer: &tx.FeePayer}
			continue
		}
		return w.confirm(ctx, c, tx)
	}
	late := !w.now().Before(c.ExpiresAt.Add(w.grace))
	switch {
	case unseen && !late:
		return nil
	case rejected != nil:
		return w.reject(ctx, c, rejected)
	case late:
		if err := w.repo.expire(ctx, c.ID); err != nil {
			return err
		}
		w.release(c.StoreID)
	}
	return nil
}

func (w *Worker) confirm(ctx context.Context, c candidate, tx *solana.Transaction) error {
	err := w.tx.InTx(ctx, func(ctx context.Context) error {
		ok, err := w.repo.lockPending(ctx, c.ID)
		if err != nil || !ok {
			return err
		}
		return w.repo.confirm(ctx, c.ID, tx.Signature, tx.FeePayer, w.now())
	})
	if database.IsUniqueViolation(err, "orders_signature_key") {
		// Satu transaksi hanya boleh melunasi satu order.
		return w.reject(ctx, c, &rejection{code: failureMismatch, signature: tx.Signature, payer: &tx.FeePayer})
	}
	if err != nil {
		return err
	}
	w.release(c.StoreID)
	return nil
}

func (w *Worker) reject(ctx context.Context, c candidate, r *rejection) error {
	if err := w.repo.fail(ctx, c.ID, r.code, &r.signature, r.payer); err != nil {
		return err
	}
	w.release(c.StoreID)
	return nil
}

// release membersihkan katalog, karena stok tertahan berubah.
func (w *Worker) release(storeID uuid.UUID) {
	w.cache.DeletePrefix(item.CatalogPrefix(storeID))
}
