package refund

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/order"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
)

// Status refund.
const (
	StatusAwaitingSignature = "awaiting_signature"
	StatusConfirming        = "confirming"
	StatusCompleted         = "completed"
	StatusFailed            = "failed"
	StatusExpired           = "expired"
)

// Repository berisi query refund.
type Repository struct {
	db *database.DB
}

// NewRepository membuat repository Postgres.
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) get(ctx context.Context, storeID, id uuid.UUID) (*order.Refund, error) {
	f, err := order.ScanRefund(r.db.Q(ctx).QueryRow(ctx, order.SelectRefund+` WHERE f.id = $1 AND c.store_id = $2`, id, storeID))
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("baca refund: %w", err)
	}
	return &f, nil
}

// Filter adalah saringan riwayat refund.
type Filter struct {
	Status *string
	From   *time.Time
	To     *time.Time
}

func (r *Repository) list(ctx context.Context, storeID uuid.UUID, f Filter, p api.Page) ([]order.Refund, error) {
	sql := order.SelectRefund + ` WHERE c.store_id = $1`
	args := []any{storeID}
	add := func(cond string, v any) {
		args = append(args, v)
		sql += fmt.Sprintf(cond, len(args))
	}
	if f.Status != nil {
		add(` AND f.status = $%d`, *f.Status)
	}
	if f.From != nil {
		add(` AND f.created_at >= $%d`, *f.From)
	}
	if f.To != nil {
		add(` AND f.created_at < $%d`, *f.To)
	}
	if p.Cursor != nil {
		args = append(args, p.Cursor.Time, p.Cursor.ID)
		sql += fmt.Sprintf(` AND (f.created_at, f.id) < ($%d, $%d)`, len(args)-1, len(args))
	}
	args = append(args, p.Limit+1)
	sql += fmt.Sprintf(` ORDER BY f.created_at DESC, f.id DESC LIMIT $%d`, len(args))
	rows, err := r.db.Q(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("daftar refund: %w", err)
	}
	defer rows.Close()
	out := []order.Refund{}
	for rows.Next() {
		v, err := order.ScanRefund(rows)
		if err != nil {
			return nil, fmt.Errorf("baca refund: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) idempotent(ctx context.Context, userID uuid.UUID, key string) (*uuid.UUID, string, error) {
	var id uuid.UUID
	var hash string
	err := r.db.Q(ctx).QueryRow(ctx,
		`SELECT id, request_hash FROM refunds WHERE created_by = $1 AND idempotency_key = $2`, userID, key).Scan(&id, &hash)
	if database.IsNoRows(err) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("baca idempotency: %w", err)
	}
	return &id, hash, nil
}

// lockLine membuat refund pada satu baris berjalan bergantian.
func (r *Repository) lockLine(ctx context.Context, lineID uuid.UUID) error {
	var locked uuid.UUID
	err := r.db.Q(ctx).QueryRow(ctx, `SELECT id FROM order_lines WHERE id = $1 FOR UPDATE`, lineID).Scan(&locked)
	if err != nil {
		return fmt.Errorf("kunci baris order: %w", err)
	}
	return nil
}

func (r *Repository) running(ctx context.Context, lineID uuid.UUID) (bool, error) {
	var found bool
	err := r.db.Q(ctx).QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM refunds
			WHERE order_line_id = $1 AND status IN ('awaiting_signature', 'confirming')
		)`, lineID).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("cek refund berjalan: %w", err)
	}
	return found, nil
}

func (r *Repository) storeWallet(ctx context.Context, storeID uuid.UUID) (*string, error) {
	var wallet *string
	err := r.db.Q(ctx).QueryRow(ctx, `SELECT wallet_address FROM stores WHERE id = $1`, storeID).Scan(&wallet)
	if err != nil {
		return nil, fmt.Errorf("baca wallet toko: %w", err)
	}
	return wallet, nil
}

type newRefund struct {
	ID        uuid.UUID
	LineID    uuid.UUID
	CreatedBy uuid.UUID
	Unit      Unit
	Source    string
	Reference string
	Reason    *string
	Key       string
	Hash      string
	ExpiresAt time.Time
}

func (r *Repository) insert(ctx context.Context, n newRefund) error {
	_, err := r.db.Q(ctx).Exec(ctx, `
		INSERT INTO refunds (id, order_line_id, created_by, amount_idr, amount_lamports, consignor_deduction_idr,
		                     source_wallet, reference, reason, idempotency_key, request_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		n.ID, n.LineID, n.CreatedBy, n.Unit.AmountIDR, n.Unit.AmountLamports, n.Unit.DeductionIDR,
		n.Source, n.Reference, n.Reason, n.Key, n.Hash, n.ExpiresAt)
	if err != nil {
		return fmt.Errorf("simpan refund: %w", err)
	}
	return nil
}

func (r *Repository) submit(ctx context.Context, id uuid.UUID, signature string) (bool, error) {
	tag, err := r.db.Q(ctx).Exec(ctx, `
		UPDATE refunds SET status = 'confirming', signature = $2, updated_at = now()
		WHERE id = $1 AND status = 'awaiting_signature'`, id, signature)
	if err != nil {
		return false, fmt.Errorf("simpan signature: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// candidate adalah refund yang masih berjalan.
type candidate struct {
	ID          uuid.UUID
	Reference   string
	Signature   *string
	Source      string
	Destination string
	Lamports    uint64
	ExpiresAt   time.Time
}

func (r *Repository) candidates(ctx context.Context, limit int) ([]candidate, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `
		SELECT f.id, f.reference, f.signature, f.source_wallet, o.payer_wallet, f.amount_lamports, f.expires_at
		FROM refunds f
		JOIN order_lines l ON l.id = f.order_line_id
		JOIN orders o ON o.id = l.order_id
		WHERE f.status IN ('awaiting_signature', 'confirming')
		ORDER BY f.expires_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("baca refund berjalan: %w", err)
	}
	defer rows.Close()
	var out []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.ID, &c.Reference, &c.Signature, &c.Source, &c.Destination, &c.Lamports, &c.ExpiresAt); err != nil {
			return nil, fmt.Errorf("baca refund berjalan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// lockRunning mengunci refund yang masih berjalan. Instance lain yang memegangnya dilewati.
func (r *Repository) lockRunning(ctx context.Context, id uuid.UUID) (int64, bool, error) {
	var deduction int64
	err := r.db.Q(ctx).QueryRow(ctx, `
		SELECT consignor_deduction_idr FROM refunds
		WHERE id = $1 AND status IN ('awaiting_signature', 'confirming')
		FOR UPDATE SKIP LOCKED`, id).Scan(&deduction)
	if database.IsNoRows(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("kunci refund: %w", err)
	}
	return deduction, true, nil
}

func (r *Repository) complete(ctx context.Context, id uuid.UUID, signature string, at time.Time) error {
	_, err := r.db.Q(ctx).Exec(ctx, `
		UPDATE refunds SET status = 'completed', signature = $2, completed_at = $3, updated_at = now()
		WHERE id = $1`, id, signature, at)
	if err != nil {
		return fmt.Errorf("selesaikan refund: %w", err)
	}
	return nil
}

func (r *Repository) insertDeduction(ctx context.Context, refundID uuid.UUID) error {
	_, err := r.db.Q(ctx).Exec(ctx, `INSERT INTO deductions (id, refund_id) VALUES ($1, $2)`, uuid.New(), refundID)
	if err != nil {
		return fmt.Errorf("simpan potongan: %w", err)
	}
	return nil
}

// close menutup refund tanpa mengubah saldo atau potongan.
func (r *Repository) close(ctx context.Context, id uuid.UUID, status string) error {
	_, err := r.db.Q(ctx).Exec(ctx, `
		UPDATE refunds SET status = $2, updated_at = now()
		WHERE id = $1 AND status IN ('awaiting_signature', 'confirming')`, id, status)
	if err != nil {
		return fmt.Errorf("tutup refund: %w", err)
	}
	return nil
}
