package solana

import (
	"context"
	"errors"
	"fmt"
	"time"

	sol "github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

const (
	signatureLimit = 100
	rpcTimeout     = 8 * time.Second
	rentFreeBytes  = 0
)

// RPC adalah Client yang memanggil node Solana sungguhan.
type RPC struct {
	client *rpc.Client
}

// NewRPC membuat klien untuk endpoint RPC.
func NewRPC(endpoint string) *RPC {
	return &RPC{client: rpc.New(endpoint)}
}

// LatestBlockhash membaca blockhash terbaru.
func (c *RPC) LatestBlockhash(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	out, err := c.client.GetLatestBlockhash(ctx, rpc.CommitmentConfirmed)
	if err != nil {
		return "", fmt.Errorf("getLatestBlockhash: %w", err)
	}
	return out.Value.Blockhash.String(), nil
}

// Balance membaca saldo wallet dalam lamport.
func (c *RPC) Balance(ctx context.Context, address string) (uint64, error) {
	key, err := sol.PublicKeyFromBase58(address)
	if err != nil {
		return 0, fmt.Errorf("alamat: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	out, err := c.client.GetBalance(ctx, key, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, fmt.Errorf("getBalance: %w", err)
	}
	return out.Value, nil
}

// RentExemptMinimum membaca saldo minimum akun tanpa data.
func (c *RPC) RentExemptMinimum(ctx context.Context) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	out, err := c.client.GetMinimumBalanceForRentExemption(ctx, rentFreeBytes, rpc.CommitmentConfirmed)
	if err != nil {
		return 0, fmt.Errorf("getMinimumBalanceForRentExemption: %w", err)
	}
	return out, nil
}

// FindSignatures mencari transaksi yang memuat reference, terlama lebih dulu.
func (c *RPC) FindSignatures(ctx context.Context, reference string) ([]SignatureInfo, error) {
	key, err := sol.PublicKeyFromBase58(reference)
	if err != nil {
		return nil, fmt.Errorf("reference: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	limit := signatureLimit
	out, err := c.client.GetSignaturesForAddressWithOpts(ctx, key, &rpc.GetSignaturesForAddressOpts{
		Limit:      &limit,
		Commitment: rpc.CommitmentConfirmed,
	})
	if err != nil {
		return nil, fmt.Errorf("getSignaturesForAddress: %w", err)
	}
	infos := make([]SignatureInfo, 0, len(out))
	for i := len(out) - 1; i >= 0; i-- {
		infos = append(infos, SignatureInfo{Signature: out[i].Signature.String(), Failed: out[i].Err != nil})
	}
	return infos, nil
}

// GetTransaction membaca transaksi yang sudah terkonfirmasi.
func (c *RPC) GetTransaction(ctx context.Context, signature string) (*Transaction, error) {
	sig, err := sol.SignatureFromBase58(signature)
	if err != nil {
		return nil, fmt.Errorf("signature: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	version := uint64(0)
	out, err := c.client.GetTransaction(ctx, sig, &rpc.GetTransactionOpts{
		Encoding:                       sol.EncodingBase64,
		Commitment:                     rpc.CommitmentConfirmed,
		MaxSupportedTransactionVersion: &version,
	})
	if errors.Is(err, rpc.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getTransaction: %w", err)
	}
	if out == nil || out.Meta == nil || out.Transaction == nil {
		return nil, ErrNotFound
	}
	tx, err := out.Transaction.GetTransaction()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	return Decode(tx, signature, out.Meta.Err != nil)
}
