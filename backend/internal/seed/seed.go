// Package seed memuat data demo: satu toko, tiga penitip, wallet, dan barangnya (FR-21).
package seed

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mr-tron/base58"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

const (
	storeName           = "Toko Titip Nusantara"
	defaultCashierEmail = "kasir@fastconsignee.test"
)

type product struct {
	name  string
	price int64
	bps   int32
	stock int32
}

// certifiedProduct adalah barang demo yang mengirim NFT sertifikat. Alur demo utama tidak memakainya.
const certifiedProduct = "Kain Batik Tulis"

type consignor struct {
	name  string
	email string
	items []product
}

var consignors = []consignor{
	{"Sari Anyaman", "sari@fastconsignee.test", []product{
		{"Tas Rotan Bundar", 185000, 2000, 12},
		{"Keranjang Bambu", 95000, 2000, 20},
		{"Dompet Pandan", 45000, 1500, 30},
	}},
	{"Budi Kopi", "budi@fastconsignee.test", []product{
		{"Kopi Gayo 250 g", 85000, 1000, 40},
		{"Kopi Toraja 250 g", 95000, 1000, 40},
		{"Gula Aren Cair", 38000, 1200, 25},
	}},
	{"Rina Batik", "rina@fastconsignee.test", []product{
		{"Kain Batik Tulis", 750000, 2500, 5},
		{"Selendang Sutra", 240000, 2500, 10},
		{"Syal Batik", 120000, 2500, 15},
	}},
}

// Options mengatur akun demo. Hanya OwnerEmail yang wajib, karena pemilik harus bisa login Google.
type Options struct {
	OwnerEmail       string
	CashierEmail     string
	ConsignorEmails  []string
	ConsignorWallets []string
}

// Result merangkum isi yang baru dibuat. Menjalankan seed dua kali tidak membuat data baru.
type Result struct {
	StoreID    uuid.UUID
	Users      int
	Consignors int
	Items      int
}

func (o Options) normalized() (Options, error) {
	o.OwnerEmail = strings.ToLower(strings.TrimSpace(o.OwnerEmail))
	if o.OwnerEmail == "" {
		return o, errors.New("email pemilik wajib diisi")
	}
	o.CashierEmail = strings.ToLower(strings.TrimSpace(o.CashierEmail))
	if o.CashierEmail == "" {
		o.CashierEmail = defaultCashierEmail
	}
	if n := len(o.ConsignorEmails); n != 0 && n != len(consignors) {
		return o, fmt.Errorf("email penitip harus %d, bukan %d", len(consignors), n)
	}
	if n := len(o.ConsignorWallets); n != 0 && n != len(consignors) {
		return o, fmt.Errorf("wallet penitip harus %d, bukan %d", len(consignors), n)
	}
	for _, w := range o.ConsignorWallets {
		if !solana.ValidAddress(w) {
			return o, fmt.Errorf("wallet penitip tidak valid: %q", w)
		}
	}
	return o, nil
}

func (o Options) consignorEmail(i int) string {
	if len(o.ConsignorEmails) == 0 {
		return consignors[i].email
	}
	return strings.ToLower(strings.TrimSpace(o.ConsignorEmails[i]))
}

// consignorWallet memakai alamat pilihan pengguna. Tanpa itu, alamat dibuat acak dan kunci
// privatnya dibuang, jadi dana Devnet yang masuk ke sana tidak bisa dipakai lagi.
func (o Options) consignorWallet(i int) (string, error) {
	if len(o.ConsignorWallets) != 0 {
		return o.ConsignorWallets[i], nil
	}
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("buat wallet demo: %w", err)
	}
	return base58.Encode(public), nil
}

// Run memuat data demo dalam satu transaksi.
func Run(ctx context.Context, db *database.DB, options Options) (Result, error) {
	o, err := options.normalized()
	if err != nil {
		return Result{}, err
	}
	var out Result
	err = db.InTx(ctx, func(ctx context.Context) error {
		s := seeder{q: db.Q(ctx)}
		if err := s.run(ctx, o); err != nil {
			return err
		}
		out = s.result
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return out, nil
}

type seeder struct {
	q      database.Querier
	result Result
}

func (s *seeder) run(ctx context.Context, o Options) error {
	if err := s.store(ctx, o.OwnerEmail); err != nil {
		return err
	}
	if _, err := s.user(ctx, auth.RoleCashier, o.CashierEmail, "Kasir Demo"); err != nil {
		return err
	}
	for i, c := range consignors {
		id, err := s.consignor(ctx, c, o, i)
		if err != nil {
			return err
		}
		for _, p := range c.items {
			if err := s.item(ctx, id, p); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *seeder) store(ctx context.Context, ownerEmail string) error {
	err := s.q.QueryRow(ctx, `SELECT store_id FROM users WHERE email = $1`, ownerEmail).Scan(&s.result.StoreID)
	if err == nil {
		return nil
	}
	if !database.IsNoRows(err) {
		return fmt.Errorf("cari pemilik: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	if _, err := s.q.Exec(ctx, `INSERT INTO stores (id, name) VALUES ($1, $2)`, id, storeName); err != nil {
		return fmt.Errorf("buat toko: %w", err)
	}
	s.result.StoreID = id
	_, err = s.user(ctx, auth.RoleOwner, ownerEmail, "Pemilik Demo")
	return err
}

func (s *seeder) user(ctx context.Context, role auth.Role, email, name string) (uuid.UUID, error) {
	var (
		id    uuid.UUID
		store uuid.UUID
	)
	err := s.q.QueryRow(ctx, `SELECT id, store_id FROM users WHERE email = $1`, email).Scan(&id, &store)
	if err == nil {
		if store != s.result.StoreID {
			return uuid.Nil, fmt.Errorf("email %s sudah terdaftar di toko lain", email)
		}
		return id, nil
	}
	if !database.IsNoRows(err) {
		return uuid.Nil, fmt.Errorf("cari akun: %w", err)
	}
	id, err = uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	_, err = s.q.Exec(ctx, `INSERT INTO users (id, store_id, role, email, name) VALUES ($1, $2, $3, $4, $5)`,
		id, s.result.StoreID, string(role), email, name)
	if err != nil {
		return uuid.Nil, fmt.Errorf("buat akun: %w", err)
	}
	s.result.Users++
	return id, nil
}

func (s *seeder) consignor(ctx context.Context, c consignor, o Options, index int) (uuid.UUID, error) {
	userID, err := s.user(ctx, auth.RoleConsignor, o.consignorEmail(index), c.name)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = s.q.QueryRow(ctx, `SELECT id FROM consignors WHERE user_id = $1`, userID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !database.IsNoRows(err) {
		return uuid.Nil, fmt.Errorf("cari penitip: %w", err)
	}
	wallet, err := o.consignorWallet(index)
	if err != nil {
		return uuid.Nil, err
	}
	id, err = uuid.NewV7()
	if err != nil {
		return uuid.Nil, err
	}
	_, err = s.q.Exec(ctx, `INSERT INTO consignors (id, store_id, user_id, name, wallet_address) VALUES ($1, $2, $3, $4, $5)`,
		id, s.result.StoreID, userID, c.name, wallet)
	if err != nil {
		return uuid.Nil, fmt.Errorf("buat penitip: %w", err)
	}
	s.result.Consignors++
	return id, nil
}

func (s *seeder) item(ctx context.Context, consignorID uuid.UUID, p product) error {
	var exists bool
	err := s.q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM items WHERE consignor_id = $1 AND name = $2)`, consignorID, p.name).Scan(&exists)
	if err != nil {
		return fmt.Errorf("cari barang: %w", err)
	}
	if exists {
		return nil
	}
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	_, err = s.q.Exec(ctx, `INSERT INTO items (id, consignor_id, name, price_idr, commission_bps, stock_on_hand, certificate_enabled) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, consignorID, p.name, p.price, p.bps, p.stock, p.name == certifiedProduct)
	if err != nil {
		return fmt.Errorf("buat barang: %w", err)
	}
	s.result.Items++
	return nil
}
