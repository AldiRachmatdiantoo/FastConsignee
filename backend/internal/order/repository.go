package order

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/item"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
)

// Repository berisi query order.
type Repository struct {
	db *database.DB
}

// NewRepository membuat repository Postgres.
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

// StoreInfo adalah data toko yang dibutuhkan pembagian.
type StoreInfo struct {
	Name   string
	Wallet *string
}

func (r *Repository) store(ctx context.Context, id uuid.UUID) (StoreInfo, error) {
	var s StoreInfo
	err := r.db.Q(ctx).QueryRow(ctx, `SELECT name, wallet_address FROM stores WHERE id = $1`, id).Scan(&s.Name, &s.Wallet)
	if err != nil {
		return StoreInfo{}, fmt.Errorf("baca toko: %w", err)
	}
	return s, nil
}

type cartItem struct {
	ID            uuid.UUID
	Name          string
	PriceIDR      int64
	CommissionBps int32
	StockOnHand   int32
	Active        bool
	Consignor     item.ConsignorRef
	Wallet        *string
}

func (r *Repository) cartItems(ctx context.Context, storeID uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]cartItem, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `
		SELECT i.id, i.name, i.price_idr, i.commission_bps, i.stock_on_hand, i.active, c.id, c.name, c.wallet_address
		FROM items i
		JOIN consignors c ON c.id = i.consignor_id
		WHERE c.store_id = $1 AND i.id = ANY($2)`, storeID, ids)
	if err != nil {
		return nil, fmt.Errorf("baca keranjang: %w", err)
	}
	defer rows.Close()
	out := make(map[uuid.UUID]cartItem, len(ids))
	for rows.Next() {
		var c cartItem
		if err := rows.Scan(&c.ID, &c.Name, &c.PriceIDR, &c.CommissionBps, &c.StockOnHand, &c.Active,
			&c.Consignor.ID, &c.Consignor.Name, &c.Wallet); err != nil {
			return nil, fmt.Errorf("baca barang keranjang: %w", err)
		}
		out[c.ID] = c
	}
	return out, rows.Err()
}

func (r *Repository) reserved(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]int32, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `
		SELECT l.item_id, sum(l.quantity)::int
		FROM order_lines l
		JOIN orders o ON o.id = l.order_id
		WHERE o.status = 'pending' AND l.item_id = ANY($1)
		GROUP BY l.item_id`, ids)
	if err != nil {
		return nil, fmt.Errorf("baca stok tertahan: %w", err)
	}
	defer rows.Close()
	out := make(map[uuid.UUID]int32, len(ids))
	for rows.Next() {
		var id uuid.UUID
		var n int32
		if err := rows.Scan(&id, &n); err != nil {
			return nil, fmt.Errorf("baca stok tertahan: %w", err)
		}
		out[id] = n
	}
	return out, rows.Err()
}

// lockRows menjalankan SELECT ... FOR UPDATE dan membuang hasilnya.
func (r *Repository) lockRows(ctx context.Context, sql string, args ...any) error {
	rows, err := r.db.Q(ctx).Query(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("kunci baris: %w", err)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("kunci baris: %w", err)
	}
	return nil
}

// lockItems mengunci barang dalam urutan ID, supaya dua order tidak saling menunggu.
func (r *Repository) lockItems(ctx context.Context, ids []uuid.UUID) error {
	return r.lockRows(ctx, `SELECT id FROM items WHERE id = ANY($1) ORDER BY id FOR UPDATE`, ids)
}

const deductionsOf = `
	FROM deductions d
	JOIN refunds f ON f.id = d.refund_id
	JOIN order_lines l ON l.id = f.order_line_id
	JOIN items i ON i.id = l.item_id
	WHERE i.consignor_id = ANY($1)
	ORDER BY d.created_at, d.id
`

// lockDeductions mengunci potongan penitip, terlama lebih dulu, supaya tidak dilunasi dua order.
func (r *Repository) lockDeductions(ctx context.Context, consignorIDs []uuid.UUID) error {
	return r.lockRows(ctx, `SELECT d.id`+deductionsOf+` FOR UPDATE OF d`, consignorIDs)
}

// outstanding membaca sisa potongan. Pelunasan dari order expired atau failed tidak dihitung, jadi
// potongannya otomatis kembali terbuka. Harus statement terpisah dari kuncinya, karena snapshot
// statement yang menunggu kunci tidak melihat pelunasan yang baru di-commit.
func (r *Repository) outstanding(ctx context.Context, consignorIDs []uuid.UUID) ([]Outstanding, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `
		SELECT d.id, i.consignor_id, f.consignor_deduction_idr - COALESCE((
			SELECT sum(ds.amount_idr)
			FROM deduction_settlements ds
			JOIN order_splits sp ON sp.id = ds.order_split_id
			JOIN orders o ON o.id = sp.order_id
			WHERE ds.deduction_id = d.id AND o.status IN ('pending', 'confirmed')
		), 0)::bigint`+deductionsOf, consignorIDs)
	if err != nil {
		return nil, fmt.Errorf("baca potongan: %w", err)
	}
	defer rows.Close()
	var out []Outstanding
	for rows.Next() {
		var o Outstanding
		if err := rows.Scan(&o.DeductionID, &o.ConsignorID, &o.RemainingIDR); err != nil {
			return nil, fmt.Errorf("baca potongan: %w", err)
		}
		if o.RemainingIDR > 0 {
			out = append(out, o)
		}
	}
	return out, rows.Err()
}

// idempotent membaca order yang memakai kunci ini.
func (r *Repository) idempotent(ctx context.Context, userID uuid.UUID, key string) (*uuid.UUID, string, error) {
	var id uuid.UUID
	var hash string
	err := r.db.Q(ctx).QueryRow(ctx,
		`SELECT id, request_hash FROM orders WHERE created_by = $1 AND idempotency_key = $2`, userID, key).Scan(&id, &hash)
	if database.IsNoRows(err) {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", fmt.Errorf("baca idempotency: %w", err)
	}
	return &id, hash, nil
}

type newOrder struct {
	ID        uuid.UUID
	CreatedBy uuid.UUID
	Reference string
	Key       string
	Hash      string
	ExpiresAt time.Time
	Plan      Plan
}

// insert menyimpan order, baris, pembagian, dan pelunasan. Dipanggil di dalam transaksi.
func (r *Repository) insert(ctx context.Context, o newOrder) error {
	q := r.db.Q(ctx)
	if _, err := q.Exec(ctx, `
		INSERT INTO orders (id, created_by, reference, idr_per_sol, rate_quoted_at, idempotency_key, request_hash, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		o.ID, o.CreatedBy, o.Reference, o.Plan.Rate.IDRPerSOL, o.Plan.Rate.QuotedAt, o.Key, o.Hash, o.ExpiresAt); err != nil {
		return fmt.Errorf("simpan order: %w", err)
	}
	if err := r.insertLines(ctx, o); err != nil {
		return err
	}
	splits, err := r.insertSplits(ctx, o)
	if err != nil {
		return err
	}
	return r.insertSettlements(ctx, o.Plan.Settlements, splits)
}

func (r *Repository) insertLines(ctx context.Context, o newOrder) error {
	n := len(o.Plan.Lines)
	ids, items := make([]uuid.UUID, n), make([]uuid.UUID, n)
	quantities, bps := make([]int32, n), make([]int32, n)
	prices, discounts := make([]int64, n), make([]int64, n)
	bearers := make([]*string, n)
	for i, l := range o.Plan.Lines {
		// UUIDv7 terurut waktu, jadi urutan keranjang tersimpan tanpa kolom tambahan.
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("buat ID baris: %w", err)
		}
		ids[i], items[i], quantities[i], bps[i], prices[i] = id, l.ItemID, l.Quantity, l.CommissionBps, l.UnitPriceIDR
		if l.Discount != nil {
			discounts[i], bearers[i] = l.Discount.AmountIDR, &l.Discount.BorneBy
		}
	}
	_, err := r.db.Q(ctx).Exec(ctx, `
		INSERT INTO order_lines (id, order_id, item_id, quantity, unit_price_idr, commission_bps, discount_idr, discount_borne_by)
		SELECT t.id, $1, t.item_id, t.quantity, t.price, t.bps, t.discount, t.bearer
		FROM unnest($2::uuid[], $3::uuid[], $4::int[], $5::bigint[], $6::int[], $7::bigint[], $8::text[])
		     AS t(id, item_id, quantity, price, bps, discount, bearer)`,
		o.ID, ids, items, quantities, prices, bps, discounts, bearers)
	if err != nil {
		return fmt.Errorf("simpan baris order: %w", err)
	}
	return nil
}

func (r *Repository) insertSplits(ctx context.Context, o newOrder) (map[uuid.UUID]uuid.UUID, error) {
	bySplit := make(map[uuid.UUID]uuid.UUID, len(o.Plan.Split.Recipients))
	n := len(o.Plan.Split.Recipients)
	ids, consignors := make([]uuid.UUID, n), make([]*uuid.UUID, n)
	wallets, lamports := make([]string, n), make([]int64, n)
	for i, s := range o.Plan.Split.Recipients {
		ids[i], consignors[i], wallets[i], lamports[i] = uuid.New(), s.ConsignorID, s.Wallet, s.AmountLamports
		if s.ConsignorID != nil {
			bySplit[*s.ConsignorID] = ids[i]
		}
	}
	_, err := r.db.Q(ctx).Exec(ctx, `
		INSERT INTO order_splits (id, order_id, consignor_id, wallet_address, amount_lamports)
		SELECT t.id, $1, t.consignor_id, t.wallet, t.lamports
		FROM unnest($2::uuid[], $3::uuid[], $4::text[], $5::bigint[]) AS t(id, consignor_id, wallet, lamports)`,
		o.ID, ids, consignors, wallets, lamports)
	if err != nil {
		return nil, fmt.Errorf("simpan pembagian: %w", err)
	}
	return bySplit, nil
}

func (r *Repository) insertSettlements(ctx context.Context, settlements []Settlement, splits map[uuid.UUID]uuid.UUID) error {
	if len(settlements) == 0 {
		return nil
	}
	n := len(settlements)
	ids, deductions, splitIDs := make([]uuid.UUID, n), make([]uuid.UUID, n), make([]uuid.UUID, n)
	amounts := make([]int64, n)
	for i, s := range settlements {
		ids[i], deductions[i], splitIDs[i], amounts[i] = uuid.New(), s.DeductionID, splits[s.ConsignorID], s.AmountIDR
	}
	_, err := r.db.Q(ctx).Exec(ctx, `
		INSERT INTO deduction_settlements (id, deduction_id, order_split_id, amount_idr)
		SELECT * FROM unnest($1::uuid[], $2::uuid[], $3::uuid[], $4::bigint[])`, ids, deductions, splitIDs, amounts)
	if err != nil {
		return fmt.Errorf("simpan pelunasan: %w", err)
	}
	return nil
}

const selectOrder = `
	SELECT o.id, o.created_by, o.status, o.reference, o.idr_per_sol, o.rate_quoted_at, o.payer_wallet,
	       o.signature, o.failure_code, s.name, o.expires_at, o.confirmed_at, o.created_at, o.updated_at
	FROM orders o
	JOIN users u ON u.id = o.created_by
	JOIN stores s ON s.id = u.store_id
`

func scanOrder(row pgx.Row) (Order, error) {
	var o Order
	err := row.Scan(&o.ID, &o.CreatedBy, &o.Status, &o.Reference, &o.Rate.IDRPerSOL, &o.Rate.QuotedAt, &o.PayerWallet,
		&o.Signature, &o.FailureCode, &o.StoreName, &o.ExpiresAt, &o.ConfirmedAt, &o.CreatedAt, &o.UpdatedAt)
	return o, err
}

// head membaca baris order tanpa rinciannya.
func (r *Repository) head(ctx context.Context, sql string, args ...any) (*Order, error) {
	o, err := scanOrder(r.db.Q(ctx).QueryRow(ctx, sql, args...))
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("baca order: %w", err)
	}
	return &o, nil
}

// Get membaca order dalam batas scope.
func (r *Repository) Get(ctx context.Context, scope Scope, id uuid.UUID) (*Order, error) {
	sql := selectOrder + ` WHERE o.id = $1 AND u.store_id = $2`
	args := []any{id, scope.StoreID}
	if scope.CreatedBy != nil {
		sql += ` AND o.created_by = $3`
		args = append(args, *scope.CreatedBy)
	}
	o, err := r.head(ctx, sql, args...)
	if o == nil || err != nil {
		return nil, err
	}
	return o, r.fill(ctx, o)
}

// Find membaca order tanpa batas toko, untuk layar pembeli dan worker.
func (r *Repository) Find(ctx context.Context, id uuid.UUID) (*Order, error) {
	o, err := r.head(ctx, selectOrder+` WHERE o.id = $1`, id)
	if o == nil || err != nil {
		return nil, err
	}
	return o, r.fill(ctx, o)
}

func (r *Repository) fill(ctx context.Context, o *Order) error {
	ids := []uuid.UUID{o.ID}
	lines, err := r.lines(ctx, ids)
	if err != nil {
		return err
	}
	splits, err := r.splits(ctx, ids)
	if err != nil {
		return err
	}
	o.Lines = lines[o.ID]
	o.Totals, o.Split, err = assemble(o.Lines, splits[o.ID], o.StoreName)
	if err != nil {
		return err
	}
	o.Refunds, err = r.refunds(ctx, o.ID)
	return err
}

func (r *Repository) lines(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID][]Line, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `
		SELECT l.order_id, l.id, l.item_id, i.name, c.id, c.name, l.quantity, l.unit_price_idr, l.commission_bps,
		       l.discount_idr, l.discount_borne_by,
		       (SELECT count(*) FROM refunds f WHERE f.order_line_id = l.id AND f.status = 'completed')::int,
		       t.status, t.mint_address
		FROM order_lines l
		JOIN items i ON i.id = l.item_id
		JOIN consignors c ON c.id = i.consignor_id
		LEFT JOIN certificates t ON t.order_line_id = l.id
		WHERE l.order_id = ANY($1)
		ORDER BY l.order_id, l.id`, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("baca baris order: %w", err)
	}
	defer rows.Close()
	out := make(map[uuid.UUID][]Line, len(orderIDs))
	for rows.Next() {
		var (
			orderID     uuid.UUID
			l           Line
			discount    int64
			bearer      *string
			certificate *string
			mint        *string
		)
		if err := rows.Scan(&orderID, &l.ID, &l.ItemID, &l.ItemName, &l.Consignor.ID, &l.Consignor.Name, &l.Quantity,
			&l.UnitPriceIDR, &l.CommissionBps, &discount, &bearer, &l.RefundedQuantity, &certificate, &mint); err != nil {
			return nil, fmt.Errorf("baca baris order: %w", err)
		}
		borneBy := ""
		if bearer != nil {
			borneBy = *bearer
			l.Discount = &Discount{AmountIDR: discount, BorneBy: borneBy}
		}
		l.Amounts, err = ComputeLine(l.UnitPriceIDR, l.Quantity, l.CommissionBps, discount, borneBy)
		if err != nil {
			return nil, fmt.Errorf("hitung baris %s: %w", l.ID, err)
		}
		if certificate != nil {
			l.Certificate = &Certificate{Status: *certificate, MintAddress: mint}
		}
		out[orderID] = append(out[orderID], l)
	}
	return out, rows.Err()
}

// storedSplit adalah satu baris order_splits beserta potongan yang dilunasinya.
type storedSplit struct {
	ConsignorID  *uuid.UUID
	Wallet       string
	Lamports     int64
	DeductionIDR int64
}

func (r *Repository) splits(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID][]storedSplit, error) {
	rows, err := r.db.Q(ctx).Query(ctx, `
		SELECT sp.order_id, sp.consignor_id, sp.wallet_address, sp.amount_lamports, COALESCE(sum(ds.amount_idr), 0)::bigint
		FROM order_splits sp
		LEFT JOIN deduction_settlements ds ON ds.order_split_id = sp.id
		WHERE sp.order_id = ANY($1)
		GROUP BY sp.id`, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("baca pembagian: %w", err)
	}
	defer rows.Close()
	out := make(map[uuid.UUID][]storedSplit, len(orderIDs))
	for rows.Next() {
		var orderID uuid.UUID
		var s storedSplit
		if err := rows.Scan(&orderID, &s.ConsignorID, &s.Wallet, &s.Lamports, &s.DeductionIDR); err != nil {
			return nil, fmt.Errorf("baca pembagian: %w", err)
		}
		out[orderID] = append(out[orderID], s)
	}
	return out, rows.Err()
}

// assemble menyusun ulang penerima dari baris tersimpan, dengan urutan yang sama seperti saat dibuat.
func assemble(lines []Line, splits []storedSplit, storeName string) (Totals, Split, error) {
	var totals Totals
	split := Split{}
	var commission int64
	shares := make(map[uuid.UUID]int64, len(lines))
	var order []item.ConsignorRef
	for _, l := range lines {
		totals.SubtotalIDR += l.Amounts.Gross
		totals.DiscountIDR += l.Amounts.Discount
		split.TotalIDR += l.Amounts.LineTotal
		commission += l.Amounts.Commission
		if _, seen := shares[l.Consignor.ID]; !seen {
			order = append(order, l.Consignor)
		}
		shares[l.Consignor.ID] += l.Amounts.ConsignorShare
	}
	byConsignor := make(map[uuid.UUID]storedSplit, len(splits))
	var store storedSplit
	for _, s := range splits {
		split.TotalLamports += s.Lamports
		if s.ConsignorID == nil {
			store = s
			continue
		}
		byConsignor[*s.ConsignorID] = s
	}
	var recovered int64
	for _, c := range order {
		s, ok := byConsignor[c.ID]
		if !ok {
			return Totals{}, Split{}, fmt.Errorf("pembagian penitip %s tidak ada", c.ID)
		}
		id := c.ID
		split.Recipients = append(split.Recipients, Recipient{
			ConsignorID: &id, Name: c.Name, Wallet: s.Wallet, ShareIDR: shares[c.ID], DeductionIDR: s.DeductionIDR,
			AmountIDR: shares[c.ID] - s.DeductionIDR, AmountLamports: s.Lamports,
		})
		recovered += s.DeductionIDR
	}
	split.Recipients = append(split.Recipients, Recipient{
		Name: storeName, Wallet: store.Wallet, ShareIDR: commission, RecoveredIDR: recovered,
		AmountIDR: commission + recovered, AmountLamports: store.Lamports,
	})
	return totals, split, nil
}

// SelectRefund membaca refund beserta konteks ordernya.
const SelectRefund = `
	SELECT f.id, l.order_id, f.order_line_id, i.name, f.status, f.amount_idr, f.amount_lamports, f.source_wallet,
	       o.payer_wallet, f.reference, c.id, c.name, f.consignor_deduction_idr, f.reason, f.signature,
	       f.expires_at, f.created_at, f.completed_at
	FROM refunds f
	JOIN order_lines l ON l.id = f.order_line_id
	JOIN orders o ON o.id = l.order_id
	JOIN items i ON i.id = l.item_id
	JOIN consignors c ON c.id = i.consignor_id
`

// ScanRefund membaca satu baris SelectRefund.
func ScanRefund(row pgx.Row) (Refund, error) {
	var f Refund
	err := row.Scan(&f.ID, &f.OrderID, &f.OrderLineID, &f.ItemName, &f.Status, &f.AmountIDR, &f.AmountLamports, &f.SourceWallet,
		&f.DestinationWallet, &f.Reference, &f.Consignor.ID, &f.Consignor.Name, &f.DeductionIDR, &f.Reason, &f.Signature,
		&f.ExpiresAt, &f.CreatedAt, &f.CompletedAt)
	return f, err
}

func (r *Repository) refunds(ctx context.Context, orderID uuid.UUID) ([]Refund, error) {
	rows, err := r.db.Q(ctx).Query(ctx, SelectRefund+` WHERE l.order_id = $1 ORDER BY f.created_at, f.id`, orderID)
	if err != nil {
		return nil, fmt.Errorf("baca refund order: %w", err)
	}
	defer rows.Close()
	out := []Refund{}
	for rows.Next() {
		f, err := ScanRefund(rows)
		if err != nil {
			return nil, fmt.Errorf("baca refund order: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// list membaca riwayat order, terbaru lebih dulu.
func (r *Repository) list(ctx context.Context, scope Scope, f Filter, p api.Page) ([]Summary, error) {
	sql := `
		SELECT o.id, o.status, o.signature, o.created_at, o.confirmed_at
		FROM orders o
		JOIN users u ON u.id = o.created_by
		WHERE u.store_id = $1`
	args := []any{scope.StoreID}
	add := func(cond string, v any) {
		args = append(args, v)
		sql += fmt.Sprintf(cond, len(args))
	}
	if scope.CreatedBy != nil {
		add(` AND o.created_by = $%d`, *scope.CreatedBy)
	}
	if f.Status != nil {
		add(` AND o.status = $%d`, *f.Status)
	}
	if f.From != nil {
		add(` AND o.created_at >= $%d`, *f.From)
	}
	if f.To != nil {
		add(` AND o.created_at < $%d`, *f.To)
	}
	if p.Cursor != nil {
		args = append(args, p.Cursor.Time, p.Cursor.ID)
		sql += fmt.Sprintf(` AND (o.created_at, o.id) < ($%d, $%d)`, len(args)-1, len(args))
	}
	args = append(args, p.Limit+1)
	sql += fmt.Sprintf(` ORDER BY o.created_at DESC, o.id DESC LIMIT $%d`, len(args))

	rows, err := r.db.Q(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("daftar order: %w", err)
	}
	defer rows.Close()
	out := []Summary{}
	ids := []uuid.UUID{}
	for rows.Next() {
		var s Summary
		if err := rows.Scan(&s.ID, &s.Status, &s.Signature, &s.CreatedAt, &s.ConfirmedAt); err != nil {
			return nil, fmt.Errorf("baca order: %w", err)
		}
		out = append(out, s)
		ids = append(ids, s.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	return out, r.summarize(ctx, out, ids)
}

func (r *Repository) summarize(ctx context.Context, summaries []Summary, ids []uuid.UUID) error {
	lines, err := r.lines(ctx, ids)
	if err != nil {
		return err
	}
	splits, err := r.splits(ctx, ids)
	if err != nil {
		return err
	}
	for i := range summaries {
		s := &summaries[i]
		for _, l := range lines[s.ID] {
			s.TotalIDR += l.Amounts.LineTotal
			s.CommissionIDR += l.Amounts.Commission
			s.ItemCount += l.Quantity
		}
		for _, sp := range splits[s.ID] {
			s.TotalLamports += sp.Lamports
		}
	}
	return nil
}

// status membaca status ringan order.
func (r *Repository) status(ctx context.Context, scope Scope, id uuid.UUID) (*StatusView, error) {
	sql := `
		SELECT o.id, o.status, o.expires_at, o.signature, o.failure_code, o.updated_at
		FROM orders o
		JOIN users u ON u.id = o.created_by
		WHERE o.id = $1 AND u.store_id = $2`
	args := []any{id, scope.StoreID}
	if scope.CreatedBy != nil {
		sql += ` AND o.created_by = $3`
		args = append(args, *scope.CreatedBy)
	}
	var v StatusView
	err := r.db.Q(ctx).QueryRow(ctx, sql, args...).Scan(&v.ID, &v.Status, &v.ExpiresAt, &v.Signature, &v.FailureCode, &v.UpdatedAt)
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("baca status order: %w", err)
	}
	return &v, nil
}
