package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
)

// Repository berisi query akun dan wallet.
type Repository interface {
	FindBySubject(ctx context.Context, subject string) (*Principal, error)
	FindUnboundByEmail(ctx context.Context, email string) (*Principal, error)
	BindSubject(ctx context.Context, userID uuid.UUID, subject string) error
	CreateNonce(ctx context.Context, n Nonce) error
	LockNonce(ctx context.Context, userID uuid.UUID, value string) (*Nonce, error)
	ConsumeNonce(ctx context.Context, id uuid.UUID, at time.Time) error
	WalletOwner(ctx context.Context, address string) (*WalletOwner, error)
	SetStoreWallet(ctx context.Context, storeID uuid.UUID, address string, at time.Time) error
	SetConsignorWallet(ctx context.Context, consignorID uuid.UUID, address string, at time.Time) error
}

// PgRepository adalah Repository di Postgres.
type PgRepository struct {
	db *database.DB
}

// NewRepository membuat repository Postgres.
func NewRepository(db *database.DB) *PgRepository {
	return &PgRepository{db: db}
}

const principalQuery = `
	SELECT u.id, u.store_id, s.name, u.role, u.email, u.name, c.id,
	       CASE WHEN u.role = 'store_owner' THEN s.wallet_address
	            WHEN u.role = 'consignor' AND c.wallet_verified_at IS NOT NULL THEN c.wallet_address END,
	       CASE WHEN u.role = 'store_owner' THEN s.wallet_linked_at
	            WHEN u.role = 'consignor' THEN c.wallet_verified_at END
	FROM users u
	JOIN stores s ON s.id = u.store_id
	LEFT JOIN consignors c ON c.user_id = u.id
`

func (r *PgRepository) principal(ctx context.Context, where string, arg any) (*Principal, error) {
	var p Principal
	err := r.db.Q(ctx).QueryRow(ctx, principalQuery+where, arg).Scan(
		&p.UserID, &p.StoreID, &p.StoreName, &p.Role, &p.Email, &p.Name,
		&p.ConsignorID, &p.WalletAddress, &p.WalletLinkedAt,
	)
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("baca akun: %w", err)
	}
	return &p, nil
}

// FindBySubject mencari akun yang sudah pernah login.
func (r *PgRepository) FindBySubject(ctx context.Context, subject string) (*Principal, error) {
	return r.principal(ctx, "WHERE u.auth_subject = $1", subject)
}

// FindUnboundByEmail mencari akun terdaftar yang belum terikat ke identitas Google. Subject lokal
// dianggap belum terikat, karena login lokal hanya ada di mesin pengembang.
func (r *PgRepository) FindUnboundByEmail(ctx context.Context, email string) (*Principal, error) {
	return r.principal(ctx, "WHERE u.email = $1 AND (u.auth_subject IS NULL OR u.auth_subject LIKE 'local:%')", email)
}

// BindSubject mengikat akun ke identitas Google.
func (r *PgRepository) BindSubject(ctx context.Context, userID uuid.UUID, subject string) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`UPDATE users SET auth_subject = $2, updated_at = now()
		 WHERE id = $1 AND (auth_subject IS NULL OR auth_subject LIKE 'local:%')`,
		userID, subject)
	if err != nil {
		return fmt.Errorf("ikat akun: %w", err)
	}
	return nil
}

// CreateNonce menyimpan nonce baru.
func (r *PgRepository) CreateNonce(ctx context.Context, n Nonce) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`INSERT INTO wallet_nonces (id, user_id, address, nonce, expires_at) VALUES ($1, $2, $3, $4, $5)`,
		n.ID, n.UserID, n.Address, n.Value, n.ExpiresAt)
	if err != nil {
		return fmt.Errorf("simpan nonce: %w", err)
	}
	return nil
}

// LockNonce membaca nonce dan menguncinya supaya tidak bisa dipakai bersamaan.
func (r *PgRepository) LockNonce(ctx context.Context, userID uuid.UUID, value string) (*Nonce, error) {
	var n Nonce
	err := r.db.Q(ctx).QueryRow(ctx, `
		SELECT id, user_id, address, nonce, expires_at, consumed_at
		FROM wallet_nonces WHERE user_id = $1 AND nonce = $2 FOR UPDATE`, userID, value).
		Scan(&n.ID, &n.UserID, &n.Address, &n.Value, &n.ExpiresAt, &n.ConsumedAt)
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("baca nonce: %w", err)
	}
	return &n, nil
}

// ConsumeNonce menandai nonce sudah terpakai.
func (r *PgRepository) ConsumeNonce(ctx context.Context, id uuid.UUID, at time.Time) error {
	_, err := r.db.Q(ctx).Exec(ctx, `UPDATE wallet_nonces SET consumed_at = $2 WHERE id = $1`, id, at)
	if err != nil {
		return fmt.Errorf("pakai nonce: %w", err)
	}
	return nil
}

// WalletOwner mencari toko atau penitip yang memakai alamat ini.
func (r *PgRepository) WalletOwner(ctx context.Context, address string) (*WalletOwner, error) {
	var o WalletOwner
	err := r.db.Q(ctx).QueryRow(ctx, `
		SELECT id, NULL::uuid FROM stores WHERE wallet_address = $1
		UNION ALL
		SELECT NULL::uuid, id FROM consignors WHERE wallet_address = $1
		LIMIT 1`, address).Scan(&o.StoreID, &o.ConsignorID)
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cari pemilik wallet: %w", err)
	}
	return &o, nil
}

// SetStoreWallet menautkan wallet ke toko.
func (r *PgRepository) SetStoreWallet(ctx context.Context, storeID uuid.UUID, address string, at time.Time) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`UPDATE stores SET wallet_address = $2, wallet_linked_at = $3, updated_at = now() WHERE id = $1`,
		storeID, address, at)
	if err != nil {
		return fmt.Errorf("simpan wallet toko: %w", err)
	}
	return nil
}

// SetConsignorWallet menautkan wallet ke penitip sebagai wallet terbukti.
func (r *PgRepository) SetConsignorWallet(ctx context.Context, consignorID uuid.UUID, address string, at time.Time) error {
	_, err := r.db.Q(ctx).Exec(ctx,
		`UPDATE consignors SET wallet_address = $2, wallet_verified_at = $3, updated_at = now() WHERE id = $1`,
		consignorID, address, at)
	if err != nil {
		return fmt.Errorf("simpan wallet penitip: %w", err)
	}
	return nil
}
