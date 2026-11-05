package solana

import (
	"encoding/base64"
	"errors"
	"testing"

	sol "github.com/gagliardetto/solana-go"
)

const blockhash = "4vJ9JU1bJJE96FWSJKvHsmmFADCg4gpZQff4P3bkLKi"

func address(t *testing.T) string {
	t.Helper()
	a, err := NewReference()
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func build(t *testing.T, payer, reference string, transfers []Transfer) (*sol.Transaction, string) {
	t.Helper()
	encoded, err := BuildTransfers(payer, blockhash, reference, transfers)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := sol.TransactionFromBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	return tx, encoded
}

func TestBuildTransfersRoundTrip(t *testing.T) {
	payer, reference := address(t), address(t)
	want := []Transfer{
		{From: payer, To: address(t), Lamports: 32000000},
		{From: payer, To: address(t), Lamports: 34000000},
		{From: payer, To: address(t), Lamports: 14000000},
	}
	_, encoded := build(t, payer, reference, want)
	got, size, err := DecodeBase64(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if got.FeePayer != payer {
		t.Fatalf("fee payer %s", got.FeePayer)
	}
	if len(got.Transfers) != len(want) {
		t.Fatalf("transfer %d", len(got.Transfers))
	}
	for i := range want {
		if got.Transfers[i] != want[i] {
			t.Fatalf("transfer %d: %+v", i, got.Transfers[i])
		}
	}
	if !Matches(got, reference, payer, want) {
		t.Fatal("transaksi seharusnya cocok")
	}
	if size > 1232 {
		t.Fatalf("transaksi %d byte melebihi batas paket", size)
	}
}

func TestBuildTransfersLeavesOneEmptySignatureSlot(t *testing.T) {
	payer := address(t)
	tx, _ := build(t, payer, address(t), []Transfer{{From: payer, To: address(t), Lamports: 1}})
	if tx.Message.Header.NumRequiredSignatures != 1 || len(tx.Signatures) != 1 {
		t.Fatalf("penanda tangan %d, slot %d", tx.Message.Header.NumRequiredSignatures, len(tx.Signatures))
	}
	if !tx.Signatures[0].IsZero() {
		t.Fatal("server tidak boleh menandatangani")
	}
	if tx.Message.RecentBlockhash.String() != blockhash {
		t.Fatalf("blockhash %s", tx.Message.RecentBlockhash)
	}
}

func TestReferenceIsReadOnlyAndNotASigner(t *testing.T) {
	payer, reference := address(t), address(t)
	tx, _ := build(t, payer, reference, []Transfer{{From: payer, To: address(t), Lamports: 1}})
	key := sol.MustPublicKeyFromBase58(reference)
	index := -1
	for i, k := range tx.Message.AccountKeys {
		if k.Equals(key) {
			index = i
		}
	}
	if index < 0 {
		t.Fatal("reference tidak ada di transaksi")
	}
	writable, err := tx.Message.IsWritable(key)
	if err != nil {
		t.Fatal(err)
	}
	if writable || tx.Message.IsSigner(key) {
		t.Fatalf("reference harus read-only: writable %v, signer %v", writable, tx.Message.IsSigner(key))
	}
}

func TestFiveConsignorsAndStoreFitInOnePacket(t *testing.T) {
	payer := address(t)
	transfers := make([]Transfer, 6)
	for i := range transfers {
		transfers[i] = Transfer{From: payer, To: address(t), Lamports: uint64(i + 1)}
	}
	encoded, err := BuildTransfers(payer, blockhash, address(t), transfers)
	if err != nil {
		t.Fatal(err)
	}
	_, size, err := DecodeBase64(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if size > 1232 || len(encoded) > 2048 {
		t.Fatalf("transaksi %d byte, base64 %d karakter", size, len(encoded))
	}
}

func TestBuildTransfersRejectsBadInput(t *testing.T) {
	payer, to := address(t), address(t)
	one := []Transfer{{From: payer, To: to, Lamports: 1}}
	cases := map[string]func() (string, error){
		"tanpa transfer": func() (string, error) { return BuildTransfers(payer, blockhash, to, nil) },
		"payer salah":    func() (string, error) { return BuildTransfers("abc", blockhash, to, one) },
		"reference":      func() (string, error) { return BuildTransfers(payer, blockhash, "abc", one) },
		"blockhash":      func() (string, error) { return BuildTransfers(payer, "abc", to, one) },
		"penerima": func() (string, error) {
			return BuildTransfers(payer, blockhash, to, []Transfer{{To: "abc", Lamports: 1}})
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := run(); err == nil {
				t.Fatal("seharusnya ditolak")
			}
		})
	}
}

func TestMatches(t *testing.T) {
	payer, a, b, reference := address(t), address(t), address(t), address(t)
	want := []Transfer{{To: a, Lamports: 10}, {To: b, Lamports: 20}}
	base := func() *Transaction {
		return &Transaction{
			FeePayer:    payer,
			AccountKeys: []string{payer, a, b, reference},
			Transfers:   []Transfer{{From: payer, To: b, Lamports: 20}, {From: payer, To: a, Lamports: 10}},
		}
	}
	if !Matches(base(), reference, "", want) || !Matches(base(), reference, payer, want) {
		t.Fatal("urutan transfer tidak boleh berpengaruh")
	}
	cases := map[string]func(*Transaction){
		"jumlah kurang":   func(x *Transaction) { x.Transfers[0].Lamports = 19 },
		"jumlah lebih":    func(x *Transaction) { x.Transfers[0].Lamports = 21 },
		"penerima lain":   func(x *Transaction) { x.Transfers[0].To = address(t) },
		"pengirim lain":   func(x *Transaction) { x.Transfers[0].From = address(t) },
		"transfer kurang": func(x *Transaction) { x.Transfers = x.Transfers[:1] },
		"transfer lebih": func(x *Transaction) {
			x.Transfers = append(x.Transfers, Transfer{From: payer, To: a, Lamports: 1})
		},
		"transfer ganda":  func(x *Transaction) { x.Transfers[1] = x.Transfers[0] },
		"tanpa reference": func(x *Transaction) { x.AccountKeys = x.AccountKeys[:3] },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			x := base()
			mutate(x)
			if Matches(x, reference, "", want) {
				t.Fatal("seharusnya tidak cocok")
			}
		})
	}
	if Matches(base(), reference, address(t), want) {
		t.Fatal("fee payer lain harus ditolak kalau payer ditentukan")
	}
}

func TestDecodeIgnoresGuardInstructions(t *testing.T) {
	payer, to, reference := address(t), address(t), address(t)
	tx, _ := build(t, payer, reference, []Transfer{{From: payer, To: to, Lamports: 5}})
	tx.Message.AccountKeys = append(tx.Message.AccountKeys, sol.MustPublicKeyFromBase58("ComputeBudget111111111111111111111111111111"))
	tx.Message.Instructions = append(tx.Message.Instructions, sol.CompiledInstruction{
		ProgramIDIndex: uint16(len(tx.Message.AccountKeys) - 1), Data: []byte{2, 0, 0, 0, 0},
	})
	got, err := Decode(tx, "sig", false)
	if err != nil {
		t.Fatal(err)
	}
	if !Matches(got, reference, "", []Transfer{{To: to, Lamports: 5}}) {
		t.Fatalf("instruksi tambahan tidak boleh menggagalkan: %+v", got)
	}
}

func TestDecodeRejectsBrokenIndexes(t *testing.T) {
	payer := address(t)
	tx, _ := build(t, payer, address(t), []Transfer{{From: payer, To: address(t), Lamports: 5}})
	tx.Message.Instructions[0].Accounts[1] = 200
	if _, err := Decode(tx, "sig", false); !errors.Is(err, ErrMalformed) {
		t.Fatalf("seharusnya ErrMalformed: %v", err)
	}
	tx.Message.Instructions[0].ProgramIDIndex = 200
	if _, err := Decode(tx, "sig", false); !errors.Is(err, ErrMalformed) {
		t.Fatalf("seharusnya ErrMalformed: %v", err)
	}
}
