//go:build devnet

package payment_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
	"github.com/mr-tron/base58"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

const (
	systemProgram = "11111111111111111111111111111111"
	storeFunding  = 50_000_000
)

type devnet struct {
	t   *testing.T
	url string
}

func (d devnet) call(method string, params []any, out any) error {
	d.t.Helper()
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	if err != nil {
		return err
	}
	res, err := http.Post(d.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return errors.New(res.Status)
	}
	var reply struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&reply); err != nil {
		return err
	}
	if reply.Error != nil {
		return errors.New(reply.Error.Message)
	}
	return json.Unmarshal(reply.Result, out)
}

// transfer mengisi wallet lain dari wallet yang sudah punya saldo.
func (d devnet) transfer(client solana.Client, from testkit.Wallet, to string, lamports uint64) {
	d.t.Helper()
	ctx := context.Background()
	blockhash, err := client.LatestBlockhash(ctx)
	if err != nil {
		d.t.Fatal(err)
	}
	reference, err := solana.NewReference()
	if err != nil {
		d.t.Fatal(err)
	}
	encoded, err := solana.BuildTransfers(from.Address, blockhash, reference, []solana.Transfer{{From: from.Address, To: to, Lamports: lamports}})
	if err != nil {
		d.t.Fatal(err)
	}
	d.send(from, encoded)
	d.waitFor(func() bool {
		balance, err := client.Balance(ctx, to)
		return err == nil && balance >= lamports
	}, "saldo masuk")
}

func (d devnet) waitFor(done func() bool, what string) {
	d.t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		if done() {
			return
		}
		time.Sleep(2 * time.Second)
	}
	d.t.Fatalf("menunggu %s melewati batas waktu", what)
}

// send menandatangani transaksi dari server dengan satu signer, lalu mengirimnya ke jaringan.
func (d devnet) send(w testkit.Wallet, encoded string) {
	d.t.Helper()
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		d.t.Fatal(err)
	}
	if raw[0] != 1 {
		d.t.Fatalf("jumlah signer %d", raw[0])
	}
	signature, err := base58.Decode(w.Sign(string(raw[65:])))
	if err != nil {
		d.t.Fatal(err)
	}
	copy(raw[1:65], signature)
	var sent string
	params := []any{base64.StdEncoding.EncodeToString(raw), map[string]any{"encoding": "base64"}}
	if err := d.call("sendTransaction", params, &sent); err != nil {
		d.t.Fatalf("sendTransaction: %v", err)
	}
	d.t.Logf("transaksi %s", solana.ExplorerURL(sent))
}

func devnetClient(t *testing.T) (devnet, *solana.RPC) {
	t.Helper()
	url := os.Getenv("SOLANA_RPC_URL")
	if url == "" {
		url = "https://api.devnet.solana.com"
	}
	return devnet{t: t, url: url}, solana.NewRPC(url)
}

// Membaca transaksi nyata tidak butuh saldo, jadi selalu bisa dijalankan.
func TestDevnetReadsRealNetwork(t *testing.T) {
	ctx := context.Background()
	_, client := devnetClient(t)
	rent, err := client.RentExemptMinimum(ctx)
	if err != nil || rent == 0 {
		t.Fatalf("rent %d: %v", rent, err)
	}
	if _, err := client.LatestBlockhash(ctx); err != nil {
		t.Fatal(err)
	}
	empty := testkit.NewWallet().Address
	if balance, err := client.Balance(ctx, empty); err != nil || balance != 0 {
		t.Fatalf("saldo %d: %v", balance, err)
	}
	if found, err := client.FindSignatures(ctx, empty); err != nil || len(found) != 0 {
		t.Fatalf("signature %v: %v", found, err)
	}
	found, err := client.FindSignatures(ctx, systemProgram)
	if err != nil || len(found) == 0 {
		t.Fatalf("signature %d: %v", len(found), err)
	}
	read := 0
	for _, s := range found {
		if read == 3 {
			break
		}
		time.Sleep(3 * time.Second)
		tx, err := client.GetTransaction(ctx, s.Signature)
		var rpcErr *jsonrpc.RPCError
		if errors.Is(err, solana.ErrMalformed) || (errors.As(err, &rpcErr) && rpcErr.Code == http.StatusTooManyRequests) {
			continue
		}
		if err != nil {
			t.Fatalf("%s: %v", s.Signature, err)
		}
		if tx.Signature != s.Signature || tx.FeePayer == "" || len(tx.AccountKeys) == 0 {
			t.Fatalf("transaksi salah: %+v", tx)
		}
		read++
	}
	t.Logf("%d transaksi nyata terbaca dari %d signature", read, len(found))
	if read == 0 {
		t.Fatal("tidak ada transaksi yang terbaca")
	}
}

// Uji Devnet sungguhan: order, bayar, konfirmasi worker, lalu refund. Faucet RPC publik sudah
// ditutup, jadi test ini butuh wallet Devnet buangan berisi sekitar 0,1 SOL.
func TestDevnetPaymentAndRefund(t *testing.T) {
	secret := os.Getenv("DEVNET_PAYER_SECRET")
	if secret == "" {
		t.Skip("isi DEVNET_PAYER_SECRET dengan kunci wallet Devnet yang punya saldo")
	}
	buyer, err := testkit.WalletFromSecret(secret)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	d, client := devnetClient(t)
	a := testkit.NewAppWith(t, client)
	store := a.NewStore()
	owner := a.NewUser(store.ID, auth.RoleOwner)
	cashier := a.NewUser(store.ID, auth.RoleCashier)
	consignor := a.NewConsignor(store.ID)
	item := a.NewItem(consignor.ConsignorID, 100000, 2000, 5)
	d.transfer(client, buyer, store.Wallet.Address, storeFunding)
	storeBefore, err := client.Balance(ctx, store.Wallet.Address)
	if err != nil {
		t.Fatal(err)
	}

	var o struct {
		ID    string `json:"id"`
		Lines []struct {
			ID string `json:"id"`
		} `json:"lines"`
	}
	begin := time.Now()
	a.PostKey(cashier, "/api/v1/orders", "kunci-devnet-order", map[string]any{
		"lines": []map[string]any{{"item_id": item, "quantity": 1}},
	}).Want(201).JSON(&o)
	t.Logf("checkout memakai RPC asli: %s", time.Since(begin))

	var pay struct {
		Transaction string `json:"transaction"`
	}
	a.Public("POST", "/api/v1/solana-pay/orders/"+o.ID, map[string]any{"account": buyer.Address}).Want(200).JSON(&pay)
	d.send(buyer, pay.Transaction)

	d.waitFor(func() bool {
		a.Tick()
		var view struct {
			Status string `json:"status"`
		}
		a.Get(cashier, "/api/v1/orders/"+o.ID).Want(200).JSON(&view)
		return view.Status == "confirmed"
	}, "order confirmed")

	share, err := client.Balance(ctx, consignor.Wallet.Address)
	if err != nil {
		t.Fatal(err)
	}
	if share != 32_000_000 {
		t.Fatalf("bagian penitip %d", share)
	}
	storeAfter, err := client.Balance(ctx, store.Wallet.Address)
	if err != nil {
		t.Fatal(err)
	}
	if storeAfter-storeBefore != 8_000_000 {
		t.Fatalf("komisi toko %d", storeAfter-storeBefore)
	}

	var draft struct {
		Transaction string `json:"transaction"`
	}
	a.PostKey(owner, "/api/v1/orders/"+o.ID+"/refunds", "kunci-devnet-refund", map[string]any{
		"order_line_id": o.Lines[0].ID,
	}).Want(201).JSON(&draft)
	d.send(store.Wallet, draft.Transaction)
	d.waitFor(func() bool {
		a.Tick()
		var n int
		a.Scalar(&n, `SELECT count(*) FROM refunds WHERE status = 'completed'`)
		return n == 1
	}, "refund completed")
}
