package payment_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	sol "github.com/gagliardetto/solana-go"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/config"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/testkit"
)

type certificateRow struct {
	ID     string
	Status string
	Mint   *string
}

func newSigner(t *testing.T) (sol.PrivateKey, testkit.Option) {
	t.Helper()
	key, err := sol.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key, func(c *config.Config) { c.CertificateSigner = key.String() }
}

// paidCertificate membuat order lunas dengan satu barang bersertifikat.
func paidCertificate(t *testing.T, f *fixture) certificateRow {
	t.Helper()
	f.a.Exec(`UPDATE items SET certificate_enabled = true WHERE id = $1`, f.bag)
	f.pay(t, f.order(t, f.bag))
	f.a.Tick()
	// Jam uji berbeda dari now() database, jadi waktu antrean disamakan dengan jam uji.
	now := f.a.Clock.Now()
	f.a.Exec(`UPDATE certificates SET created_at = $1, updated_at = $1`, now)
	f.a.Clock.Advance(time.Second)
	return readCertificate(t, f)
}

func readCertificate(t *testing.T, f *fixture) certificateRow {
	t.Helper()
	var row certificateRow
	err := f.a.DB.Pool.QueryRow(context.Background(), `SELECT id::text, status, mint_address FROM certificates`).
		Scan(&row.ID, &row.Status, &row.Mint)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func tickCertificates(t *testing.T, f *fixture) {
	t.Helper()
	if err := f.a.API.Certificates.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCertificateIsMintedToBuyer(t *testing.T) {
	key, option := newSigner(t)
	f := setup(t, option)
	row := paidCertificate(t, f)
	mint := solana.DeriveMint(key, row.ID).PublicKey()

	tickCertificates(t, f)
	sent := f.a.Solana.SentTransactions()
	if len(sent) != 1 {
		t.Fatalf("terkirim %d transaksi", len(sent))
	}
	tx := decodeTransaction(t, sent[0])
	if err := tx.VerifySignatures(); err != nil {
		t.Fatalf("tanda tangan: %v", err)
	}
	if tx.Message.AccountKeys[0] != key.PublicKey() || tx.Message.AccountKeys[1] != mint {
		t.Fatalf("penanda tangan salah: %v", tx.Message.AccountKeys[:2])
	}
	want := []sol.PublicKey{
		sol.SystemProgramID, sol.TokenProgramID, sol.SPLAssociatedTokenAccountProgramID,
		sol.TokenProgramID, sol.TokenMetadataProgramID, sol.TokenProgramID,
	}
	if len(tx.Message.Instructions) != len(want) {
		t.Fatalf("jumlah instruksi %d", len(tx.Message.Instructions))
	}
	for i, program := range want {
		got, err := tx.ResolveProgramIDIndex(tx.Message.Instructions[i].ProgramIDIndex)
		if err != nil || got != program {
			t.Fatalf("instruksi %d memakai %v: %v", i, got, err)
		}
	}
	owner := sol.MustPublicKeyFromBase58(f.buyer.Address)
	tokenAccount, _, err := sol.FindAssociatedTokenAddress(owner, mint)
	if err != nil {
		t.Fatal(err)
	}
	if !hasAccount(tx, tokenAccount) || !hasAccount(tx, owner) {
		t.Fatal("akun token pembeli tidak ada di transaksi")
	}

	tickCertificates(t, f)
	row = readCertificate(t, f)
	if row.Status != "sent" || row.Mint == nil || *row.Mint != mint.String() {
		t.Fatalf("sertifikat %+v", row)
	}
	if n := len(f.a.Solana.SentTransactions()); n != 1 {
		t.Fatalf("transaksi terkirim %d kali", n)
	}
}

func TestCertificateShowsInPublicOrder(t *testing.T) {
	_, option := newSigner(t)
	f := setup(t, option)
	f.a.Exec(`UPDATE items SET certificate_enabled = true WHERE id = $1`, f.bag)
	o := f.order(t, f.bag)
	f.pay(t, o)
	f.a.Tick()
	now := f.a.Clock.Now()
	f.a.Exec(`UPDATE certificates SET created_at = $1, updated_at = $1`, now)
	tickCertificates(t, f)
	tickCertificates(t, f)

	var body struct {
		Lines []struct {
			Certificate struct {
				Status      string  `json:"status"`
				ExplorerURL *string `json:"explorer_url"`
			} `json:"certificate"`
		} `json:"lines"`
	}
	f.a.Do(testkit.Request{Method: "GET", Path: "/api/v1/public/orders/" + o.ID}).Want(200).JSON(&body)
	line := body.Lines[0].Certificate
	if line.Status != "sent" || line.ExplorerURL == nil || !strings.Contains(*line.ExplorerURL, "cluster=devnet") {
		t.Fatalf("sertifikat di bukti pembayaran: %+v", line)
	}
}

func TestCertificateRetryKeepsTheSameMint(t *testing.T) {
	key, option := newSigner(t)
	f := setup(t, option)
	row := paidCertificate(t, f)
	f.a.Solana.DropTransactions(true)

	tickCertificates(t, f)
	tickCertificates(t, f)
	if n := len(f.a.Solana.SentTransactions()); n != 1 {
		t.Fatalf("transaksi yang masih berjalan dikirim ulang %d kali", n)
	}

	f.a.Clock.Advance(31 * time.Second)
	tickCertificates(t, f)
	sent := f.a.Solana.SentTransactions()
	if len(sent) != 2 {
		t.Fatalf("seharusnya dicoba lagi setelah 30 detik: %d", len(sent))
	}
	mint := solana.DeriveMint(key, row.ID).PublicKey()
	for _, raw := range sent {
		if decodeTransaction(t, raw).Message.AccountKeys[1] != mint {
			t.Fatal("pengiriman ulang memakai mint lain")
		}
	}
}

func TestCertificateFailsAfterDeadline(t *testing.T) {
	_, option := newSigner(t)
	f := setup(t, option)
	paidCertificate(t, f)
	f.a.Solana.SetSendErr(errors.New("saldo habis"))

	tickCertificates(t, f)
	if row := readCertificate(t, f); row.Status != "pending" {
		t.Fatalf("status %s sebelum batas waktu", row.Status)
	}
	f.a.Clock.Advance(2 * time.Hour)
	tickCertificates(t, f)
	if row := readCertificate(t, f); row.Status != "failed" || row.Mint != nil {
		t.Fatalf("sertifikat %+v", row)
	}
}

func TestCertificateWithoutSignerNeverSends(t *testing.T) {
	f := setup(t)
	paidCertificate(t, f)

	tickCertificates(t, f)
	if n := len(f.a.Solana.SentTransactions()); n != 0 {
		t.Fatalf("terkirim %d transaksi tanpa signer", n)
	}
	if row := readCertificate(t, f); row.Status != "pending" {
		t.Fatalf("status %s", row.Status)
	}
	f.a.Clock.Advance(2 * time.Hour)
	tickCertificates(t, f)
	if row := readCertificate(t, f); row.Status != "failed" {
		t.Fatalf("status %s setelah batas waktu", row.Status)
	}
}

func TestCertificateMetadataIsPublicAndHidesPrivateData(t *testing.T) {
	f := setup(t)
	row := paidCertificate(t, f)

	r := f.a.Do(testkit.Request{Method: "GET", Path: "/api/v1/public/certificates/" + row.ID + "/metadata",
		Headers: map[string]string{"Origin": "https://explorer.solana.com"}}).Want(200)
	if r.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("Explorer harus bisa membaca metadata: %q", r.Header.Get("Access-Control-Allow-Origin"))
	}
	var body struct {
		Name       string `json:"name"`
		Symbol     string `json:"symbol"`
		Image      string `json:"image"`
		Attributes []struct {
			TraitType string `json:"trait_type"`
			Value     string `json:"value"`
		} `json:"attributes"`
	}
	r.JSON(&body)
	if !strings.HasPrefix(body.Name, "Sertifikat ") || body.Symbol != "FCERT" || body.Image == "" || len(body.Attributes) != 4 {
		t.Fatalf("metadata %+v", body)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{f.buyer.Address, f.sari.Wallet.Address, f.sari.Email} {
		if strings.Contains(string(raw), private) {
			t.Fatalf("metadata membocorkan %s", private)
		}
	}
}

func TestCertificateMetadataRejectsBadID(t *testing.T) {
	f := setup(t)
	f.a.Do(testkit.Request{Method: "GET", Path: "/api/v1/public/certificates/bukan-uuid/metadata"}).WantProblem(400, "VALIDATION_FAILED")
	f.a.Do(testkit.Request{Method: "GET", Path: "/api/v1/public/certificates/00000000-0000-4000-8000-000000000001/metadata"}).WantProblem(404, "NOT_FOUND")
}

func TestCertificateTransactionFitsOnePacket(t *testing.T) {
	signer, err := sol.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := solana.BuildCertificate(solana.CertificateMint{
		Signer: signer, Mint: solana.DeriveMint(signer, "sertifikat"), Owner: testkit.NewWallet().Address,
		Item: strings.Repeat("é", 100), URI: "https://" + strings.Repeat("a", 192), Blockhash: testkit.Blockhash, Rent: 1461600,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 1232 {
		t.Fatalf("transaksi %d byte, batas jaringan 1232", len(raw))
	}
	_, err = solana.BuildCertificate(solana.CertificateMint{
		Signer: signer, Mint: signer, Owner: testkit.NewWallet().Address, URI: strings.Repeat("a", 201), Blockhash: testkit.Blockhash,
	})
	if err == nil {
		t.Fatal("URI lebih dari 200 byte seharusnya ditolak")
	}
}

func TestCertificateMintIsStable(t *testing.T) {
	signer, err := sol.NewRandomPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	first := solana.DeriveMint(signer, "a").PublicKey()
	if first != solana.DeriveMint(signer, "a").PublicKey() {
		t.Fatal("mint berubah untuk sertifikat yang sama")
	}
	if first == solana.DeriveMint(signer, "b").PublicKey() {
		t.Fatal("dua sertifikat memakai mint yang sama")
	}
}

func TestCertificateNameFitsMetadataLimit(t *testing.T) {
	for _, item := range []string{"Tas", strings.Repeat("a", 60), strings.Repeat("é", 40)} {
		name := solana.CertificateName(item)
		if len(name) > 32 || !strings.HasPrefix(name, "Sertifikat ") || strings.ContainsRune(name, 0xFFFD) {
			t.Fatalf("nama %q, %d byte", name, len(name))
		}
	}
}

func TestSignerSecretIsValidated(t *testing.T) {
	if key, err := solana.ParseSigner(""); err != nil || key != nil {
		t.Fatalf("kunci kosong: %v %v", key, err)
	}
	for _, bad := range []string{"bukan-kunci", "111"} {
		_, err := solana.ParseSigner(bad)
		if err == nil {
			t.Fatalf("kunci %q seharusnya ditolak", bad)
		}
		if strings.Contains(err.Error(), bad) {
			t.Fatal("pesan error memuat kunci")
		}
	}
}

func decodeTransaction(t *testing.T, raw []byte) *sol.Transaction {
	t.Helper()
	tx, err := sol.TransactionFromBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	return tx
}

func hasAccount(tx *sol.Transaction, key sol.PublicKey) bool {
	for _, account := range tx.Message.AccountKeys {
		if account == key {
			return true
		}
	}
	return false
}
