// Command seed memuat data demo ke database. Aman dijalankan berulang.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/seed"
)

func main() {
	if err := run(context.Background()); err != nil {
		slog.Error("seed gagal", "error", err)
		os.Exit(1)
	}
}

func list(name string) []string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return nil
	}
	return strings.Split(v, ",")
}

func run(ctx context.Context) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL wajib diisi")
	}
	db, err := database.Open(ctx, url)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := database.Migrate(ctx, db); err != nil {
		return err
	}
	out, err := seed.Run(ctx, db, seed.Options{
		OwnerEmail:       os.Getenv("SEED_OWNER_EMAIL"),
		CashierEmail:     os.Getenv("SEED_CASHIER_EMAIL"),
		ConsignorEmails:  list("SEED_CONSIGNOR_EMAILS"),
		ConsignorWallets: list("SEED_CONSIGNOR_WALLETS"),
	})
	if err != nil {
		return err
	}
	fmt.Printf("Toko %s siap. Baru dibuat: %d akun, %d penitip, %d barang.\n", out.StoreID, out.Users, out.Consignors, out.Items)
	return nil
}
