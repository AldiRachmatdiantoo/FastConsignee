package order

import (
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/item"
)

// Status order.
const (
	StatusPending   = "pending"
	StatusConfirmed = "confirmed"
	StatusExpired   = "expired"
	StatusFailed    = "failed"
)

// Discount adalah diskon pada satu baris.
type Discount struct {
	AmountIDR int64  `json:"amount_idr"`
	BorneBy   string `json:"borne_by"`
}

// CartLine adalah satu baris keranjang.
type CartLine struct {
	ItemID   uuid.UUID `json:"item_id"`
	Quantity int32     `json:"quantity"`
	Discount *Discount `json:"discount"`
}

// Cart adalah keranjang yang sudah lolos validasi format.
type Cart struct {
	Lines []CartLine `json:"lines"`
}

// Rate adalah harga 1 SOL dalam Rupiah.
type Rate struct {
	IDRPerSOL int64     `json:"idr_per_sol"`
	QuotedAt  time.Time `json:"quoted_at"`
}

// Certificate adalah status sertifikat satu baris.
type Certificate struct {
	Status      string
	MintAddress *string
}

// Line adalah satu baris order atau rincian pembagian.
type Line struct {
	ID               uuid.UUID
	ItemID           uuid.UUID
	ItemName         string
	Consignor        item.ConsignorRef
	Quantity         int32
	UnitPriceIDR     int64
	CommissionBps    int32
	Discount         *Discount
	Amounts          Amounts
	RefundedQuantity int32
	Certificate      *Certificate
}

// Totals adalah jumlah semua baris.
type Totals struct {
	SubtotalIDR int64
	DiscountIDR int64
}

// Plan adalah hasil perhitungan keranjang.
type Plan struct {
	Lines       []Line
	Totals      Totals
	Split       Split
	Rate        Rate
	Settlements []Settlement
}

// Order adalah order lengkap.
type Order struct {
	ID          uuid.UUID
	CreatedBy   uuid.UUID
	Status      string
	Reference   string
	Rate        Rate
	PayerWallet *string
	Signature   *string
	FailureCode *string
	StoreName   string
	ExpiresAt   time.Time
	ConfirmedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Lines       []Line
	Totals      Totals
	Split       Split
	Refunds     []Refund
}

// Summary adalah satu baris riwayat order.
type Summary struct {
	ID            uuid.UUID
	Status        string
	TotalIDR      int64
	TotalLamports int64
	CommissionIDR int64
	ItemCount     int32
	Signature     *string
	CreatedAt     time.Time
	ConfirmedAt   *time.Time
}

// StatusView adalah status ringan untuk polling kasir.
type StatusView struct {
	ID          uuid.UUID
	Status      string
	ExpiresAt   time.Time
	Signature   *string
	FailureCode *string
	UpdatedAt   time.Time
}

// Refund adalah refund satu unit barang.
type Refund struct {
	ID                uuid.UUID
	OrderID           uuid.UUID
	OrderLineID       uuid.UUID
	ItemName          string
	Status            string
	AmountIDR         int64
	AmountLamports    int64
	SourceWallet      string
	DestinationWallet string
	Reference         string
	Consignor         item.ConsignorRef
	DeductionIDR      int64
	Reason            *string
	Signature         *string
	ExpiresAt         time.Time
	CreatedAt         time.Time
	CompletedAt       *time.Time
}

// Scope membatasi order yang boleh dibaca.
type Scope struct {
	StoreID   uuid.UUID
	CreatedBy *uuid.UUID
}

// ScopeOf menghitung batas baca. Kasir hanya melihat order buatannya.
func ScopeOf(p auth.Principal) Scope {
	s := Scope{StoreID: p.StoreID}
	if p.Role == auth.RoleCashier {
		id := p.UserID
		s.CreatedBy = &id
	}
	return s
}

// Filter adalah saringan riwayat order.
type Filter struct {
	Status *string
	From   *time.Time
	To     *time.Time
}
