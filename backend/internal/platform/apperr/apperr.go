// Package apperr mendefinisikan error bisnis yang dipahami semua layer.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code adalah nilai `code` di kontrak API. Klien mengambil keputusan dari nilai ini.
type Code string

const (
	ValidationFailed           Code = "VALIDATION_FAILED"
	Unauthenticated            Code = "UNAUTHENTICATED"
	Forbidden                  Code = "FORBIDDEN"
	RoleNotAssigned            Code = "ROLE_NOT_ASSIGNED"
	WalletNotLinked            Code = "WALLET_NOT_LINKED"
	NotFound                   Code = "NOT_FOUND"
	MethodNotAllowed           Code = "METHOD_NOT_ALLOWED"
	RateLimited                Code = "RATE_LIMITED"
	Internal                   Code = "INTERNAL_ERROR"
	ServiceUnavailable         Code = "SERVICE_UNAVAILABLE"
	NonceInvalid               Code = "NONCE_INVALID"
	SignatureInvalid           Code = "SIGNATURE_INVALID"
	WalletAlreadyLinked        Code = "WALLET_ALREADY_LINKED"
	StockBelowReserved         Code = "STOCK_BELOW_RESERVED"
	ConsignorEmailTaken        Code = "CONSIGNOR_EMAIL_TAKEN"
	IdempotencyKeyReused       Code = "IDEMPOTENCY_KEY_REUSED"
	InsufficientStock          Code = "INSUFFICIENT_STOCK"
	TooManyConsignors          Code = "TOO_MANY_CONSIGNORS"
	DiscountTooLarge           Code = "DISCOUNT_TOO_LARGE"
	RecipientWalletMissing     Code = "RECIPIENT_WALLET_MISSING"
	StoreWalletNotLinked       Code = "STORE_WALLET_NOT_LINKED"
	BelowRentMinimum           Code = "BELOW_RENT_MINIMUM"
	RateUnavailable            Code = "RATE_UNAVAILABLE"
	OrderNotRefundable         Code = "ORDER_NOT_REFUNDABLE"
	AlreadyRefunded            Code = "ALREADY_REFUNDED"
	RefundInProgress           Code = "REFUND_IN_PROGRESS"
	RefundNotAwaitingSignature Code = "REFUND_NOT_AWAITING_SIGNATURE"
)

type meta struct {
	status int
	title  string
}

var codes = map[Code]meta{
	ValidationFailed:           {http.StatusBadRequest, "Invalid request"},
	Unauthenticated:            {http.StatusUnauthorized, "Sign-in required"},
	Forbidden:                  {http.StatusForbidden, "Access denied"},
	RoleNotAssigned:            {http.StatusForbidden, "Account not registered at a store"},
	WalletNotLinked:            {http.StatusForbidden, "Wallet not linked"},
	NotFound:                   {http.StatusNotFound, "Not found"},
	MethodNotAllowed:           {http.StatusMethodNotAllowed, "Method not allowed"},
	RateLimited:                {http.StatusTooManyRequests, "Too many requests"},
	Internal:                   {http.StatusInternalServerError, "Server error"},
	ServiceUnavailable:         {http.StatusServiceUnavailable, "Service unavailable"},
	NonceInvalid:               {http.StatusUnprocessableEntity, "Invalid nonce"},
	SignatureInvalid:           {http.StatusUnprocessableEntity, "Invalid signature"},
	WalletAlreadyLinked:        {http.StatusConflict, "Wallet already linked"},
	StockBelowReserved:         {http.StatusConflict, "Stock below held quantity"},
	ConsignorEmailTaken:        {http.StatusConflict, "Email already registered"},
	IdempotencyKeyReused:       {http.StatusConflict, "Idempotency key reused"},
	InsufficientStock:          {http.StatusConflict, "Insufficient stock"},
	TooManyConsignors:          {http.StatusUnprocessableEntity, "Too many consignors"},
	DiscountTooLarge:           {http.StatusUnprocessableEntity, "Discount too large"},
	RecipientWalletMissing:     {http.StatusUnprocessableEntity, "Recipient wallet missing"},
	StoreWalletNotLinked:       {http.StatusUnprocessableEntity, "Store wallet not linked"},
	BelowRentMinimum:           {http.StatusUnprocessableEntity, "Below rent minimum"},
	RateUnavailable:            {http.StatusServiceUnavailable, "Rate unavailable"},
	OrderNotRefundable:         {http.StatusConflict, "Order not refundable"},
	AlreadyRefunded:            {http.StatusConflict, "Already refunded"},
	RefundInProgress:           {http.StatusConflict, "Refund in progress"},
	RefundNotAwaitingSignature: {http.StatusConflict, "Refund not awaiting signature"},
}

// Status mengembalikan HTTP status untuk kode ini.
func (c Code) Status() int {
	if m, ok := codes[c]; ok {
		return m.status
	}
	return http.StatusInternalServerError
}

// Title mengembalikan judul singkat untuk kode ini.
func (c Code) Title() string {
	if m, ok := codes[c]; ok {
		return m.title
	}
	return codes[Internal].title
}

// Codes mengembalikan semua kode yang dikenal.
func Codes() []Code {
	out := make([]Code, 0, len(codes))
	for c := range codes {
		out = append(out, c)
	}
	return out
}

// FieldError menjelaskan satu kolom yang salah.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error adalah error yang boleh ditampilkan ke klien.
type Error struct {
	Code    Code
	Detail  string
	Fields  []FieldError
	Details map[string]any
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Detail)
}

// New membuat error dengan pesan untuk manusia.
func New(code Code, detail string) *Error {
	return &Error{Code: code, Detail: detail}
}

// WithDetails menambahkan data pendukung untuk klien.
func (e *Error) WithDetails(details map[string]any) *Error {
	e.Details = details
	return e
}

// Validation membuat error validasi per kolom.
func Validation(fields ...FieldError) *Error {
	return &Error{Code: ValidationFailed, Detail: "Periksa kembali isian yang dikirim.", Fields: fields}
}

// As mengambil *Error dari rantai error.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}

// Is memeriksa apakah err membawa kode tertentu.
func Is(err error, code Code) bool {
	e, ok := As(err)
	return ok && e.Code == code
}
