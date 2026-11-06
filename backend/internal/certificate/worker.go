package certificate

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

// Transaksi yang sudah terkirim biasanya terlihat dalam beberapa detik, jadi pengiriman ulang ditunda.
const retryAfter = 30 * time.Second

// Worker menerbitkan sertifikat yang masih antre. RPC dipanggil di luar transaksi database, dan aman
// berjalan di banyak instance karena alamat mint tiap sertifikat selalu sama.
type Worker struct {
	repo     *Repository
	solana   solana.Client
	signer   solana.Signer
	baseURL  string
	batch    int
	deadline time.Duration
	now      func() time.Time
}

// NewWorker membuat worker sertifikat. Tanpa signer, sertifikat hanya menunggu sampai batas waktu.
func NewWorker(repo *Repository, client solana.Client, signer solana.Signer, baseURL string, batch int,
	deadline time.Duration, now func() time.Time) *Worker {
	return &Worker{repo: repo, solana: client, signer: signer, baseURL: baseURL, batch: batch, deadline: deadline, now: now}
}

// Tick memproses satu putaran. Kegagalan satu sertifikat tidak menghentikan yang lain.
func (w *Worker) Tick(ctx context.Context) error {
	list, err := w.repo.candidates(ctx, w.batch)
	if err != nil {
		return err
	}
	for _, c := range list {
		if err := w.process(ctx, c); err != nil {
			slog.Warn("sertifikat belum terkirim", "certificate_id", c.ID, "error", err)
		}
	}
	return nil
}

func (w *Worker) process(ctx context.Context, c candidate) error {
	now := w.now()
	expired := now.Sub(c.CreatedAt) > w.deadline
	if w.signer == nil {
		if expired {
			return w.repo.markFailed(ctx, c.ID, now)
		}
		return nil
	}

	mint := solana.DeriveMint(w.signer, c.ID.String())
	address := mint.PublicKey().String()
	exists, err := w.solana.AccountExists(ctx, address)
	if err != nil {
		return err
	}
	if exists {
		return w.repo.markSent(ctx, c.ID, address, now)
	}
	if expired {
		return w.repo.markFailed(ctx, c.ID, now)
	}
	if c.UpdatedAt.After(c.CreatedAt) && now.Sub(c.UpdatedAt) < retryAfter {
		return nil
	}

	raw, err := w.build(ctx, c, mint)
	if err != nil {
		return err
	}
	if _, err := w.solana.SendTransaction(ctx, raw); err != nil {
		return err
	}
	return w.repo.markAttempted(ctx, c.ID, now)
}

func (w *Worker) build(ctx context.Context, c candidate, mint solana.Signer) ([]byte, error) {
	blockhash, err := w.solana.LatestBlockhash(ctx)
	if err != nil {
		return nil, err
	}
	rent, err := w.solana.RentExemptFor(ctx, solana.MintSize)
	if err != nil {
		return nil, err
	}
	return solana.BuildCertificate(solana.CertificateMint{
		Signer: w.signer, Mint: mint, Owner: c.Owner, Item: c.Item, Blockhash: blockhash, Rent: rent,
		URI: fmt.Sprintf("%s/api/v1/public/certificates/%s/metadata", w.baseURL, c.ID),
	})
}
