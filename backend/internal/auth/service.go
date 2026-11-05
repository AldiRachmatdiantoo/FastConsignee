package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

// Service memegang aturan akun dan wallet.
type Service struct {
	repo     Repository
	tx       database.TxRunner
	now      func() time.Time
	domain   string
	nonceTTL time.Duration
}

// NewService membuat service auth.
func NewService(repo Repository, tx database.TxRunner, now func() time.Time, domain string, nonceTTL time.Duration) *Service {
	return &Service{repo: repo, tx: tx, now: now, domain: domain, nonceTTL: nonceTTL}
}

// Resolve mengubah identitas menjadi Principal. Akun dicocokkan lewat subject, lalu lewat email
// terverifikasi untuk login pertama. Peran selalu datang dari database.
func (s *Service) Resolve(ctx context.Context, id Identity) (Principal, error) {
	p, err := s.repo.FindBySubject(ctx, id.Subject)
	if err != nil {
		return Principal{}, err
	}
	if p != nil {
		return *p, nil
	}
	notAssigned := apperr.New(apperr.RoleNotAssigned, "Akun ini belum terdaftar di toko mana pun.")
	if !id.EmailVerified || id.Email == "" {
		return Principal{}, notAssigned
	}
	p, err = s.repo.FindUnboundByEmail(ctx, id.Email)
	if err != nil {
		return Principal{}, err
	}
	if p == nil {
		return Principal{}, notAssigned
	}
	if err := s.repo.BindSubject(ctx, p.UserID, id.Subject); err != nil {
		return Principal{}, err
	}
	return *p, nil
}

// Message adalah teks yang harus ditandatangani wallet.
func Message(domain, address, nonce string, expiresAt time.Time) string {
	return fmt.Sprintf(
		"%s meminta kamu menautkan wallet ini ke FastConsignee.\n\nAlamat: %s\nNonce: %s\nBerlaku sampai: %s\n\nMenandatangani pesan ini tidak memakai saldo.",
		domain, address, nonce, expiresAt.UTC().Format(time.RFC3339))
}

// CreateNonce membuat nonce sekali pakai.
func (s *Service) CreateNonce(ctx context.Context, p Principal, address string) (Nonce, string, error) {
	if !solana.ValidAddress(address) {
		return Nonce{}, "", apperr.Validation(apperr.FieldError{Field: "address", Message: "Alamat Solana tidak valid."})
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return Nonce{}, "", fmt.Errorf("buat nonce: %w", err)
	}
	n := Nonce{
		ID:        uuid.New(),
		UserID:    p.UserID,
		Address:   address,
		Value:     hex.EncodeToString(b),
		ExpiresAt: s.now().Add(s.nonceTTL).UTC().Truncate(time.Second),
	}
	if err := s.repo.CreateNonce(ctx, n); err != nil {
		return Nonce{}, "", err
	}
	return n, Message(s.domain, n.Address, n.Value, n.ExpiresAt), nil
}

// LinkWallet memverifikasi tanda tangan, lalu menautkan wallet dalam satu transaksi database.
func (s *Service) LinkWallet(ctx context.Context, p Principal, address, nonce, signature string) (string, time.Time, error) {
	if !solana.ValidAddress(address) {
		return "", time.Time{}, apperr.Validation(apperr.FieldError{Field: "address", Message: "Alamat Solana tidak valid."})
	}
	at := s.now().UTC()
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		n, err := s.repo.LockNonce(ctx, p.UserID, nonce)
		if err != nil {
			return err
		}
		if n == nil || n.ConsumedAt != nil || !at.Before(n.ExpiresAt) || n.Address != address {
			return apperr.New(apperr.NonceInvalid, "Nonce tidak berlaku. Minta nonce baru.")
		}
		if !solana.VerifyMessage(address, Message(s.domain, n.Address, n.Value, n.ExpiresAt), signature) {
			return apperr.New(apperr.SignatureInvalid, "Tanda tangan tidak cocok dengan alamat wallet.")
		}
		owner, err := s.repo.WalletOwner(ctx, address)
		if err != nil {
			return err
		}
		if owner != nil && !ownedBy(owner, p) {
			return apperr.New(apperr.WalletAlreadyLinked, "Wallet ini sudah tertaut ke pihak lain.")
		}
		if p.Role == RoleOwner {
			err = s.repo.SetStoreWallet(ctx, p.StoreID, address, at)
		} else {
			err = s.repo.SetConsignorWallet(ctx, *p.ConsignorID, address, at)
		}
		if err != nil {
			return err
		}
		return s.repo.ConsumeNonce(ctx, n.ID, at)
	})
	if database.IsUniqueViolation(err) {
		return "", time.Time{}, apperr.New(apperr.WalletAlreadyLinked, "Wallet ini sudah tertaut ke pihak lain.")
	}
	if err != nil {
		return "", time.Time{}, err
	}
	return address, at, nil
}

func ownedBy(o *WalletOwner, p Principal) bool {
	if p.Role == RoleOwner {
		return o.StoreID != nil && *o.StoreID == p.StoreID
	}
	return o.ConsignorID != nil && p.ConsignorID != nil && *o.ConsignorID == *p.ConsignorID
}
