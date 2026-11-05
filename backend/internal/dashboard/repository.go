// Package dashboard membaca ringkasan untuk penitip dan pemilik toko.
package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/order"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
)

// Repository berisi query dasbor. Semua nilai uang dihitung ulang lewat order.ComputeLine.
type Repository struct {
	db *database.DB
}

// NewRepository membuat repository Postgres.
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// Period adalah periode laporan. `From` inklusif dan `To` eksklusif.
type Period struct {
	From *time.Time
	To   *time.Time
}

// Sale adalah satu barang yang terjual.
type Sale struct {
	OrderID          uuid.UUID
	LineID           uuid.UUID
	ItemName         string
	Quantity         int32
	UnitPriceIDR     int64
	Amounts          order.Amounts
	RefundedQuantity int32
	DeductionIDR     int64
	TransferLamports int64
	SoldAt           time.Time
	Signature        string
}

func (r *Repository) sales(ctx context.Context, consignorID uuid.UUID, period Period, p api.Page) ([]Sale, error) {
	sql := `
		SELECT l.order_id, l.id, i.name, l.quantity, l.unit_price_idr, l.commission_bps, l.discount_idr, l.discount_borne_by,
		       (SELECT count(*) FROM refunds f WHERE f.order_line_id = l.id AND f.status = 'completed')::int,
		       COALESCE((SELECT sum(ds.amount_idr) FROM deduction_settlements ds WHERE ds.order_split_id = sp.id), 0)::bigint,
		       sp.amount_lamports, o.confirmed_at, o.signature
		FROM order_lines l
		JOIN items i ON i.id = l.item_id
		JOIN orders o ON o.id = l.order_id
		JOIN order_splits sp ON sp.order_id = o.id AND sp.consignor_id = i.consignor_id
		WHERE i.consignor_id = $1 AND o.status = 'confirmed'`
	args := []any{consignorID}
	add := func(cond string, v any) {
		args = append(args, v)
		sql += fmt.Sprintf(cond, len(args))
	}
	if period.From != nil {
		add(` AND o.confirmed_at >= $%d`, *period.From)
	}
	if period.To != nil {
		add(` AND o.confirmed_at < $%d`, *period.To)
	}
	if p.Cursor != nil {
		args = append(args, p.Cursor.Time, p.Cursor.ID)
		sql += fmt.Sprintf(` AND (o.confirmed_at, l.id) < ($%d, $%d)`, len(args)-1, len(args))
	}
	args = append(args, p.Limit+1)
	sql += fmt.Sprintf(` ORDER BY o.confirmed_at DESC, l.id DESC LIMIT $%d`, len(args))

	rows, err := r.db.Q(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("daftar penjualan: %w", err)
	}
	defer rows.Close()
	out := []Sale{}
	for rows.Next() {
		var (
			s        Sale
			bps      int32
			discount int64
			bearer   *string
		)
		if err := rows.Scan(&s.OrderID, &s.LineID, &s.ItemName, &s.Quantity, &s.UnitPriceIDR, &bps, &discount, &bearer,
			&s.RefundedQuantity, &s.DeductionIDR, &s.TransferLamports, &s.SoldAt, &s.Signature); err != nil {
			return nil, fmt.Errorf("baca penjualan: %w", err)
		}
		s.Amounts, err = computeLine(s.UnitPriceIDR, s.Quantity, bps, discount, bearer)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func computeLine(price int64, quantity, bps int32, discount int64, bearer *string) (order.Amounts, error) {
	borneBy := ""
	if bearer != nil {
		borneBy = *bearer
	}
	a, err := order.ComputeLine(price, quantity, bps, discount, borneBy)
	if err != nil {
		return order.Amounts{}, fmt.Errorf("hitung baris: %w", err)
	}
	return a, nil
}

// Deduction adalah potongan refund satu penitip.
type Deduction struct {
	ID           uuid.UUID
	RefundID     uuid.UUID
	OrderID      uuid.UUID
	ItemName     string
	AmountIDR    int64
	RemainingIDR int64
	CreatedAt    time.Time
	LastSettled  *time.Time
}

// Status adalah status potongan, dihitung dari sisanya.
func (d Deduction) Status() string {
	if d.RemainingIDR > 0 {
		return statusOutstanding
	}
	return statusSettled
}

const (
	statusOutstanding = "outstanding"
	statusSettled     = "settled"
)

// Pelunasan pada order expired atau failed tidak dihitung, sama seperti saat order dibuat.
const deductionRows = `
	SELECT d.id, f.id AS refund_id, l.order_id, i.name, f.consignor_deduction_idr,
	       f.consignor_deduction_idr - COALESCE(s.settled, 0) AS remaining, d.created_at, s.last_settled
	FROM deductions d
	JOIN refunds f ON f.id = d.refund_id
	JOIN order_lines l ON l.id = f.order_line_id
	JOIN items i ON i.id = l.item_id
	LEFT JOIN LATERAL (
		SELECT sum(ds.amount_idr)::bigint AS settled, max(ds.created_at) AS last_settled
		FROM deduction_settlements ds
		JOIN order_splits sp ON sp.id = ds.order_split_id
		JOIN orders o ON o.id = sp.order_id
		WHERE ds.deduction_id = d.id AND o.status IN ('pending', 'confirmed')
	) s ON true
	WHERE i.consignor_id = $1
`

func (r *Repository) deductions(ctx context.Context, consignorID uuid.UUID, status *string, p api.Page) ([]Deduction, error) {
	sql := `SELECT * FROM (` + deductionRows + `) t WHERE true`
	args := []any{consignorID}
	if status != nil {
		if *status == statusOutstanding {
			sql += ` AND t.remaining > 0`
		} else {
			sql += ` AND t.remaining = 0`
		}
	}
	if p.Cursor != nil {
		args = append(args, p.Cursor.Time, p.Cursor.ID)
		sql += fmt.Sprintf(` AND (t.created_at, t.id) < ($%d, $%d)`, len(args)-1, len(args))
	}
	args = append(args, p.Limit+1)
	sql += fmt.Sprintf(` ORDER BY t.created_at DESC, t.id DESC LIMIT $%d`, len(args))

	rows, err := r.db.Q(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("daftar potongan: %w", err)
	}
	defer rows.Close()
	out := []Deduction{}
	for rows.Next() {
		var d Deduction
		if err := rows.Scan(&d.ID, &d.RefundID, &d.OrderID, &d.ItemName, &d.AmountIDR, &d.RemainingIDR, &d.CreatedAt, &d.LastSettled); err != nil {
			return nil, fmt.Errorf("baca potongan: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) outstandingTotal(ctx context.Context, consignorID uuid.UUID) (int64, error) {
	var total int64
	err := r.db.Q(ctx).QueryRow(ctx,
		`SELECT COALESCE(sum(t.remaining), 0)::bigint FROM (`+deductionRows+`) t`, consignorID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("total potongan: %w", err)
	}
	return total, nil
}

// Summary adalah ringkasan penjualan toko.
type Summary struct {
	From             *time.Time
	To               *time.Time
	TotalSalesIDR    int64
	TotalCommission  int64
	TransactionCount int32
	GeneratedAt      time.Time
}

func (r *Repository) summary(ctx context.Context, storeID uuid.UUID, period Period) (Summary, error) {
	sql := `
		SELECT l.order_id, l.quantity, l.unit_price_idr, l.commission_bps, l.discount_idr, l.discount_borne_by
		FROM order_lines l
		JOIN orders o ON o.id = l.order_id
		JOIN users u ON u.id = o.created_by
		WHERE u.store_id = $1 AND o.status = 'confirmed'`
	args := []any{storeID}
	if period.From != nil {
		args = append(args, *period.From)
		sql += fmt.Sprintf(` AND o.confirmed_at >= $%d`, len(args))
	}
	if period.To != nil {
		args = append(args, *period.To)
		sql += fmt.Sprintf(` AND o.confirmed_at < $%d`, len(args))
	}
	rows, err := r.db.Q(ctx).Query(ctx, sql, args...)
	if err != nil {
		return Summary{}, fmt.Errorf("ringkasan toko: %w", err)
	}
	defer rows.Close()
	s := Summary{From: period.From, To: period.To}
	orders := map[uuid.UUID]struct{}{}
	for rows.Next() {
		var (
			id       uuid.UUID
			qty, bps int32
			price    int64
			discount int64
			bearer   *string
		)
		if err := rows.Scan(&id, &qty, &price, &bps, &discount, &bearer); err != nil {
			return Summary{}, fmt.Errorf("baca ringkasan: %w", err)
		}
		a, err := computeLine(price, qty, bps, discount, bearer)
		if err != nil {
			return Summary{}, err
		}
		s.TotalSalesIDR += a.LineTotal
		s.TotalCommission += a.Commission
		orders[id] = struct{}{}
	}
	s.TransactionCount = int32(len(orders))
	return s, rows.Err()
}
