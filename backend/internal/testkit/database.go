// Package testkit berisi alat bantu test: database sungguhan, klien API, dan validator kontrak.
package testkit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/migrations"
)

const lockKey = 7249013

// NewDB membuat database baru yang sudah termigrasi, khusus untuk satu test. Test dilewati kalau
// TEST_DATABASE_URL kosong.
func NewDB(t testing.TB) *database.DB {
	t.Helper()
	admin := os.Getenv("TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("TEST_DATABASE_URL kosong, test database dilewati")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("hubungi Postgres: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", lockKey); err != nil {
		t.Fatalf("advisory lock: %v", err)
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockKey) //nolint:errcheck

	template := "fc_tpl_" + migrationHash(t)
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", template).Scan(&exists); err != nil {
		t.Fatalf("cek template: %v", err)
	}
	if !exists {
		createTemplate(ctx, t, conn, admin, template)
	}

	name := "fc_t_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err := conn.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", name, template)); err != nil {
		t.Fatalf("buat database test: %v", err)
	}
	db, err := database.Open(ctx, withDatabase(t, admin, name))
	if err != nil {
		t.Fatalf("buka database test: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		cctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer ccancel()
		c, err := pgx.Connect(cctx, admin)
		if err != nil {
			t.Logf("hapus database test: %v", err)
			return
		}
		defer c.Close(cctx)
		if _, err := c.Exec(cctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name)); err != nil {
			t.Logf("hapus database test: %v", err)
		}
	})
	return db
}

func createTemplate(ctx context.Context, t testing.TB, conn *pgx.Conn, admin, template string) {
	t.Helper()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+template); err != nil {
		t.Fatalf("buat template: %v", err)
	}
	db, err := database.Open(ctx, withDatabase(t, admin, template))
	if err != nil {
		t.Fatalf("buka template: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		t.Fatalf("migrasi template: %v", err)
	}
}

func withDatabase(t testing.TB, raw, name string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("TEST_DATABASE_URL tidak valid: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}

func migrationHash(t testing.TB) string {
	t.Helper()
	h := sha256.New()
	err := fs.WalkDir(migrations.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(migrations.FS, path)
		if err != nil {
			return err
		}
		h.Write(b)
		return nil
	})
	if err != nil {
		t.Fatalf("hash migrasi: %v", err)
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}
