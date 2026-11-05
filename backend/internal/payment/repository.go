// Package payment mengurus layar bayar, Solana Pay, dan konfirmasi pembayaran.
package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

// Repository berisi query pembayaran.
type Repository struct {
	db *database.DB
}

// NewRepository membuat repository Postgres.
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

type metadata struct {
	Label string
	Icon  *string
}

func (r *Repository) metadata(ctx context.Context, orderID uuid.UUID) (*metadata, error) {
	var m metadata
	err := r.db.Q(ctx).QueryRow(ctx, `
		SELECT s.name, s.icon_url
		FROM orders o
		JOIN users u ON u.id = o.created_by
		JOIN stores s ON s.id = u.store_id
		WHERE o.id = $1`, orderID).Scan(&m.Label, &m.Icon)
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("baca toko order: %w", err)
	}
	return &m, nil
}

// candidate adalah order yang menunggu pembayaran.
type candidate struct {
	ID        uuid.UUID
	StoreID   uuid.UUID
	Reference string
	ExpiresAt time.Time
}

func (r *Repository) candidates(ctx context.Context, limit int) ([]candidate, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `
		SELECT o.id, u.store_id, o.reference, o.expires_at
		FROM orders o
		JOIN users u ON u.id = o.created_by
		WHERE o.status = 'pending'
		ORDER BY o.expires_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("baca order menunggu: %w", err)
	}
	defer rows.Close()
	var out []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.ID, &c.StoreID, &c.Reference, &c.ExpiresAt); err != nil {
			return nil, fmt.Errorf("baca order menunggu: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) transfers(ctx context.Context, orderID uuid.UUID) ([]solana.Transfer, error) {
	rows, err := r.db.Q(ctx).Query(ctx,
		`SELECT wallet_address, amount_lamports FROM order_splits WHERE order_id = $1 AND amount_lamports > 0`, orderID)
	if err != nil {
		return nil, fmt.Errorf("baca pembagian: %w", err)
	}
	defer rows.Close()
	var out []solana.Transfer
	for rows.Next() {
		var t solana.Transfer
		if err := rows.Scan(&t.To, &t.Lamports); err != nil {
			return nil, fmt.Errorf("baca pembagian: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// lockPending mengunci order yang masih menunggu. Instance lain yang sedang memproses order yang
// sama dilewati, bukan ditunggu.
func (r *Repository) lockPending(ctx context.Context, id uuid.UUID) (bool, error) {
	var locked uuid.UUID
	err := r.db.Q(ctx).QueryRow(ctx,
		`SELECT id FROM orders WHERE id = $1 AND status = 'pending' FOR UPDATE SKIP LOCKED`, id).Scan(&locked)
	if database.IsNoRows(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("kunci order: %w", err)
	}
	return true, nil
}

// confirm menandai order lunas, memotong stok, dan membuat antrean sertifikat. Dipanggil di dalam
// transaksi setelah lockPending.
func (r *Repository) confirm(ctx context.Context, id uuid.UUID, signature, payer string, at time.Time) error {
	q := r.db.Q(ctx)
	rows, err := q.Query(ctx, `
		SELECT i.id
		FROM items i
		JOIN order_lines l ON l.item_id = i.id
		WHERE l.order_id = $1
		ORDER BY i.id
		FOR UPDATE OF i`, id)
	if err != nil {
		return fmt.Errorf("kunci barang: %w", err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("kunci barang: %w", err)
	}
	if _, err := q.Exec(ctx, `
		UPDATE orders
		SET status = 'confirmed', signature = $2, payer_wallet = $3, confirmed_at = $4, updated_at = now()
		WHERE id = $1`, id, signature, payer, at); err != nil {
		return fmt.Errorf("konfirmasi order: %w", err)
	}
	if _, err := q.Exec(ctx, `
		UPDATE items i
		SET stock_on_hand = i.stock_on_hand - l.quantity, updated_at = now()
		FROM order_lines l
		WHERE l.order_id = $1 AND l.item_id = i.id`, id); err != nil {
		return fmt.Errorf("potong stok: %w", err)
	}
	if _, err := q.Exec(ctx, `
		INSERT INTO certificates (id, order_line_id)
		SELECT gen_random_uuid(), l.id
		FROM order_lines l
		JOIN items i ON i.id = l.item_id
		WHERE l.order_id = $1 AND i.certificate_enabled`, id); err != nil {
		return fmt.Errorf("antre sertifikat: %w", err)
	}
	return nil
}

// fail menutup order. Signature dan pembayar disimpan supaya pemilik toko bisa menelusuri dana
// yang telanjur masuk. Signature yang sudah milik order lain tidak disalin.
func (r *Repository) fail(ctx context.Context, id uuid.UUID, code string, signature, payer *string) error {
	_, err := r.db.Q(ctx).Exec(ctx, `
		UPDATE orders
		SET status = 'failed', failure_code = $2, payer_wallet = $4, updated_at = now(),
		    signature = CASE WHEN EXISTS (SELECT 1 FROM orders x WHERE x.signature = $3) THEN NULL ELSE $3 END
		WHERE id = $1 AND status = 'pending'`, id, code, signature, payer)
	if err != nil {
		return fmt.Errorf("gagalkan order: %w", err)
	}
	return nil
}

func (r *Repository) expire(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`UPDATE orders SET status = 'expired', updated_at = now() WHERE id = $1 AND status = 'pending'`, id)
	if err != nil {
		return fmt.Errorf("akhiri order: %w", err)
	}
	return nil
}
