package refund

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

// Worker memvalidasi transaksi refund. RPC selalu dipanggil di luar transaksi database.
type Worker struct {
	repo   *Repository
	tx     database.TxRunner
	solana solana.Client
	batch  int
	grace  time.Duration
	now    func() time.Time
}

// NewWorker membuat worker refund. Masa tenggang menutup transaksi yang ditandatangani tepat sebelum
// batas waktu, supaya refund itu tidak dianggap kedaluwarsa lalu dibuat ulang.
func NewWorker(repo *Repository, tx database.TxRunner, client solana.Client, batch int, grace time.Duration, now func() time.Time) *Worker {
	return &Worker{repo: repo, tx: tx, solana: client, batch: batch, grace: grace, now: now}
}

// Tick memproses satu putaran. Kegagalan satu refund tidak menghentikan refund lain.
func (w *Worker) Tick(ctx context.Context) error {
	list, err := w.repo.candidates(ctx, w.batch)
	if err != nil {
		return err
	}
	for _, c := range list {
		if err := w.process(ctx, c); err != nil {
			slog.Warn("refund belum bisa diproses", "refund_id", c.ID, "error", err)
		}
	}
	return nil
}

func (w *Worker) process(ctx context.Context, c candidate) error {
	found, err := w.solana.FindSignatures(ctx, c.Reference)
	if err != nil {
		// Tanpa jawaban RPC, refund tidak boleh dianggap kedaluwarsa.
		return fmt.Errorf("cari transaksi: %w", err)
	}
	signatures := make([]string, 0, len(found)+1)
	failed := false
	for _, s := range found {
		if s.Failed {
			failed = true
			continue
		}
		signatures = append(signatures, s.Signature)
	}
	if c.Signature != nil && !slices.Contains(signatures, *c.Signature) {
		signatures = append(signatures, *c.Signature)
	}
	want := []solana.Transfer{{To: c.Destination, Lamports: c.Lamports}}
	unseen := false
	for _, signature := range signatures {
		tx, err := w.solana.GetTransaction(ctx, signature)
		switch {
		case errors.Is(err, solana.ErrNotFound):
			unseen = true
			continue
		case errors.Is(err, solana.ErrMalformed):
			failed = true
			continue
		case err != nil:
			return fmt.Errorf("baca transaksi: %w", err)
		}
		if tx.Failed || !solana.Matches(tx, c.Reference, c.Source, want) {
			failed = true
			continue
		}
		return w.complete(ctx, c, signature)
	}
	late := !w.now().Before(c.ExpiresAt.Add(w.grace))
	switch {
	case unseen && !late:
		return nil
	case failed:
		return w.repo.close(ctx, c.ID, StatusFailed)
	case late:
		return w.repo.close(ctx, c.ID, StatusExpired)
	}
	return nil
}

// complete menyimpan refund dan potongan penitip dalam satu transaksi database.
func (w *Worker) complete(ctx context.Context, c candidate, signature string) error {
	err := w.tx.InTx(ctx, func(ctx context.Context) error {
		deduction, ok, err := w.repo.lockRunning(ctx, c.ID)
		if err != nil || !ok {
			return err
		}
		if err := w.repo.complete(ctx, c.ID, signature, w.now()); err != nil {
			return err
		}
		if deduction == 0 {
			return nil
		}
		return w.repo.insertDeduction(ctx, c.ID)
	})
	if database.IsUniqueViolation(err, "refunds_signature_key") {
		return w.repo.close(ctx, c.ID, StatusFailed)
	}
	return err
}
