// Package solana membungkus RPC Solana dan penyusunan transaksi.
// Server tidak pernah menyimpan private key, jadi transaksi selalu dikirim tanpa tanda tangan.
package solana

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"

	sol "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/mr-tron/base58"
)

// ExplorerURL mengembalikan tautan Explorer ke cluster Devnet.
func ExplorerURL(signature string) string {
	return "https://explorer.solana.com/tx/" + signature + "?cluster=devnet"
}

// ExplorerAddressURL mengembalikan tautan Explorer untuk sebuah akun di Devnet.
func ExplorerAddressURL(address string) string {
	return "https://explorer.solana.com/address/" + address + "?cluster=devnet"
}

// Transfer adalah satu perpindahan SOL.
type Transfer struct {
	From     string
	To       string
	Lamports uint64
}

// SignatureInfo adalah transaksi yang ditemukan lewat reference.
type SignatureInfo struct {
	Signature string
	Failed    bool
}

// Transaction adalah transaksi yang sudah terkonfirmasi, dalam bentuk yang dibutuhkan validator.
type Transaction struct {
	Signature   string
	Failed      bool
	FeePayer    string
	Transfers   []Transfer
	AccountKeys []string
	// OtherInstructions menghitung instruksi selain transfer SOL, selain Compute Budget.
	OtherInstructions int
}

// Client adalah semua kebutuhan RPC aplikasi. Test memakai implementasi tiruan.
type Client interface {
	LatestBlockhash(ctx context.Context) (string, error)
	Balance(ctx context.Context, address string) (uint64, error)
	RentExemptMinimum(ctx context.Context) (uint64, error)
	FindSignatures(ctx context.Context, reference string) ([]SignatureInfo, error)
	GetTransaction(ctx context.Context, signature string) (*Transaction, error)
}

// ErrNotFound berarti transaksi belum terlihat di RPC.
var ErrNotFound = errors.New("transaksi tidak ditemukan")

// NewReference membuat public key acak untuk menandai satu transaksi.
func NewReference() (string, error) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("buat reference: %w", err)
	}
	return base58.Encode(pub), nil
}

// ValidAddress memeriksa bahwa alamat adalah base58 dari 32 byte.
func ValidAddress(address string) bool {
	b, err := base58.Decode(address)
	return err == nil && len(b) == ed25519.PublicKeySize
}

// VerifyMessage memeriksa tanda tangan ed25519 dari wallet atas pesan.
func VerifyMessage(address, message, signature string) bool {
	pub, err := base58.Decode(address)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return false
	}
	sig, err := base58.Decode(signature)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pub), []byte(message), sig)
}

// BuildTransfers menyusun semua transfer dalam satu transaksi, jadi semuanya berhasil atau gagal
// bersama. payer menjadi fee payer dan satu-satunya penanda tangan. reference ikut sebagai akun
// read-only pada instruksi pertama, supaya transaksinya bisa ditemukan lewat getSignaturesForAddress.
func BuildTransfers(payer, blockhash, reference string, transfers []Transfer) (string, error) {
	if len(transfers) == 0 {
		return "", errors.New("transaksi tanpa transfer")
	}
	payerKey, err := sol.PublicKeyFromBase58(payer)
	if err != nil {
		return "", fmt.Errorf("alamat payer: %w", err)
	}
	refKey, err := sol.PublicKeyFromBase58(reference)
	if err != nil {
		return "", fmt.Errorf("reference: %w", err)
	}
	hash, err := sol.HashFromBase58(blockhash)
	if err != nil {
		return "", fmt.Errorf("blockhash: %w", err)
	}

	instructions := make([]sol.Instruction, 0, len(transfers))
	for i, t := range transfers {
		to, err := sol.PublicKeyFromBase58(t.To)
		if err != nil {
			return "", fmt.Errorf("alamat penerima: %w", err)
		}
		ix, err := system.NewTransferInstruction(t.Lamports, payerKey, to).ValidateAndBuild()
		if err != nil {
			return "", fmt.Errorf("instruksi transfer: %w", err)
		}
		if i > 0 {
			instructions = append(instructions, ix)
			continue
		}
		data, err := ix.Data()
		if err != nil {
			return "", fmt.Errorf("data transfer: %w", err)
		}
		accounts := append(ix.Accounts(), sol.Meta(refKey))
		instructions = append(instructions, sol.NewInstruction(ix.ProgramID(), accounts, data))
	}

	tx, err := sol.NewTransaction(instructions, hash, sol.TransactionPayer(payerKey))
	if err != nil {
		return "", fmt.Errorf("susun transaksi: %w", err)
	}
	tx.Signatures = make([]sol.Signature, tx.Message.Header.NumRequiredSignatures)
	raw, err := tx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("serialisasi transaksi: %w", err)
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

const systemTransferIndex = 2

var computeBudgetProgram = sol.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111")

// Decode mengubah transaksi mentah menjadi Transaction. Wallet boleh menambah instruksi
// Compute Budget, jadi instruksi itu tidak dihitung.
func Decode(tx *sol.Transaction, signature string, failed bool) (*Transaction, error) {
	msg := tx.Message
	if len(msg.AccountKeys) == 0 {
		return nil, errors.New("transaksi tanpa akun")
	}
	out := &Transaction{Signature: signature, Failed: failed, FeePayer: msg.AccountKeys[0].String()}
	for _, k := range msg.AccountKeys {
		out.AccountKeys = append(out.AccountKeys, k.String())
	}
	for _, ix := range msg.Instructions {
		if int(ix.ProgramIDIndex) >= len(msg.AccountKeys) {
			return nil, errors.New("indeks program di luar batas")
		}
		program := msg.AccountKeys[ix.ProgramIDIndex]
		if program.Equals(computeBudgetProgram) {
			continue
		}
		if program.Equals(sol.SystemProgramID) && len(ix.Data) == 12 && len(ix.Accounts) >= 2 &&
			binary.LittleEndian.Uint32(ix.Data[:4]) == systemTransferIndex {
			from, to := int(ix.Accounts[0]), int(ix.Accounts[1])
			if from >= len(msg.AccountKeys) || to >= len(msg.AccountKeys) {
				return nil, errors.New("indeks akun di luar batas")
			}
			out.Transfers = append(out.Transfers, Transfer{
				From:     msg.AccountKeys[from].String(),
				To:       msg.AccountKeys[to].String(),
				Lamports: binary.LittleEndian.Uint64(ix.Data[4:12]),
			})
			continue
		}
		out.OtherInstructions++
	}
	return out, nil
}

// DecodeBase64 membaca transaksi base64 yang disusun BuildTransfers.
func DecodeBase64(encoded string) (*Transaction, int, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, 0, fmt.Errorf("base64: %w", err)
	}
	tx, err := sol.TransactionFromBytes(raw)
	if err != nil {
		return nil, 0, fmt.Errorf("transaksi: %w", err)
	}
	out, err := Decode(tx, "", false)
	if err != nil {
		return nil, 0, err
	}
	return out, len(raw), nil
}

// ParseLamports mengubah string lamport menjadi uint64.
func ParseLamports(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}
