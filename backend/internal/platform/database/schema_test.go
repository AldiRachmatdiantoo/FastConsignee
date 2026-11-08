package database_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

func TestEveryForeignKeyHasAnIndex(t *testing.T) {
	db := testkit.NewDB(t)
	rows, err := db.Pool.Query(context.Background(), `
		SELECT c.conrelid::regclass::text, c.conname
		FROM pg_constraint c
		WHERE c.contype = 'f'
		  AND NOT EXISTS (
		    SELECT 1 FROM pg_index i
		    WHERE i.indrelid = c.conrelid
		      AND i.indisvalid
		      AND i.indpred IS NULL
		      AND (string_to_array(i.indkey::text, ' ')::int2[])[1:cardinality(c.conkey)] @> c.conkey
		  )`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var table, name string
		if err := rows.Scan(&table, &name); err != nil {
			t.Fatal(err)
		}
		t.Errorf("foreign key %s di %s tidak punya indeks", name, table)
	}
}

// RLS tanpa policy menutup tabel dari peran anon Supabase. API Go tetap bisa membaca karena memakai pemilik tabel.
func TestEveryTableHasRowLevelSecurity(t *testing.T) {
	db := testkit.NewDB(t)
	rows, err := db.Pool.Query(context.Background(), `
		SELECT relname FROM pg_class
		WHERE relnamespace = 'public'::regnamespace AND relkind = 'r' AND NOT relrowsecurity`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatal(err)
		}
		t.Errorf("tabel %s belum memakai RLS", table)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaHasTwelveTables(t *testing.T) {
	db := testkit.NewDB(t)
	var n int
	err := db.Pool.QueryRow(context.Background(), `
		SELECT count(*) FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name <> 'goose_db_version'`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 12 {
		t.Fatalf("jumlah tabel %d, seharusnya 12", n)
	}
}

func TestConstraintsRejectBadData(t *testing.T) {
	db := testkit.NewDB(t)
	ctx := context.Background()
	store, user, other := uuid.New(), uuid.New(), uuid.New()
	mustExec(t, db, `INSERT INTO stores (id, name) VALUES ($1, 'Toko'), ($2, 'Lain')`, store, other)
	mustExec(t, db, `INSERT INTO users (id, store_id, role, email, name) VALUES ($1, $2, 'consignor', 'a@b.co', 'A')`, user, store)

	t.Run("peran tidak dikenal", func(t *testing.T) {
		_, err := db.Pool.Exec(ctx, `INSERT INTO users (id, store_id, role, email, name) VALUES ($1, $2, 'admin', 'x@b.co', 'X')`, uuid.New(), store)
		if !database.IsCheckViolation(err) {
			t.Fatalf("seharusnya melanggar CHECK: %v", err)
		}
	})
	t.Run("email huruf besar", func(t *testing.T) {
		_, err := db.Pool.Exec(ctx, `INSERT INTO users (id, store_id, role, email, name) VALUES ($1, $2, 'cashier', 'X@b.co', 'X')`, uuid.New(), store)
		if !database.IsCheckViolation(err) {
			t.Fatalf("seharusnya melanggar CHECK: %v", err)
		}
	})
	t.Run("toko penitip beda dengan toko akun", func(t *testing.T) {
		_, err := db.Pool.Exec(ctx, `INSERT INTO consignors (id, store_id, user_id, name) VALUES ($1, $2, $3, 'P')`, uuid.New(), other, user)
		if !database.IsForeignKeyViolation(err) {
			t.Fatalf("seharusnya melanggar foreign key: %v", err)
		}
	})
	t.Run("toko tidak bisa dihapus selama punya akun", func(t *testing.T) {
		_, err := db.Pool.Exec(ctx, `DELETE FROM stores WHERE id = $1`, store)
		if !database.IsForeignKeyViolation(err) {
			t.Fatalf("seharusnya ditahan RESTRICT: %v", err)
		}
	})
	t.Run("satu baris penerima toko per order", func(t *testing.T) {
		order := uuid.New()
		mustExec(t, db, `INSERT INTO orders (id, created_by, reference, idr_per_sol, rate_quoted_at, idempotency_key, request_hash, expires_at)
			VALUES ($1, $2, 'ref-1', 2500000, now(), 'key-1', 'hash', now())`, order, user)
		mustExec(t, db, `INSERT INTO order_splits (id, order_id, wallet_address, amount_lamports) VALUES ($1, $2, 'w', 1)`, uuid.New(), order)
		_, err := db.Pool.Exec(ctx, `INSERT INTO order_splits (id, order_id, wallet_address, amount_lamports) VALUES ($1, $2, 'w', 1)`, uuid.New(), order)
		if !database.IsUniqueViolation(err) {
			t.Fatalf("seharusnya melanggar unik walaupun consignor_id NULL: %v", err)
		}
	})
	t.Run("order confirmed wajib punya signature", func(t *testing.T) {
		_, err := db.Pool.Exec(ctx, `INSERT INTO orders (id, created_by, status, reference, idr_per_sol, rate_quoted_at, idempotency_key, request_hash, expires_at)
			VALUES ($1, $2, 'confirmed', 'ref-2', 2500000, now(), 'key-2', 'hash', now())`, uuid.New(), user)
		if !database.IsCheckViolation(err) {
			t.Fatalf("seharusnya melanggar CHECK: %v", err)
		}
	})
}

func TestInTxRollsBackOnError(t *testing.T) {
	db := testkit.NewDB(t)
	ctx := context.Background()
	id := uuid.New()
	err := db.InTx(ctx, func(ctx context.Context) error {
		if _, err := db.Q(ctx).Exec(ctx, `INSERT INTO stores (id, name) VALUES ($1, 'Toko')`, id); err != nil {
			return err
		}
		return context.Canceled
	})
	if err == nil {
		t.Fatal("error seharusnya diteruskan")
	}
	var n int
	if err := db.Pool.QueryRow(ctx, `SELECT count(*) FROM stores WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("transaksi seharusnya dibatalkan")
	}
}

func TestInTxNestsInOneTransaction(t *testing.T) {
	db := testkit.NewDB(t)
	ctx := context.Background()
	id := uuid.New()
	err := db.InTx(ctx, func(ctx context.Context) error {
		if err := db.InTx(ctx, func(ctx context.Context) error {
			_, err := db.Q(ctx).Exec(ctx, `INSERT INTO stores (id, name) VALUES ($1, 'Toko')`, id)
			return err
		}); err != nil {
			return err
		}
		return context.DeadlineExceeded
	})
	if err == nil {
		t.Fatal("error seharusnya diteruskan")
	}
	var n int
	if err := db.Pool.QueryRow(ctx, `SELECT count(*) FROM stores WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("transaksi dalam seharusnya ikut dibatalkan")
	}
}

func mustExec(t *testing.T, db *database.DB, sql string, args ...any) {
	t.Helper()
	if _, err := db.Pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}
