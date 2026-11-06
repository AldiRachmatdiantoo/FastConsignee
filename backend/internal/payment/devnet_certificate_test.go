//go:build devnet

package payment_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"strings"
	"testing"
	"time"

	sol "github.com/gagliardetto/solana-go"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/config"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

// fundedAccount memilih akun validator sebagai pembayar simulasi. Simulasi tidak mengubah jaringan,
// jadi uji ini tidak butuh SOL atau faucet.
func fundedAccount(t *testing.T, d devnet) sol.PublicKey {
	t.Helper()
	var votes struct {
		Current []struct {
			NodePubkey string `json:"nodePubkey"`
		} `json:"current"`
	}
	if err := d.call("getVoteAccounts", nil, &votes); err != nil {
		t.Fatal(err)
	}
	if len(votes.Current) == 0 {
		t.Fatal("tidak ada validator aktif")
	}
	return sol.MustPublicKeyFromBase58(votes.Current[0].NodePubkey)
}

// simulate menjalankan transaksi di program asli, lalu menjawab log dan biaya yang ditanggung pembayar.
func simulate(t *testing.T, d devnet, client *solana.RPC, encoded string, payer sol.PublicKey) (string, uint64) {
	t.Helper()
	before, err := client.Balance(context.Background(), payer.String())
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Value struct {
			Err      any      `json:"err"`
			Logs     []string `json:"logs"`
			Accounts []struct {
				Lamports uint64 `json:"lamports"`
			} `json:"accounts"`
		} `json:"value"`
	}
	options := map[string]any{
		"encoding": "base64", "sigVerify": false, "replaceRecentBlockhash": true, "commitment": "confirmed",
		"accounts": map[string]any{"addresses": []string{payer.String()}, "encoding": "base64"},
	}
	if err := d.call("simulateTransaction", []any{encoded, options}, &out); err != nil {
		t.Fatal(err)
	}
	logs := strings.Join(out.Value.Logs, "\n")
	if out.Value.Err != nil {
		t.Fatalf("simulasi gagal: %v\n%s", out.Value.Err, logs)
	}
	if len(out.Value.Accounts) != 1 {
		t.Fatalf("saldo pembayar tidak terbaca: %+v", out.Value.Accounts)
	}
	return logs, before - out.Value.Accounts[0].Lamports
}

func TestDevnetCertificateSimulates(t *testing.T) {
	d, client := devnetClient(t)
	ctx := context.Background()
	funded := fundedAccount(t, d)

	signer, err := sol.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	blockhash, err := client.LatestBlockhash(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rent, err := client.RentExemptFor(ctx, solana.MintSize)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := solana.BuildCertificate(solana.CertificateMint{
		Signer: signer, Mint: solana.DeriveMint(signer, "simulasi"), Owner: testkit.NewWallet().Address,
		Item: "Kain Batik Tulis", Blockhash: blockhash, Rent: rent,
		URI: "https://fastconsignee.test/api/v1/public/certificates/00000000-0000-4000-8000-000000000001/metadata",
	})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := sol.TransactionFromBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	// Otoritas mint juga tersimpan di data instruksi, jadi ikut diganti.
	tx.Message.AccountKeys[0] = funded
	for i := range tx.Message.Instructions {
		data := tx.Message.Instructions[i].Data
		tx.Message.Instructions[i].Data = bytes.ReplaceAll(data, signer.PublicKey().Bytes(), funded.Bytes())
	}
	swapped, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	logs, cost := simulate(t, d, client, base64.StdEncoding.EncodeToString(swapped), funded)
	if !strings.Contains(logs, sol.TokenMetadataProgramID.String()+" success") {
		t.Fatalf("program metadata tidak berjalan:\n%s", logs)
	}
	t.Logf("%d byte, biaya %d lamport", len(raw), cost)
}

// TestDevnetCertificateMints membayar order bersertifikat, lalu memastikan NFT-nya sampai di wallet
// pembeli. Wallet buangan yang sama membayar order dan menerbitkan sertifikat.
func TestDevnetCertificateMints(t *testing.T) {
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
	a := testkit.NewAppWith(t, client, func(c *config.Config) { c.CertificateSigner = secret })
	store := a.NewStore()
	cashier := a.NewUser(store.ID, auth.RoleCashier)
	consignor := a.NewConsignor(store.ID)
	item := a.NewItem(consignor.ConsignorID, 100000, 2000, 5)
	a.Exec(`UPDATE items SET certificate_enabled = true WHERE id = $1`, item)
	d.transfer(client, buyer, store.Wallet.Address, storeFunding)

	var o struct {
		ID string `json:"id"`
	}
	a.PostKey(cashier, "/api/v1/orders", "kunci-devnet-sertifikat", map[string]any{
		"lines": []map[string]any{{"item_id": item, "quantity": 1}},
	}).Want(201).JSON(&o)
	var pay struct {
		Transaction string `json:"transaction"`
	}
	a.Public("POST", "/api/v1/solana-pay/orders/"+o.ID, map[string]any{"account": buyer.Address}).Want(200).JSON(&pay)
	d.send(buyer, pay.Transaction)
	d.waitFor(func() bool {
		a.Tick()
		var n int
		a.Scalar(&n, `SELECT count(*) FROM certificates`)
		return n == 1
	}, "order confirmed")

	// Jam uji berbeda dari now() database, jadi waktu antrean disamakan dengan jam uji.
	a.Exec(`UPDATE certificates SET created_at = $1, updated_at = $1`, a.Clock.Now())
	a.Clock.Advance(time.Second)
	d.waitFor(func() bool {
		if err := a.API.Certificates.Tick(ctx); err != nil {
			t.Fatal(err)
		}
		var status string
		a.Scalar(&status, `SELECT status FROM certificates`)
		return status == "sent"
	}, "sertifikat terkirim")

	var address string
	a.Scalar(&address, `SELECT mint_address FROM certificates`)
	mint := sol.MustPublicKeyFromBase58(address)
	tokenAccount, _, err := sol.FindAssociatedTokenAddress(sol.MustPublicKeyFromBase58(buyer.Address), mint)
	if err != nil {
		t.Fatal(err)
	}
	var balance struct {
		Value struct {
			Amount string `json:"amount"`
		} `json:"value"`
	}
	params := []any{tokenAccount.String(), map[string]any{"commitment": "confirmed"}}
	if err := d.call("getTokenAccountBalance", params, &balance); err != nil {
		t.Fatal(err)
	}
	if balance.Value.Amount != "1" {
		t.Fatalf("saldo NFT pembeli %q", balance.Value.Amount)
	}
	metadata, _, err := sol.FindTokenMetadataAddress(mint)
	if err != nil {
		t.Fatal(err)
	}
	exists, err := client.AccountExists(ctx, metadata.String())
	if err != nil || !exists {
		t.Fatalf("akun metadata tidak ada: %v", err)
	}
	t.Logf("NFT %s", solana.ExplorerAddressURL(address))
}

func TestDevnetPaymentSimulates(t *testing.T) {
	d, client := devnetClient(t)
	funded := fundedAccount(t, d)
	reference, err := solana.NewReference()
	if err != nil {
		t.Fatal(err)
	}
	blockhash, err := client.LatestBlockhash(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	transfers := []solana.Transfer{
		{To: testkit.NewWallet().Address, Lamports: 20_000_000},
		{To: testkit.NewWallet().Address, Lamports: 5_000_000},
	}
	encoded, err := solana.BuildTransfers(funded.String(), blockhash, reference, transfers)
	if err != nil {
		t.Fatal(err)
	}
	_, cost := simulate(t, d, client, encoded, funded)
	if cost < 25_000_000 {
		t.Fatalf("biaya %d lamport lebih kecil dari jumlah transfer", cost)
	}
	t.Logf("biaya %d lamport", cost)
}
