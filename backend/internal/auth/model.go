// Package auth mengurus identitas, peran, dan penautan wallet.
package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Role adalah peran pengguna di toko.
type Role string

const (
	RoleCashier   Role = "cashier"
	RoleOwner     Role = "store_owner"
	RoleConsignor Role = "consignor"
)

// Identity adalah hasil verifikasi JWT. JWT hanya membuktikan identitas, bukan peran.
type Identity struct {
	Subject       string
	Email         string
	EmailVerified bool
}

// Principal adalah pengguna yang sudah dikenali, dan selalu dibaca dari database.
type Principal struct {
	UserID         uuid.UUID
	StoreID        uuid.UUID
	StoreName      string
	Role           Role
	Email          string
	Name           string
	ConsignorID    *uuid.UUID
	WalletAddress  *string
	WalletLinkedAt *time.Time
}

// WalletRequired bernilai true untuk peran yang harus menautkan wallet.
func (p Principal) WalletRequired() bool {
	return p.Role == RoleOwner || p.Role == RoleConsignor
}

// HasWallet bernilai true kalau wallet sudah terbukti lewat tanda tangan.
func (p Principal) HasWallet() bool {
	return p.WalletAddress != nil
}

type principalKey struct{}

// WithPrincipal menyimpan Principal di context.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// FromContext membaca Principal. Handler di balik Guard selalu punya nilai ini.
func FromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// MustFromContext membaca Principal dan panic kalau route lupa memasang Guard.
func MustFromContext(ctx context.Context) Principal {
	p, ok := FromContext(ctx)
	if !ok {
		panic("route ini belum memasang auth.Guard")
	}
	return p
}

// Nonce adalah nonce penautan wallet.
type Nonce struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Address    string
	Value      string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}

// WalletOwner menjelaskan pemilik sebuah alamat wallet.
type WalletOwner struct {
	StoreID     *uuid.UUID
	ConsignorID *uuid.UUID
}
