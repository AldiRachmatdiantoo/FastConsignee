// Package api berisi helper HTTP yang dipakai semua handler.
package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
)

const maxBodyBytes = 1 << 20

type requestIDKey struct{}

// WithRequestID menyimpan request ID di context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestID membaca request ID dari context.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// Problem adalah jawaban error menurut RFC 9457.
type Problem struct {
	Type      string              `json:"type"`
	Title     string              `json:"title"`
	Status    int                 `json:"status"`
	Detail    string              `json:"detail,omitempty"`
	Instance  string              `json:"instance,omitempty"`
	Code      apperr.Code         `json:"code"`
	RequestID string              `json:"request_id"`
	Errors    []apperr.FieldError `json:"errors,omitempty"`
	Details   map[string]any      `json:"details,omitempty"`
}

// JSON menulis jawaban JSON.
func JSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("tulis jawaban gagal", "error", err)
	}
}

// Error menulis problem+json. Error yang tidak dikenal dicatat dan disembunyikan dari klien.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	e, ok := apperr.As(err)
	if !ok {
		e = classify(r, err)
	}
	if e.Code == apperr.Unauthenticated {
		w.Header().Set("WWW-Authenticate", `Bearer realm="fastconsignee"`)
	}
	p := Problem{
		Type:      "about:blank",
		Title:     e.Code.Title(),
		Status:    e.Code.Status(),
		Detail:    e.Detail,
		Instance:  r.URL.Path,
		Code:      e.Code,
		RequestID: RequestID(r.Context()),
		Errors:    e.Fields,
		Details:   e.Details,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(p.Status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		slog.Error("tulis problem gagal", "error", err)
	}
}

func classify(r *http.Request, err error) *apperr.Error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		slog.Warn("permintaan terhenti", "error", err, "request_id", RequestID(r.Context()))
		return apperr.New(apperr.ServiceUnavailable, "Permintaan terhenti. Coba lagi.")
	}
	slog.Error("kesalahan server", "error", err, "path", r.URL.Path, "request_id", RequestID(r.Context()))
	return apperr.New(apperr.Internal, "Terjadi kesalahan di server.")
}

// Decode membaca body JSON dan menolak kolom yang tidak dikenal.
func Decode(w http.ResponseWriter, r *http.Request, dst any) error {
	return decode(w, r, dst, true)
}

// DecodeLoose membaca body JSON dan mengabaikan kolom tambahan.
func DecodeLoose(w http.ResponseWriter, r *http.Request, dst any) error {
	return decode(w, r, dst, false)
}

func decode(w http.ResponseWriter, r *http.Request, dst any, strict bool) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(dst); err != nil {
		return apperr.Validation(apperr.FieldError{Field: "body", Message: decodeMessage(err)})
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return apperr.Validation(apperr.FieldError{Field: "body", Message: "Body hanya boleh berisi satu objek JSON."})
	}
	return nil
}

func decodeMessage(err error) string {
	var typeErr *json.UnmarshalTypeError
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		return "Body terlalu besar."
	case errors.As(err, &typeErr):
		return fmt.Sprintf("Kolom %s bertipe salah.", typeErr.Field)
	case errors.Is(err, io.EOF):
		return "Body kosong."
	case strings.HasPrefix(err.Error(), "json: unknown field"):
		return "Ada kolom yang tidak dikenal: " + strings.TrimPrefix(err.Error(), "json: unknown field ")
	default:
		return "Body bukan JSON yang valid."
	}
}

// PathUUID membaca parameter path sebagai UUID.
func PathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		return uuid.Nil, apperr.Validation(apperr.FieldError{Field: name, Message: "Harus berupa UUID."})
	}
	return id, nil
}

var solanaAddress = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{32,44}$`)
var solanaSignature = regexp.MustCompile(`^[1-9A-HJ-NP-Za-km-z]{64,88}$`)
var idempotencyKey = regexp.MustCompile(`^[A-Za-z0-9_.:-]{8,128}$`)

// Validator mengumpulkan kesalahan per kolom.
type Validator struct {
	fields []apperr.FieldError
}

// Add mencatat satu kesalahan.
func (v *Validator) Add(field, message string) {
	v.fields = append(v.fields, apperr.FieldError{Field: field, Message: message})
}

// Err mengembalikan error kalau ada kesalahan.
func (v *Validator) Err() error {
	if len(v.fields) == 0 {
		return nil
	}
	return apperr.Validation(v.fields...)
}

// Text memeriksa panjang teks dalam karakter.
func (v *Validator) Text(field, value string, minLen, maxLen int) {
	n := utf8.RuneCountInString(value)
	if n < minLen || n > maxLen {
		v.Add(field, fmt.Sprintf("Panjangnya harus %d sampai %d karakter.", minLen, maxLen))
	}
}

// Range memeriksa angka.
func (v *Validator) Range(field string, value, minValue, maxValue int64) {
	if value < minValue || value > maxValue {
		v.Add(field, fmt.Sprintf("Nilainya harus %d sampai %d.", minValue, maxValue))
	}
}

// Email memeriksa format email.
func (v *Validator) Email(field, value string) {
	addr, err := mail.ParseAddress(value)
	if err != nil || addr.Address != value || len(value) > 254 {
		v.Add(field, "Format email tidak valid.")
	}
}

// Address memeriksa format alamat Solana.
func (v *Validator) Address(field, value string) {
	if !IsSolanaAddress(value) {
		v.Add(field, "Alamat Solana tidak valid.")
	}
}

// Signature memeriksa format signature Solana.
func (v *Validator) Signature(field, value string) {
	if !solanaSignature.MatchString(value) {
		v.Add(field, "Signature tidak valid.")
	}
}

// IsSolanaAddress memeriksa format base58 dan panjang 32 byte.
func IsSolanaAddress(value string) bool {
	return solanaAddress.MatchString(value)
}

// IdempotencyKey membaca header wajib Idempotency-Key.
func IdempotencyKey(r *http.Request) (string, error) {
	key := r.Header.Get("Idempotency-Key")
	if !idempotencyKey.MatchString(key) {
		return "", apperr.Validation(apperr.FieldError{
			Field:   "Idempotency-Key",
			Message: "Header wajib diisi, 8 sampai 128 karakter.",
		})
	}
	return key, nil
}

// Page adalah parameter daftar yang sudah divalidasi.
type Page struct {
	Limit  int
	Cursor *Cursor
}

// Cursor menandai posisi terakhir di daftar yang terurut waktu lalu ID.
type Cursor struct {
	Time time.Time `json:"t"`
	ID   uuid.UUID `json:"i"`
}

// ParsePage membaca `limit` dan `cursor`.
func ParsePage(r *http.Request) (Page, error) {
	q := r.URL.Query()
	p := Page{Limit: 20}
	var v Validator
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 100 {
			v.Add("limit", "Nilainya harus 1 sampai 100.")
		} else {
			p.Limit = n
		}
	}
	if s := q.Get("cursor"); s != "" {
		c, err := decodeCursor(s)
		if err != nil {
			v.Add("cursor", "Cursor tidak valid.")
		} else {
			p.Cursor = &c
		}
	}
	return p, v.Err()
}

// EncodeCursor mengubah cursor menjadi nilai `next_cursor`.
func EncodeCursor(t time.Time, id uuid.UUID) string {
	b, _ := json.Marshal(Cursor{Time: t, ID: id})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (Cursor, error) {
	if len(s) > 512 {
		return Cursor{}, errors.New("cursor terlalu panjang")
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return Cursor{}, err
	}
	var c Cursor
	if err := json.Unmarshal(b, &c); err != nil {
		return Cursor{}, err
	}
	return c, nil
}

// NextCursor menghitung `next_cursor`. Repository mengambil limit+1 baris, jadi baris lebihnya
// menandakan masih ada halaman berikutnya.
func NextCursor[T any](rows []T, limit int, key func(T) (time.Time, uuid.UUID)) ([]T, *string) {
	if len(rows) <= limit {
		return rows, nil
	}
	rows = rows[:limit]
	t, id := key(rows[limit-1])
	next := EncodeCursor(t, id)
	return rows, &next
}

// ParseTimeRange membaca `from` dan `to` dalam RFC 3339.
func ParseTimeRange(r *http.Request) (from, to *time.Time, err error) {
	var v Validator
	read := func(name string) *time.Time {
		s := r.URL.Query().Get(name)
		if s == "" {
			return nil
		}
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			v.Add(name, "Harus berupa waktu RFC 3339.")
			return nil
		}
		t = t.UTC()
		return &t
	}
	from, to = read("from"), read("to")
	return from, to, v.Err()
}

// Enum membaca parameter query yang nilainya terbatas.
func Enum(r *http.Request, name string, allowed ...string) (*string, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return nil, nil
	}
	for _, a := range allowed {
		if s == a {
			return &s, nil
		}
	}
	return nil, apperr.Validation(apperr.FieldError{Field: name, Message: "Harus salah satu dari: " + strings.Join(allowed, ", ")})
}
