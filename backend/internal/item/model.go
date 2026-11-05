// Package item mengurus katalog barang.
package item

import (
	"time"

	"github.com/google/uuid"
)

// ConsignorRef adalah penitip pemilik barang.
type ConsignorRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Item adalah barang dengan stok terhitung.
type Item struct {
	ID                 uuid.UUID
	Name               string
	PriceIDR           int64
	CommissionBps      int32
	Consignor          ConsignorRef
	StockOnHand        int32
	StockReserved      int32
	Active             bool
	CertificateEnabled bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// StockAvailable adalah stok yang bisa dijual.
func (i Item) StockAvailable() int32 {
	return max(i.StockOnHand-i.StockReserved, 0)
}

// Filter adalah saringan katalog.
type Filter struct {
	Query       *string
	ConsignorID *uuid.UUID
	Active      *bool
}

// Create adalah data barang baru.
type Create struct {
	Name               string
	PriceIDR           int64
	CommissionBps      int32
	ConsignorID        uuid.UUID
	StockOnHand        int32
	CertificateEnabled bool
}

// Update adalah perubahan sebagian. Nilai nil berarti tidak berubah.
type Update struct {
	Name               *string
	PriceIDR           *int64
	CommissionBps      *int32
	StockOnHand        *int32
	Active             *bool
	CertificateEnabled *bool
}
