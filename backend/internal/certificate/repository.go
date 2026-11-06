// Package certificate mengirim NFT sertifikat ke wallet pembeli setelah pembayaran terkonfirmasi.
package certificate

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
)

// Repository membaca dan mengubah antrean sertifikat.
type Repository struct {
	db *database.DB
}

// NewRepository membuat repository sertifikat.
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

type candidate struct {
	ID        uuid.UUID
	Item      string
	Owner     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (r *Repository) candidates(ctx context.Context, limit int) ([]candidate, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `
		SELECT c.id, i.name, o.payer_wallet, c.created_at, c.updated_at
		FROM certificates c
		JOIN order_lines l ON l.id = c.order_line_id
		JOIN items i ON i.id = l.item_id
		JOIN orders o ON o.id = l.order_id
		WHERE c.status = 'pending'
		ORDER BY c.created_at, c.id
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("baca antrean sertifikat: %w", err)
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (candidate, error) {
		var c candidate
		err := row.Scan(&c.ID, &c.Item, &c.Owner, &c.CreatedAt, &c.UpdatedAt)
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("baca antrean sertifikat: %w", err)
	}
	return list, nil
}

func (r *Repository) markSent(ctx context.Context, id uuid.UUID, mint string, at time.Time) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`UPDATE certificates SET status = 'sent', mint_address = $2, updated_at = $3 WHERE id = $1 AND status = 'pending'`,
		id, mint, at)
	if err != nil {
		return fmt.Errorf("tandai sertifikat terkirim: %w", err)
	}
	return nil
}

func (r *Repository) markFailed(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`UPDATE certificates SET status = 'failed', updated_at = $2 WHERE id = $1 AND status = 'pending'`, id, at)
	if err != nil {
		return fmt.Errorf("tandai sertifikat gagal: %w", err)
	}
	return nil
}

// markAttempted mencatat waktu pengiriman supaya transaksi yang masih berjalan tidak dikirim ulang.
func (r *Repository) markAttempted(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.db.Q(ctx).Exec(ctx, `UPDATE certificates SET updated_at = $2 WHERE id = $1`, id, at)
	if err != nil {
		return fmt.Errorf("catat pengiriman sertifikat: %w", err)
	}
	return nil
}

type details struct {
	Item        string
	Store       string
	Signature   *string
	ConfirmedAt *time.Time
}

func (r *Repository) details(ctx context.Context, id uuid.UUID) (*details, error) {
	var d details
	err := r.db.Q(ctx).QueryRow(ctx, `
		SELECT i.name, s.name, o.signature, o.confirmed_at
		FROM certificates c
		JOIN order_lines l ON l.id = c.order_line_id
		JOIN items i ON i.id = l.item_id
		JOIN consignors n ON n.id = i.consignor_id
		JOIN stores s ON s.id = n.store_id
		JOIN orders o ON o.id = l.order_id
		WHERE c.id = $1`, id).Scan(&d.Item, &d.Store, &d.Signature, &d.ConfirmedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperr.New(apperr.NotFound, "Sertifikat tidak ditemukan.")
	}
	if err != nil {
		return nil, fmt.Errorf("baca sertifikat: %w", err)
	}
	return &d, nil
}
