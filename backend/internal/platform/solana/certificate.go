package solana

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf8"

	sol "github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
)

const (
	// MintSize adalah ukuran akun mint SPL Token.
	MintSize = 82
	// CertificateSymbol adalah simbol NFT sertifikat.
	CertificateSymbol = "FCERT"

	certificatePrefix = "Sertifikat "
	maxNameBytes      = 32
	maxURIBytes       = 200
	createMetadataV3  = 33
)

// Signer adalah kunci wallet yang membayar dan menandatangani penerbitan sertifikat.
type Signer = sol.PrivateKey

// ParseSigner membaca kunci base58. Nilai kosong berarti pengiriman sertifikat mati.
func ParseSigner(secret string) (Signer, error) {
	if secret == "" {
		return nil, nil
	}
	key, err := sol.PrivateKeyFromBase58(secret)
	if err != nil || len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("kunci sertifikat tidak valid")
	}
	return key, nil
}

// DeriveMint menurunkan kunci mint dari kunci penanda tangan dan ID sertifikat. Hasilnya selalu
// sama, jadi pengiriman ulang tidak bisa menerbitkan NFT kedua.
func DeriveMint(signer Signer, certificateID string) Signer {
	mac := hmac.New(sha256.New, signer)
	mac.Write([]byte("fastconsignee-certificate:" + certificateID))
	return Signer(ed25519.NewKeyFromSeed(mac.Sum(nil)))
}

// CertificateName menyusun nama NFT. Token Metadata membatasi nama sampai 32 byte.
func CertificateName(item string) string {
	name := certificatePrefix + item
	for len(name) > maxNameBytes {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name
}

// CertificateMint adalah isi satu penerbitan sertifikat.
type CertificateMint struct {
	Signer    Signer
	Mint      Signer
	Owner     string
	Item      string
	URI       string
	Blockhash string
	Rent      uint64
}

// BuildCertificate menyusun NFT dalam satu transaksi: mint baru, akun token pembeli, satu token,
// metadata, lalu otoritas mint dicabut supaya jumlahnya tetap satu.
func BuildCertificate(c CertificateMint) ([]byte, error) {
	if len(c.URI) > maxURIBytes {
		return nil, errors.New("alamat metadata terlalu panjang")
	}
	owner, err := sol.PublicKeyFromBase58(c.Owner)
	if err != nil {
		return nil, fmt.Errorf("alamat pembeli: %w", err)
	}
	hash, err := sol.HashFromBase58(c.Blockhash)
	if err != nil {
		return nil, fmt.Errorf("blockhash: %w", err)
	}
	payer, mint := c.Signer.PublicKey(), c.Mint.PublicKey()
	tokenAccount, _, err := sol.FindAssociatedTokenAddress(owner, mint)
	if err != nil {
		return nil, fmt.Errorf("akun token: %w", err)
	}
	metadata, _, err := sol.FindTokenMetadataAddress(mint)
	if err != nil {
		return nil, fmt.Errorf("akun metadata: %w", err)
	}

	instructions := make([]sol.Instruction, 0, 6)
	add := func(ix sol.Instruction, err error) error {
		if err != nil {
			return err
		}
		instructions = append(instructions, ix)
		return nil
	}
	createMint, err := system.NewCreateAccountInstruction(c.Rent, MintSize, sol.TokenProgramID, payer, mint).ValidateAndBuild()
	if err := add(createMint, err); err != nil {
		return nil, fmt.Errorf("buat mint: %w", err)
	}
	initMint, err := token.NewInitializeMint2InstructionBuilder().
		SetDecimals(0).SetMintAuthority(payer).SetMintAccount(mint).ValidateAndBuild()
	if err := add(initMint, err); err != nil {
		return nil, fmt.Errorf("siapkan mint: %w", err)
	}
	createAccount, err := associatedtokenaccount.NewCreateInstruction(payer, owner, mint).ValidateAndBuild()
	if err := add(createAccount, err); err != nil {
		return nil, fmt.Errorf("buat akun token: %w", err)
	}
	mintOne, err := token.NewMintToInstruction(1, mint, tokenAccount, payer, nil).ValidateAndBuild()
	if err := add(mintOne, err); err != nil {
		return nil, fmt.Errorf("cetak token: %w", err)
	}
	instructions = append(instructions, createMetadata(metadata, mint, payer, CertificateName(c.Item), c.URI))
	lock, err := token.NewSetAuthorityInstructionBuilder().
		SetAuthorityType(token.AuthorityMintTokens).SetSubjectAccount(mint).SetAuthorityAccount(payer).ValidateAndBuild()
	if err := add(lock, err); err != nil {
		return nil, fmt.Errorf("cabut otoritas mint: %w", err)
	}

	tx, err := sol.NewTransaction(instructions, hash, sol.TransactionPayer(payer))
	if err != nil {
		return nil, fmt.Errorf("susun transaksi: %w", err)
	}
	signers := map[sol.PublicKey]*sol.PrivateKey{payer: &c.Signer, mint: &c.Mint}
	if _, err := tx.Sign(func(key sol.PublicKey) *sol.PrivateKey { return signers[key] }); err != nil {
		return nil, fmt.Errorf("tanda tangani: %w", err)
	}
	raw, err := tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("serialisasi transaksi: %w", err)
	}
	return raw, nil
}

// createMetadata menyusun instruksi CreateMetadataAccountV3. solana-go tidak menyediakannya, jadi
// datanya ditulis langsung dalam format Borsh.
func createMetadata(metadata, mint, authority sol.PublicKey, name, uri string) sol.Instruction {
	var data bytes.Buffer
	data.WriteByte(createMetadataV3)
	writeBorshString(&data, name)
	writeBorshString(&data, CertificateSymbol)
	writeBorshString(&data, uri)
	data.Write([]byte{
		0, 0, // seller_fee_basis_points
		0, // creators: None
		0, // collection: None
		0, // uses: None
		0, // is_mutable: false, jadi isi sertifikat tidak bisa diubah
		0, // collection_details: None
	})
	accounts := sol.AccountMetaSlice{
		sol.Meta(metadata).WRITE(),
		sol.Meta(mint),
		sol.Meta(authority).SIGNER(),
		sol.Meta(authority).SIGNER().WRITE(),
		sol.Meta(authority).SIGNER(),
		sol.Meta(sol.SystemProgramID),
		sol.Meta(sol.SysVarRentPubkey),
	}
	return sol.NewInstruction(sol.TokenMetadataProgramID, accounts, data.Bytes())
}

func writeBorshString(buffer *bytes.Buffer, value string) {
	var size [4]byte
	binary.LittleEndian.PutUint32(size[:], uint32(len(value))) // #nosec G115 -- panjangnya dibatasi di atas
	buffer.Write(size[:])
	buffer.WriteString(value)
}
