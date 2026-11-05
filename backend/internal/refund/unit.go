// Package refund mengurus pengembalian dana per unit barang.
package refund

import (
	"fmt"
	"math/big"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/order"
)

// Unit adalah nilai refund satu unit.
type Unit struct {
	AmountIDR      int64
	AmountLamports int64
	DeductionIDR   int64
}

// slice menghitung bagian ke-n dari total. Selisih dua pembulatan kumulatif membuat jumlah seluruh
// bagian sama persis dengan total, berapa pun sisa pembagiannya.
func slice(total int64, n, quantity int32) int64 {
	at := func(k int32) int64 {
		v := new(big.Int).Mul(big.NewInt(total), big.NewInt(int64(k)))
		return v.Quo(v, big.NewInt(int64(quantity))).Int64()
	}
	return at(n) - at(n-1)
}

// ValueOf menghitung refund unit ke-n pada satu baris, dimulai dari 1. Lamport dihitung dari kurs
// yang dikunci di order, jadi kurs saat refund tidak berpengaruh.
func ValueOf(line order.Line, idrPerSOL int64, n int32) (Unit, error) {
	lamports, err := order.FloorLamports(line.Amounts.LineTotal, idrPerSOL)
	if err != nil {
		return Unit{}, fmt.Errorf("hitung lamport baris: %w", err)
	}
	return Unit{
		AmountIDR:      slice(line.Amounts.LineTotal, n, line.Quantity),
		AmountLamports: slice(lamports, n, line.Quantity),
		DeductionIDR:   slice(line.Amounts.ConsignorShare, n, line.Quantity),
	}, nil
}
