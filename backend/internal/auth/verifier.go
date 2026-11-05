package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
)

// Verifier memverifikasi JWT Supabase.
type Verifier interface {
	Verify(ctx context.Context, token string) (Identity, error)
}

// ErrInvalidToken berarti JWT tidak bisa dipercaya.
var ErrInvalidToken = errors.New("token tidak valid")

type claims struct {
	jwt.RegisteredClaims
	Email        string `json:"email"`
	UserMetadata struct {
		EmailVerified bool `json:"email_verified"`
	} `json:"user_metadata"`
}

// JWTVerifier mengikuti RFC 8725: algoritma dibatasi, dan exp, iss, serta aud selalu diperiksa.
type JWTVerifier struct {
	parser  *jwt.Parser
	keyfunc jwt.Keyfunc
}

func newParser(issuer, audience string, now func() time.Time, methods ...string) *jwt.Parser {
	if now == nil {
		now = time.Now
	}
	return jwt.NewParser(
		jwt.WithValidMethods(methods),
		jwt.WithIssuer(issuer),
		jwt.WithAudience(audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(30*time.Second),
		jwt.WithTimeFunc(now),
	)
}

// NewHMACVerifier memakai JWT secret Supabase.
func NewHMACVerifier(secret, issuer, audience string, now func() time.Time) *JWTVerifier {
	key := []byte(secret)
	return &JWTVerifier{
		parser:  newParser(issuer, audience, now, "HS256"),
		keyfunc: func(*jwt.Token) (any, error) { return key, nil },
	}
}

// NewJWKSVerifier memakai kunci asimetris Supabase dari endpoint JWKS.
func NewJWKSVerifier(ctx context.Context, url, issuer, audience string) (*JWTVerifier, error) {
	k, err := keyfunc.NewDefaultCtx(ctx, []string{url})
	if err != nil {
		return nil, fmt.Errorf("baca JWKS: %w", err)
	}
	return &JWTVerifier{
		parser:  newParser(issuer, audience, nil, "ES256", "RS256"),
		keyfunc: k.Keyfunc,
	}, nil
}

// Verify memeriksa tanda tangan dan klaim wajib.
func (v *JWTVerifier) Verify(_ context.Context, token string) (Identity, error) {
	var c claims
	if _, err := v.parser.ParseWithClaims(token, &c, v.keyfunc); err != nil {
		return Identity{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if c.Subject == "" {
		return Identity{}, fmt.Errorf("%w: sub kosong", ErrInvalidToken)
	}
	return Identity{
		Subject:       c.Subject,
		Email:         strings.ToLower(strings.TrimSpace(c.Email)),
		EmailVerified: c.UserMetadata.EmailVerified,
	}, nil
}
