package order

import (
	"context"
	"errors"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/cache"
)

func TestComputeLine(t *testing.T) {
	cases := []struct {
		name                       string
		price                      int64
		qty, bps                   int32
		discount                   int64
		bearer                     string
		total, commission, partner int64
	}{
		{"komisi 20 persen", 100000, 1, 2000, 0, "", 100000, 20000, 80000},
		{"sisa pembulatan masuk ke toko", 3333, 1, 1500, 0, "", 3333, 500, 2833},
		{"tanpa komisi", 5000, 2, 0, 0, "", 10000, 0, 10000},
		{"komisi penuh", 5000, 2, 10000, 0, "", 10000, 10000, 0},
		{"diskon toko", 100000, 1, 2000, 5000, BearerStore, 95000, 15000, 80000},
		{"diskon penitip", 100000, 1, 2000, 5000, BearerConsignor, 95000, 20000, 75000},
		{"diskon toko habis", 100000, 1, 2000, 20000, BearerStore, 80000, 0, 80000},
		{"diskon penitip habis", 100000, 1, 2000, 80000, BearerConsignor, 20000, 20000, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, err := ComputeLine(c.price, c.qty, c.bps, c.discount, c.bearer)
			if err != nil {
				t.Fatal(err)
			}
			if a.LineTotal != c.total || a.Commission != c.commission || a.ConsignorShare != c.partner {
				t.Fatalf("hasil %+v", a)
			}
		})
	}
}

func TestComputeLineRejectsDiscountAboveBearerShare(t *testing.T) {
	if _, err := ComputeLine(100000, 1, 2000, 20001, BearerStore); !errors.Is(err, ErrDiscountTooLarge) {
		t.Fatalf("diskon toko: %v", err)
	}
	if _, err := ComputeLine(100000, 1, 2000, 80001, BearerConsignor); !errors.Is(err, ErrDiscountTooLarge) {
		t.Fatalf("diskon penitip: %v", err)
	}
	if _, err := ComputeLine(100000, 1, 2000, 1, ""); !errors.Is(err, ErrDiscountTooLarge) {
		t.Fatalf("tanpa penanggung: %v", err)
	}
}

func assertSplit(t *testing.T, s Split) {
	t.Helper()
	var idr, lamports int64
	for _, r := range s.Recipients {
		if r.AmountIDR != r.ShareIDR-r.DeductionIDR+r.RecoveredIDR {
			t.Fatalf("rumus penerima salah: %+v", r)
		}
		if r.AmountIDR < 0 || r.AmountLamports < 0 {
			t.Fatalf("nilai negatif: %+v", r)
		}
		idr += r.AmountIDR
		lamports += r.AmountLamports
	}
	if idr != s.TotalIDR {
		t.Fatalf("jumlah Rupiah %d, total %d", idr, s.TotalIDR)
	}
	if lamports != s.TotalLamports {
		t.Fatalf("jumlah lamport %d, total %d", lamports, s.TotalLamports)
	}
}

func TestBuildSplitMatchesContractExample(t *testing.T) {
	s, err := BuildSplit([]Share{
		{ConsignorID: uuid.New(), ShareIDR: 80000},
		{ConsignorID: uuid.New(), ShareIDR: 85000},
	}, StoreShare{ShareIDR: 35000}, 2500000)
	if err != nil {
		t.Fatal(err)
	}
	assertSplit(t, s)
	want := []int64{32000000, 34000000, 14000000}
	for i, r := range s.Recipients {
		if r.AmountLamports != want[i] {
			t.Fatalf("penerima %d: %d", i, r.AmountLamports)
		}
	}
	if s.Recipients[2].ConsignorID != nil || s.TotalLamports != 80000000 {
		t.Fatalf("toko harus terakhir: %+v", s)
	}
}

func TestBuildSplitRoundsInFavorOfExactTotal(t *testing.T) {
	s, err := BuildSplit([]Share{{ConsignorID: uuid.New(), ShareIDR: 1}, {ConsignorID: uuid.New(), ShareIDR: 1}},
		StoreShare{ShareIDR: 1}, 3000001)
	if err != nil {
		t.Fatal(err)
	}
	assertSplit(t, s)
	if s.Recipients[2].AmountLamports <= s.Recipients[0].AmountLamports {
		t.Fatalf("sisa pembulatan harus ke toko: %+v", s.Recipients)
	}
}

func TestBuildSplitAppliesDeduction(t *testing.T) {
	s, err := BuildSplit([]Share{
		{ConsignorID: uuid.New(), ShareIDR: 80000, OutstandingIDR: 30000},
		{ConsignorID: uuid.New(), ShareIDR: 10000, OutstandingIDR: 25000},
	}, StoreShare{ShareIDR: 10000}, 2500000)
	if err != nil {
		t.Fatal(err)
	}
	assertSplit(t, s)
	r := s.Recipients
	if r[0].AmountIDR != 50000 || r[1].AmountIDR != 0 || r[1].DeductionIDR != 10000 {
		t.Fatalf("potongan salah: %+v", r)
	}
	if r[2].RecoveredIDR != 40000 || r[2].AmountIDR != 50000 {
		t.Fatalf("talangan harus kembali ke toko: %+v", r[2])
	}
}

func TestBuildSplitRejectsLamportOverflow(t *testing.T) {
	_, err := BuildSplit(nil, StoreShare{ShareIDR: 1_000_000_000_000_000}, 1)
	if !errors.Is(err, ErrAmountTooLarge) {
		t.Fatalf("seharusnya ditolak: %v", err)
	}
}

func TestBuildSplitProperties(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 11))
	for range 20000 {
		n := r.IntN(MaxConsignors + 1)
		shares := make([]Share, n)
		for i := range shares {
			shares[i] = Share{ConsignorID: uuid.New(), ShareIDR: r.Int64N(1_000_000_000), OutstandingIDR: r.Int64N(1_000_000_000)}
		}
		s, err := BuildSplit(shares, StoreShare{ShareIDR: r.Int64N(1_000_000_000)}, 1+r.Int64N(100_000_000))
		if errors.Is(err, ErrAmountTooLarge) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		assertSplit(t, s)
		for i, c := range shares {
			got := s.Recipients[i]
			if got.DeductionIDR > c.OutstandingIDR || got.DeductionIDR > c.ShareIDR {
				t.Fatalf("potongan berlebih: %+v dari %+v", got, c)
			}
		}
	}
}

func TestComputeLineProperties(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 5))
	for range 20000 {
		price, qty, bps := 1+r.Int64N(10_000_000_000), 1+r.Int32N(1000), r.Int32N(10001)
		base, err := ComputeLine(price, qty, bps, 0, "")
		if err != nil {
			t.Fatal(err)
		}
		if base.Commission+base.ConsignorShare != base.Gross {
			t.Fatalf("bagian tidak sama dengan bruto: %+v", base)
		}
		bearer, limit := BearerStore, base.Commission
		if r.IntN(2) == 0 {
			bearer, limit = BearerConsignor, base.ConsignorShare
		}
		if limit == 0 {
			continue
		}
		a, err := ComputeLine(price, qty, bps, 1+r.Int64N(limit), bearer)
		if err != nil {
			t.Fatal(err)
		}
		if a.Commission+a.ConsignorShare != a.LineTotal || a.Commission < 0 || a.ConsignorShare < 0 {
			t.Fatalf("diskon merusak jumlah: %+v", a)
		}
	}
}

func FuzzComputeLine(f *testing.F) {
	f.Add(int64(100000), int32(1), int32(2000), int64(5000), true)
	f.Add(int64(1), int32(1), int32(9999), int64(0), false)
	f.Add(int64(10_000_000_000), int32(1000), int32(1), int64(1), false)
	f.Fuzz(func(t *testing.T, price int64, qty, bps int32, discount int64, store bool) {
		if price < 1 || price > 10_000_000_000 || qty < 1 || qty > 1000 || bps < 0 || bps > 10000 ||
			discount < 0 || discount > 1_000_000_000_000_000 {
			t.Skip()
		}
		bearer := BearerConsignor
		if store {
			bearer = BearerStore
		}
		a, err := ComputeLine(price, qty, bps, discount, bearer)
		if errors.Is(err, ErrDiscountTooLarge) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if a.Commission < 0 || a.ConsignorShare < 0 || a.Commission+a.ConsignorShare != a.LineTotal {
			t.Fatalf("hasil tidak konsisten: %+v", a)
		}
	})
}

func FuzzBuildSplit(f *testing.F) {
	f.Add(int64(80000), int64(30000), int64(85000), int64(0), int64(35000), int64(2500000))
	f.Add(int64(1), int64(1), int64(1), int64(5), int64(0), int64(3000001))
	f.Add(int64(0), int64(0), int64(0), int64(0), int64(1), int64(1))
	f.Fuzz(func(t *testing.T, a, owedA, b, owedB, store, rate int64) {
		const limit = 1_000_000_000_000_000
		for _, v := range []int64{a, owedA, b, owedB, store} {
			if v < 0 || v > limit {
				t.Skip()
			}
		}
		if rate < 1 || rate > 1_000_000_000_000 {
			t.Skip()
		}
		s, err := BuildSplit([]Share{
			{ConsignorID: uuid.New(), ShareIDR: a, OutstandingIDR: owedA},
			{ConsignorID: uuid.New(), ShareIDR: b, OutstandingIDR: owedB},
		}, StoreShare{ShareIDR: store}, rate)
		if errors.Is(err, ErrAmountTooLarge) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		assertSplit(t, s)
	})
}

func TestAllocateSettlesOldestFirst(t *testing.T) {
	a, b, c1, c2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	got := Allocate([]Outstanding{
		{DeductionID: a, ConsignorID: c1, RemainingIDR: 100},
		{DeductionID: b, ConsignorID: c1, RemainingIDR: 100},
		{DeductionID: uuid.New(), ConsignorID: c2, RemainingIDR: 100},
	}, map[uuid.UUID]int64{c1: 150})
	if len(got) != 2 {
		t.Fatalf("pelunasan %+v", got)
	}
	if got[0].DeductionID != a || got[0].AmountIDR != 100 || got[1].DeductionID != b || got[1].AmountIDR != 50 {
		t.Fatalf("urutan salah: %+v", got)
	}
}

func TestSplitSame(t *testing.T) {
	a := Split{Recipients: []Recipient{{Wallet: "A", AmountLamports: 1}}}
	if !a.Same(Split{Recipients: []Recipient{{Wallet: "A", AmountLamports: 1}}}) {
		t.Fatal("seharusnya sama")
	}
	if a.Same(Split{Recipients: []Recipient{{Wallet: "A", AmountLamports: 2}}}) || a.Same(Split{}) {
		t.Fatal("seharusnya berbeda")
	}
}

func now() time.Time { return time.Date(2026, 11, 7, 3, 0, 0, 0, time.UTC) }

func TestRateReadsMarketAndCaches(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"solana":{"idr":2534567.6}}`))
	}))
	defer server.Close()
	clock := now()
	c := cache.NewMemory(func() time.Time { return clock })
	m := NewMarketRate(server.URL, 0, time.Minute, c, func() time.Time { return clock })
	for range 3 {
		r, err := m.Current(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if r.IDRPerSOL != 2534568 {
			t.Fatalf("kurs %d", r.IDRPerSOL)
		}
	}
	if calls != 1 {
		t.Fatalf("pasar dipanggil %d kali", calls)
	}
	clock = clock.Add(2 * time.Minute)
	if _, err := m.Current(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("cache seharusnya kedaluwarsa, pasar dipanggil %d kali", calls)
	}
}

func TestRateFallsBackToFixedValue(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"status 500": func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) },
		"bukan JSON": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`halo`)) },
		"nol":        func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"solana":{"idr":0}}`)) },
		"kebesaran":  func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"solana":{"idr":1e13}}`)) },
	} {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(handler)
			defer server.Close()
			m := NewMarketRate(server.URL, 2500000, time.Minute, cache.NewMemory(now), now)
			r, err := m.Current(context.Background())
			if err != nil || r.IDRPerSOL != 2500000 {
				t.Fatalf("kurs %+v, error %v", r, err)
			}
			m = NewMarketRate(server.URL, 0, time.Minute, cache.NewMemory(now), now)
			if _, err := m.Current(context.Background()); !apperr.Is(err, apperr.RateUnavailable) {
				t.Fatalf("seharusnya RATE_UNAVAILABLE: %v", err)
			}
		})
	}
}
