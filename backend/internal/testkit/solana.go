package testkit

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"

	sol "github.com/gagliardetto/solana-go"
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
	accounts   map[string]bool
	sent       [][]byte
	drop       bool
	sendErr    error
}

// NewFakeSolana membuat RPC tiruan. Semua wallet dianggap sudah punya saldo sampai diubah.
func NewFakeSolana() *FakeSolana {
	return &FakeSolana{
		balances:   map[string]uint64{},
		signatures: map[string][]solana.SignatureInfo{},
		txs:        map[string]*solana.Transaction{},
		malformed:  map[string]bool{},
		accounts:   map[string]bool{},
		Rent:       890880,
	}
}

func (f *FakeSolana) fail() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls++
	return f.Err
}

// DropTransactions membuat transaksi terkirim tidak pernah masuk jaringan.
func (f *FakeSolana) DropTransactions(drop bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.drop = drop
}

// SetSendErr membuat pengiriman transaksi gagal.
func (f *FakeSolana) SetSendErr(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sendErr = err
}

// SentTransactions membaca transaksi yang pernah dikirim.
func (f *FakeSolana) SentTransactions() [][]byte {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]byte(nil), f.sent...)
}

// AccountExists mengecek akun yang dibuat transaksi terkirim.
func (f *FakeSolana) AccountExists(_ context.Context, address string) (bool, error) {
	if err := f.fail(); err != nil {
		return false, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.accounts[address], nil
}

// RentExemptFor menjawab saldo minimum tetap.
func (f *FakeSolana) RentExemptFor(context.Context, uint64) (uint64, error) {
	if err := f.fail(); err != nil {
		return 0, err
	}
	return f.Rent, nil
}

// SendTransaction menyimpan transaksi. Semua penanda tangannya dianggap sudah punya akun.
func (f *FakeSolana) SendTransaction(_ context.Context, raw []byte) (string, error) {
	if err := f.fail(); err != nil {
		return "", err
	}
	tx, err := sol.TransactionFromBytes(raw)
	if err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.sendErr != nil {
		return "", f.sendErr
	}
	f.sent = append(f.sent, raw)
	if !f.drop {
		for _, key := range tx.Message.AccountKeys[:tx.Message.Header.NumRequiredSignatures] {
			f.accounts[key.String()] = true
		}
	}
	return tx.Signatures[0].String(), nil
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

// AddTransaction membuat transaksi bisa dibaca lewat signature saja.
func (f *FakeSolana) AddTransaction(tx *solana.Transaction) {
	f.mu.Lock()
	defer f.mu.Unlock()
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

// WalletFromSecret membaca kunci privat base58, bentuk yang diekspor Phantom.
func WalletFromSecret(secret string) (Wallet, error) {
	raw, err := base58.Decode(secret)
	if err != nil {
		return Wallet{}, fmt.Errorf("kunci wallet: %w", err)
	}
	if len(raw) != ed25519.PrivateKeySize {
		return Wallet{}, fmt.Errorf("kunci wallet %d byte, seharusnya %d", len(raw), ed25519.PrivateKeySize)
	}
	key := ed25519.PrivateKey(raw)
	return Wallet{Address: base58.Encode(key[ed25519.SeedSize:]), private: key}, nil
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
