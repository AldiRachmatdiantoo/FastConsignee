package item

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
)

// Repository berisi query barang. Semua query dibatasi per toko.
type Repository interface {
	List(ctx context.Context, storeID uuid.UUID, f Filter, p api.Page) ([]Item, error)
	Get(ctx context.Context, storeID, id uuid.UUID) (*Item, error)
	Lock(ctx context.Context, storeID, id uuid.UUID) (*Item, error)
	ConsignorInStore(ctx context.Context, storeID, consignorID uuid.UUID) (bool, error)
	Insert(ctx context.Context, id uuid.UUID, c Create) error
	Save(ctx context.Context, i Item) error
}

// PgRepository adalah Repository di Postgres.
type PgRepository struct {
	db *database.DB
}

// NewRepository membuat repository Postgres.
func NewRepository(db *database.DB) *PgRepository {
	return &PgRepository{db: db}
}

// Stok tertahan dihitung dari order `pending`, jadi tidak ada angka reservasi yang tersimpan.
const selectItem = `
	WITH reserved AS (
		SELECT l.item_id, sum(l.quantity)::int AS quantity
		FROM orders o
		JOIN order_lines l ON l.order_id = o.id
		WHERE o.status = 'pending'
		GROUP BY l.item_id
	)
	SELECT i.id, i.name, i.price_idr, i.commission_bps, c.id, c.name,
	       i.stock_on_hand, COALESCE(r.quantity, 0), i.active, i.certificate_enabled,
	       i.created_at, i.updated_at
	FROM items i
	JOIN consignors c ON c.id = i.consignor_id
	LEFT JOIN reserved r ON r.item_id = i.id
`

func scan(row pgx.Row) (Item, error) {
	var i Item
	err := row.Scan(&i.ID, &i.Name, &i.PriceIDR, &i.CommissionBps, &i.Consignor.ID, &i.Consignor.Name,
		&i.StockOnHand, &i.StockReserved, &i.Active, &i.CertificateEnabled, &i.CreatedAt, &i.UpdatedAt)
	return i, err
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// List membaca katalog, terbaru lebih dulu.
func (r *PgRepository) List(ctx context.Context, storeID uuid.UUID, f Filter, p api.Page) ([]Item, error) {
	sql := selectItem + ` WHERE c.store_id = $1`
	args := []any{storeID}
	add := func(cond string, v any) {
		args = append(args, v)
		sql += fmt.Sprintf(cond, len(args))
	}
	if f.Query != nil {
		add(` AND i.name ILIKE '%%' || $%d || '%%'`, escapeLike(*f.Query))
	}
	if f.ConsignorID != nil {
		add(` AND i.consignor_id = $%d`, *f.ConsignorID)
	}
	if f.Active != nil {
		add(` AND i.active = $%d`, *f.Active)
	}
	if p.Cursor != nil {
		args = append(args, p.Cursor.Time, p.Cursor.ID)
		sql += fmt.Sprintf(` AND (i.created_at, i.id) < ($%d, $%d)`, len(args)-1, len(args))
	}
	args = append(args, p.Limit+1)
	sql += fmt.Sprintf(` ORDER BY i.created_at DESC, i.id DESC LIMIT $%d`, len(args))

	rows, err := r.db.Q(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("daftar barang: %w", err)
	}
	defer rows.Close()
	out := []Item{}
	for rows.Next() {
		i, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("baca barang: %w", err)
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (r *PgRepository) one(ctx context.Context, suffix string, storeID, id uuid.UUID) (*Item, error) {
	i, err := scan(r.db.Q(ctx).QueryRow(ctx, selectItem+` WHERE c.store_id = $1 AND i.id = $2`+suffix, storeID, id))
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("baca barang: %w", err)
	}
	return &i, nil
}

// Get membaca satu barang.
func (r *PgRepository) Get(ctx context.Context, storeID, id uuid.UUID) (*Item, error) {
	return r.one(ctx, "", storeID, id)
}

// Lock membaca satu barang dan mengunci barisnya sampai transaksi selesai.
func (r *PgRepository) Lock(ctx context.Context, storeID, id uuid.UUID) (*Item, error) {
	return r.one(ctx, ` FOR UPDATE OF i`, storeID, id)
}

// ConsignorInStore memeriksa penitip berada di toko ini.
func (r *PgRepository) ConsignorInStore(ctx context.Context, storeID, consignorID uuid.UUID) (bool, error) {
	var ok bool
	err := r.db.Q(ctx).QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM consignors WHERE id = $1 AND store_id = $2)`, consignorID, storeID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("cek penitip: %w", err)
	}
	return ok, nil
}

// Insert menyimpan barang baru.
func (r *PgRepository) Insert(ctx context.Context, id uuid.UUID, c Create) error {
	_, err := r.db.Q(ctx).Exec(ctx, `
		INSERT INTO items (id, consignor_id, name, price_idr, commission_bps, stock_on_hand, certificate_enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, c.ConsignorID, c.Name, c.PriceIDR, c.CommissionBps, c.StockOnHand, c.CertificateEnabled)
	if err != nil {
		return fmt.Errorf("simpan barang: %w", err)
	}
	return nil
}

// Save menyimpan perubahan barang.
func (r *PgRepository) Save(ctx context.Context, i Item) error {
	_, err := r.db.Q(ctx).Exec(ctx, `
		UPDATE items
		SET name = $2, price_idr = $3, commission_bps = $4, stock_on_hand = $5,
		    active = $6, certificate_enabled = $7, updated_at = now()
		WHERE id = $1`,
		i.ID, i.Name, i.PriceIDR, i.CommissionBps, i.StockOnHand, i.Active, i.CertificateEnabled)
	if err != nil {
		return fmt.Errorf("ubah barang: %w", err)
	}
	return nil
}
