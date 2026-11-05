package refund

import (
	"math/rand/v2"
	"testing"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/order"
)

func lineOf(t *testing.T, price int64, qty, bps int32, discount int64, bearer string) order.Line {
	t.Helper()
	a, err := order.ComputeLine(price, qty, bps, discount, bearer)
	if err != nil {
		t.Fatal(err)
	}
	return order.Line{Quantity: qty, Amounts: a}
}

func TestUnitValueExample(t *testing.T) {
	l := lineOf(t, 100000, 1, 2000, 0, "")
	u, err := ValueOf(l, 2500000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if u.AmountIDR != 100000 || u.AmountLamports != 40000000 || u.DeductionIDR != 80000 {
		t.Fatalf("hasil %+v", u)
	}
}

func TestUnitValuesSpreadRemainder(t *testing.T) {
	l := lineOf(t, 33333, 3, 1000, 1, order.BearerStore)
	var idr []int64
	for n := int32(1); n <= 3; n++ {
		u, err := ValueOf(l, 2500000, n)
		if err != nil {
			t.Fatal(err)
		}
		idr = append(idr, u.AmountIDR)
	}
	if idr[0]+idr[1]+idr[2] != 99998 {
		t.Fatalf("jumlah %v", idr)
	}
	if max(idr[0], idr[1], idr[2])-min(idr[0], idr[1], idr[2]) > 1 {
		t.Fatalf("selisih antarunit harus paling besar 1: %v", idr)
	}
}

func TestDiscountByStoreDoesNotReduceConsignorDeduction(t *testing.T) {
	l := lineOf(t, 100000, 1, 2000, 5000, order.BearerStore)
	u, err := ValueOf(l, 2500000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if u.AmountIDR != 95000 || u.DeductionIDR != 80000 {
		t.Fatalf("hasil %+v", u)
	}
}

func TestUnitValuesSumExactly(t *testing.T) {
	r := rand.New(rand.NewPCG(13, 17))
	for range 3000 {
		qty := 1 + r.Int32N(1000)
		price := 1 + r.Int64N(10_000_000_000)
		rate := 1_000 + r.Int64N(100_000_000)
		l := lineOf(t, price, qty, r.Int32N(10001), 0, "")
		wantLamports, err := order.FloorLamports(l.Amounts.LineTotal, rate)
		if err != nil {
			continue
		}
		var idr, lamports, deduction int64
		for n := int32(1); n <= qty; n++ {
			u, err := ValueOf(l, rate, n)
			if err != nil {
				t.Fatal(err)
			}
			if u.AmountIDR < 0 || u.AmountLamports < 0 || u.DeductionIDR < 0 {
				t.Fatalf("nilai negatif: %+v", u)
			}
			idr += u.AmountIDR
			lamports += u.AmountLamports
			deduction += u.DeductionIDR
		}
		if idr != l.Amounts.LineTotal || lamports != wantLamports || deduction != l.Amounts.ConsignorShare {
			t.Fatalf("jumlah tidak sama: %d %d %d dari %+v", idr, lamports, deduction, l.Amounts)
		}
	}
}

func FuzzUnitValues(f *testing.F) {
	f.Add(int64(100000), int32(3), int32(2000), int64(2500000))
	f.Add(int64(1), int32(1000), int32(0), int64(1))
	f.Fuzz(func(t *testing.T, price int64, qty, bps int32, rate int64) {
		if price < 1 || price > 10_000_000_000 || qty < 1 || qty > 1000 || bps < 0 || bps > 10000 ||
			rate < 1 || rate > 1_000_000_000_000 {
			t.Skip()
		}
		l := lineOf(t, price, qty, bps, 0, "")
		lamportTotal, err := order.FloorLamports(l.Amounts.LineTotal, rate)
		if err != nil {
			t.Skip()
		}
		var idr, lamports, deduction int64
		for n := int32(1); n <= qty; n++ {
			u, err := ValueOf(l, rate, n)
			if err != nil {
				t.Fatal(err)
			}
			idr += u.AmountIDR
			lamports += u.AmountLamports
			deduction += u.DeductionIDR
		}
		if idr != l.Amounts.LineTotal || lamports != lamportTotal || deduction != l.Amounts.ConsignorShare {
			t.Fatalf("jumlah tidak sama: %d %d %d", idr, lamports, deduction)
		}
	})
}
