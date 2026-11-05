// Package database membungkus koneksi Postgres, transaksi, dan migrasi.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/migrations"
)

// Querier dipenuhi pool maupun transaksi.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxRunner menjalankan fn dalam satu transaksi. Service bergantung pada interface ini.
type TxRunner interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type txKey struct{}

// DB adalah pool Postgres yang sadar transaksi di context.
type DB struct {
	Pool *pgxpool.Pool
}

// Open membuka pool dan memastikan database bisa dihubungi.
func Open(ctx context.Context, url string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &DB{Pool: pool}, nil
}

// Close menutup semua koneksi.
func (d *DB) Close() { d.Pool.Close() }

// Ping memeriksa koneksi.
func (d *DB) Ping(ctx context.Context) error { return d.Pool.Ping(ctx) }

// Q mengembalikan transaksi yang aktif di ctx, atau pool kalau tidak ada.
func (d *DB) Q(ctx context.Context) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return d.Pool
}

// InTx menjalankan fn dalam satu transaksi. Panggilan bersarang memakai transaksi yang sama.
// Jangan memanggil RPC Solana di dalam fn.
func (d *DB) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}
	return pgx.BeginFunc(ctx, d.Pool, func(tx pgx.Tx) error {
		return fn(context.WithValue(ctx, txKey{}, tx))
	})
}

// Migrate menjalankan semua migrasi yang belum berjalan.
func Migrate(ctx context.Context, d *DB) error {
	db := stdlib.OpenDBFromPool(d.Pool)
	defer db.Close()
	return migrateSQL(ctx, db, migrations.FS)
}

func migrateSQL(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// Kode error Postgres yang ditangani aplikasi.
const (
	codeUniqueViolation = "23505"
	codeForeignKey      = "23503"
	codeCheckViolation  = "23514"
)

// IsUniqueViolation memeriksa pelanggaran unik, dan opsional nama constraint-nya.
func IsUniqueViolation(err error, constraint ...string) bool {
	return isPgError(err, codeUniqueViolation, constraint...)
}

// IsForeignKeyViolation memeriksa pelanggaran foreign key.
func IsForeignKeyViolation(err error) bool {
	return isPgError(err, codeForeignKey)
}

// IsCheckViolation memeriksa pelanggaran CHECK.
func IsCheckViolation(err error) bool {
	return isPgError(err, codeCheckViolation)
}

func isPgError(err error, code string, constraint ...string) bool {
	var pg *pgconn.PgError
	if !errors.As(err, &pg) || pg.Code != code {
		return false
	}
	if len(constraint) == 0 {
		return true
	}
	for _, c := range constraint {
		if pg.ConstraintName == c {
			return true
		}
	}
	return false
}

// IsNoRows memeriksa hasil kosong.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
