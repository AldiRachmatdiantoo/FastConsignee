package auth_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
)

const (
	jwksIssuer   = "https://proyek.supabase.co/auth/v1"
	jwksAudience = "authenticated"
)

type jwksFixture struct {
	verifier auth.Verifier
	ec       *ecdsa.PrivateKey
	rsa      *rsa.PrivateKey
}

func b64(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

// setupJWKS menyajikan kunci publik seperti endpoint JWKS Supabase.
func setupJWKS(t *testing.T) jwksFixture {
	t.Helper()
	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]any{"keys": []map[string]any{
		{
			"kty": "EC", "crv": "P-256", "kid": "ec1", "alg": "ES256", "use": "sig",
			"x": b64(ecKey.X.FillBytes(make([]byte, 32))), "y": b64(ecKey.Y.FillBytes(make([]byte, 32))),
		},
		{
			"kty": "RSA", "kid": "rsa1", "alg": "RS256", "use": "sig",
			"n": b64(rsaKey.N.Bytes()), "e": b64(big.NewInt(int64(rsaKey.E)).Bytes()),
		},
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(keys)
	}))
	t.Cleanup(server.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	verifier, err := auth.NewJWKSVerifier(ctx, server.URL, jwksIssuer, jwksAudience)
	if err != nil {
		t.Fatal(err)
	}
	return jwksFixture{verifier: verifier, ec: ecKey, rsa: rsaKey}
}

func jwksClaims() jwt.MapClaims {
	return jwt.MapClaims{
		"sub": "pengguna-1", "email": "Sari@Example.com", "iss": jwksIssuer, "aud": jwksAudience,
		"exp":           time.Now().Add(time.Hour).Unix(),
		"user_metadata": map[string]any{"email_verified": true},
	}
}

func signed(t *testing.T, method jwt.SigningMethod, kid string, key any, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(method, claims)
	token.Header["kid"] = kid
	out, err := token.SignedString(key)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestJWKSAcceptsSupabaseKeys(t *testing.T) {
	f := setupJWKS(t)
	for name, token := range map[string]string{
		"ES256": signed(t, jwt.SigningMethodES256, "ec1", f.ec, jwksClaims()),
		"RS256": signed(t, jwt.SigningMethodRS256, "rsa1", f.rsa, jwksClaims()),
	} {
		t.Run(name, func(t *testing.T) {
			id, err := f.verifier.Verify(context.Background(), token)
			if err != nil {
				t.Fatal(err)
			}
			if id.Subject != "pengguna-1" || id.Email != "sari@example.com" || !id.EmailVerified {
				t.Fatalf("identitas %+v", id)
			}
		})
	}
}

func TestJWKSRejectsForgedTokens(t *testing.T) {
	f := setupJWKS(t)
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	expired := jwksClaims()
	expired["exp"] = time.Now().Add(-time.Minute).Unix()
	wrongIssuer := jwksClaims()
	wrongIssuer["iss"] = "https://jahat.test/auth/v1"
	wrongAudience := jwksClaims()
	wrongAudience["aud"] = "lain"

	for name, token := range map[string]string{
		"kunci lain dengan kid sama": signed(t, jwt.SigningMethodES256, "ec1", other, jwksClaims()),
		"kid tidak dikenal":          signed(t, jwt.SigningMethodES256, "tidak-ada", f.ec, jwksClaims()),
		"HS256":                      signed(t, jwt.SigningMethodHS256, "ec1", []byte("rahasia-lama"), jwksClaims()),
		"kedaluwarsa":                signed(t, jwt.SigningMethodES256, "ec1", f.ec, expired),
		"issuer salah":               signed(t, jwt.SigningMethodES256, "ec1", f.ec, wrongIssuer),
		"audience salah":             signed(t, jwt.SigningMethodES256, "ec1", f.ec, wrongAudience),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := f.verifier.Verify(context.Background(), token); !errors.Is(err, auth.ErrInvalidToken) {
				t.Fatalf("seharusnya ditolak: %v", err)
			}
		})
	}
}
