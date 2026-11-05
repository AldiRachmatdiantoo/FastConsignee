package testkit

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"sync"

	"github.com/mr-tron/base58"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

// Blockhash adalah blockhash tiruan yang valid secara format.
const Blockhash = "4vJ9JU1bJJE96FWSJKvHsmmFADCg4gpZQff4P3bkLKi"

// FakeSolana adalah solana.Client di memori.
type FakeSolana struct {
	mu         sync.Mutex
	balances   map[string]uint64
	signatures map[string][]solana.SignatureInfo
	txs        map[string]*solana.Transaction
	malformed  map[string]bool
	Rent       uint64
	Err        error
	Calls      int
}

// NewFakeSolana membuat RPC tiruan. Semua wallet dianggap sudah punya saldo sampai diubah.
func NewFakeSolana() *FakeSolana {
	return &FakeSolana{
		balances:   map[string]uint64{},
		signatures: map[string][]solana.SignatureInfo{},
		txs:        map[string]*solana.Transaction{},
		malformed:  map[string]bool{},
		Rent:       890880,
	}
}

func (f *FakeSolana) fail() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls++
	return f.Err
}

// SetErr membuat semua panggilan RPC gagal.
func (f *FakeSolana) SetErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Err = err
}

// SetBalance mengatur saldo wallet.
func (f *FakeSolana) SetBalance(address string, lamports uint64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.balances[address] = lamports
}

// Publish membuat transaksi terlihat di RPC lewat reference.
func (f *FakeSolana) Publish(reference string, tx *solana.Transaction) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.signatures[reference] = append(f.signatures[reference], solana.SignatureInfo{Signature: tx.Signature, Failed: tx.Failed})
	f.txs[tx.Signature] = tx
}

// PublishSignature membuat signature terlihat, tetapi transaksinya belum bisa dibaca.
func (f *FakeSolana) PublishSignature(reference, signature string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.signatures[reference] = append(f.signatures[reference], solana.SignatureInfo{Signature: signature})
}

// PublishMalformed membuat transaksi yang isinya tidak bisa dibaca.
func (f *FakeSolana) PublishMalformed(reference, signature string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.signatures[reference] = append(f.signatures[reference], solana.SignatureInfo{Signature: signature})
	f.malformed[signature] = true
}

// LatestBlockhash mengembalikan blockhash tetap.
func (f *FakeSolana) LatestBlockhash(context.Context) (string, error) {
	return Blockhash, f.fail()
}

// Balance membaca saldo. Wallet yang tidak diatur dianggap punya 1 SOL.
func (f *FakeSolana) Balance(_ context.Context, address string) (uint64, error) {
	if err := f.fail(); err != nil {
		return 0, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if v, ok := f.balances[address]; ok {
		return v, nil
	}
	return 1_000_000_000, nil
}

// RentExemptMinimum mengembalikan minimum rent.
func (f *FakeSolana) RentExemptMinimum(context.Context) (uint64, error) {
	return f.Rent, f.fail()
}

// FindSignatures mencari transaksi lewat reference.
func (f *FakeSolana) FindSignatures(_ context.Context, reference string) ([]solana.SignatureInfo, error) {
	if err := f.fail(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]solana.SignatureInfo(nil), f.signatures[reference]...), nil
}

// GetTransaction membaca transaksi.
func (f *FakeSolana) GetTransaction(_ context.Context, signature string) (*solana.Transaction, error) {
	if err := f.fail(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.malformed[signature] {
		return nil, solana.ErrMalformed
	}
	if tx, ok := f.txs[signature]; ok {
		return tx, nil
	}
	return nil, solana.ErrNotFound
}

// Wallet adalah pasangan kunci untuk test.
type Wallet struct {
	Address string
	private ed25519.PrivateKey
}

// NewWallet membuat wallet acak.
func NewWallet() Wallet {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(errors.Join(errors.New("buat wallet"), err))
	}
	return Wallet{Address: base58.Encode(pub), private: priv}
}

// Sign menandatangani pesan seperti wallet sungguhan.
func (w Wallet) Sign(message string) string {
	return base58.Encode(ed25519.Sign(w.private, []byte(message)))
}

// NewSignature membuat signature transaksi acak.
func NewSignature() string {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base58.Encode(b)
}
