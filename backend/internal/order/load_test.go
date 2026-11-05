//go:build load

package order_test

import (
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

// Uji NFR-02: QR muncul paling lama 3 detik. Semua order merebut baris barang yang sama, jadi ini
// kasus terburuk untuk kunci database. RPC Solana palsu, jadi latensi jaringan tidak terhitung.
func TestCheckoutLatencyUnderLoad(t *testing.T) {
	const (
		workers   = 20
		perWorker = 50
		limit     = 3 * time.Second
	)
	a := testkit.NewApp(t)
	store := a.NewStore()
	cashier := a.NewUser(store.ID, auth.RoleCashier)
	consignor := a.NewConsignor(store.ID)
	item := a.NewItem(consignor.ConsignorID, 100000, 2000, 1000000)
	body := map[string]any{"lines": []map[string]any{{"item_id": item, "quantity": 1}}}

	var (
		mu        sync.Mutex
		latencies []time.Duration
		failures  int
		wg        sync.WaitGroup
	)
	started := time.Now()
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range perWorker {
				begin := time.Now()
				res := a.PostKey(cashier, "/api/v1/orders", fmt.Sprintf("kunci-beban-%d-%d", w, i), body)
				took := time.Since(begin)
				mu.Lock()
				latencies = append(latencies, took)
				if res.Status != 201 {
					failures++
				}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(started)

	slices.Sort(latencies)
	at := func(p float64) time.Duration { return latencies[int(float64(len(latencies)-1)*p)] }
	t.Logf("%d order, %d pekerja, %.0f order/detik", len(latencies), workers, float64(len(latencies))/elapsed.Seconds())
	t.Logf("p50 %s, p95 %s, p99 %s, maks %s", at(0.50), at(0.95), at(0.99), at(1))
	if failures != 0 {
		t.Fatalf("%d order gagal", failures)
	}
	if at(0.95) > limit {
		t.Fatalf("p95 %s melewati %s", at(0.95), limit)
	}
	var reserved int
	a.Scalar(&reserved, `SELECT count(*) FROM orders WHERE status = 'pending'`)
	if reserved != workers*perWorker {
		t.Fatalf("order tersimpan %d", reserved)
	}
}
