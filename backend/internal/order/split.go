// Package order mengurus keranjang, pembagian dana, dan order.
package order

import (
	"errors"
	"math"
	"math/big"

	"github.com/google/uuid"
)

const (
	bpsTotal       = 10_000
	lamportsPerSOL = 1_000_000_000

	// MaxConsignors adalah batas penitip per order, supaya transaksi muat dalam satu paket Solana.
	MaxConsignors = 5

	// BearerStore berarti diskon diambil dari komisi toko.
	BearerStore = "store"
	// BearerConsignor berarti diskon diambil dari bagian penitip.
	BearerConsignor = "consignor"
)

var (
	// ErrDiscountTooLarge berarti diskon melebihi bagian penanggungnya di baris itu.
	ErrDiscountTooLarge = errors.New("diskon melebihi bagian penanggung")
	// ErrAmountTooLarge berarti nilai lamport tidak muat di kolom bigint.
	ErrAmountTooLarge = errors.New("nilai lamport melebihi batas")
)

// Amounts adalah nilai Rupiah satu baris.
type Amounts struct {
	Gross          int64
	Discount       int64
	LineTotal      int64
	Commission     int64
	ConsignorShare int64
}

// ComputeLine menghitung satu baris. Bagian penitip dibulatkan ke bawah, jadi sisa Rupiah masuk ke
// komisi. Diskon ditanggung penuh oleh satu pihak.
func ComputeLine(unitPrice int64, quantity, bps int32, discount int64, borneBy string) (Amounts, error) {
	gross := unitPrice * int64(quantity)
	share := gross * int64(bpsTotal-bps) / bpsTotal
	a := Amounts{Gross: gross, Discount: discount, LineTotal: gross - discount, Commission: gross - share, ConsignorShare: share}
	switch {
	case discount == 0:
	case borneBy == BearerStore && discount <= a.Commission:
		a.Commission -= discount
	case borneBy == BearerConsignor && discount <= a.ConsignorShare:
		a.ConsignorShare -= discount
	default:
		return Amounts{}, ErrDiscountTooLarge
	}
	return a, nil
}

// Share adalah total bagian satu penitip di order.
type Share struct {
	ConsignorID    uuid.UUID
	Name           string
	Wallet         string
	ShareIDR       int64
	OutstandingIDR int64
}

// StoreShare adalah total komisi toko di order.
type StoreShare struct {
	Name     string
	Wallet   string
	ShareIDR int64
}

// Recipient adalah satu penerima dana. ConsignorID bernilai nil untuk toko.
type Recipient struct {
	ConsignorID    *uuid.UUID
	Name           string
	Wallet         string
	ShareIDR       int64
	DeductionIDR   int64
	RecoveredIDR   int64
	AmountIDR      int64
	AmountLamports int64
}

// Split adalah pembagian dana satu order. Toko selalu menjadi penerima terakhir.
type Split struct {
	Recipients    []Recipient
	TotalIDR      int64
	TotalLamports int64
}

// BuildSplit menerapkan potongan refund lalu mengubah Rupiah menjadi lamport. Pembayar ditagih
// dengan pembulatan ke atas, penitip menerima pembulatan ke bawah, dan toko menerima sisanya, jadi
// jumlah semua penerima selalu sama dengan total.
func BuildSplit(consignors []Share, store StoreShare, idrPerSOL int64) (Split, error) {
	total := store.ShareIDR
	for _, c := range consignors {
		total += c.ShareIDR
	}
	totalLamports, err := toLamports(total, idrPerSOL, true)
	if err != nil {
		return Split{}, err
	}
	out := make([]Recipient, 0, len(consignors)+1)
	var recovered, paid int64
	for _, c := range consignors {
		deduction := min(c.OutstandingIDR, c.ShareIDR)
		amount := c.ShareIDR - deduction
		lamports, err := toLamports(amount, idrPerSOL, false)
		if err != nil {
			return Split{}, err
		}
		id := c.ConsignorID
		out = append(out, Recipient{
			ConsignorID: &id, Name: c.Name, Wallet: c.Wallet, ShareIDR: c.ShareIDR,
			DeductionIDR: deduction, AmountIDR: amount, AmountLamports: lamports,
		})
		recovered += deduction
		paid += lamports
	}
	out = append(out, Recipient{
		Name: store.Name, Wallet: store.Wallet, ShareIDR: store.ShareIDR, RecoveredIDR: recovered,
		AmountIDR: store.ShareIDR + recovered, AmountLamports: totalLamports - paid,
	})
	return Split{Recipients: out, TotalIDR: total, TotalLamports: totalLamports}, nil
}

// Same bernilai true kalau tiap penerima menerima lamport yang sama.
func (s Split) Same(other Split) bool {
	if len(s.Recipients) != len(other.Recipients) {
		return false
	}
	for i, r := range s.Recipients {
		o := other.Recipients[i]
		if r.Wallet != o.Wallet || r.AmountLamports != o.AmountLamports {
			return false
		}
	}
	return true
}

// FloorLamports mengubah Rupiah menjadi lamport dengan pembulatan ke bawah.
func FloorLamports(idr, idrPerSOL int64) (int64, error) {
	return toLamports(idr, idrPerSOL, false)
}

// toLamports memakai big.Int karena Rupiah dikali 1e9 bisa melampaui int64.
func toLamports(idr, idrPerSOL int64, roundUp bool) (int64, error) {
	n := new(big.Int).Mul(big.NewInt(idr), big.NewInt(lamportsPerSOL))
	rate := big.NewInt(idrPerSOL)
	q, rem := new(big.Int).QuoRem(n, rate, new(big.Int))
	if roundUp && rem.Sign() > 0 {
		q.Add(q, big.NewInt(1))
	}
	if q.Cmp(big.NewInt(math.MaxInt64)) > 0 {
		return 0, ErrAmountTooLarge
	}
	return q.Int64(), nil
}

// Outstanding adalah sisa satu potongan refund.
type Outstanding struct {
	DeductionID  uuid.UUID
	ConsignorID  uuid.UUID
	RemainingIDR int64
}

// Settlement adalah bagian potongan yang dilunasi order ini.
type Settlement struct {
	DeductionID uuid.UUID
	ConsignorID uuid.UUID
	AmountIDR   int64
}

// Allocate melunasi potongan terlama lebih dulu. outstanding harus terurut dari yang terlama.
func Allocate(outstanding []Outstanding, taken map[uuid.UUID]int64) []Settlement {
	left := make(map[uuid.UUID]int64, len(taken))
	for id, v := range taken {
		left[id] = v
	}
	var out []Settlement
	for _, o := range outstanding {
		amount := min(o.RemainingIDR, left[o.ConsignorID])
		if amount <= 0 {
			continue
		}
		left[o.ConsignorID] -= amount
		out = append(out, Settlement{DeductionID: o.DeductionID, ConsignorID: o.ConsignorID, AmountIDR: amount})
	}
	return out
}
