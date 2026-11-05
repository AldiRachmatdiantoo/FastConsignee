package order

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/item"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

const (
	maxCartLines   = 50
	maxQuantity    = 1000
	maxDiscountIDR = 1_000_000_000_000_000

	// Network adalah satu-satunya cluster yang didukung.
	Network = "devnet"
)

// Handler berisi endpoint order.
type Handler struct {
	service   *Service
	publicURL string
}

// NewHandler membuat handler order.
func NewHandler(s *Service, publicURL string) *Handler {
	return &Handler{service: s, publicURL: publicURL}
}

// SolanaPayURL adalah isi QR satu order. Spesifikasi Solana Pay mengizinkan URL tanpa encoding
// selama tidak memuat query.
func SolanaPayURL(publicURL string, id uuid.UUID) string {
	return "solana:" + publicURL + "/api/v1/solana-pay/orders/" + id.String()
}

type cartLineRequest struct {
	ItemID   *uuid.UUID `json:"item_id"`
	Quantity *int32     `json:"quantity"`
	Discount *struct {
		AmountIDR *int64  `json:"amount_idr"`
		BorneBy   *string `json:"borne_by"`
	} `json:"discount"`
}

type cartRequest struct {
	Lines []cartLineRequest `json:"lines"`
}

func (c cartRequest) cart() (Cart, error) {
	var v api.Validator
	if len(c.Lines) < 1 || len(c.Lines) > maxCartLines {
		v.Add("lines", "Isi 1 sampai 50 baris.")
	}
	out := Cart{Lines: make([]CartLine, 0, len(c.Lines))}
	seen := make(map[uuid.UUID]bool, len(c.Lines))
	for i, l := range c.Lines {
		field := func(name string) string { return fmt.Sprintf("lines[%d].%s", i, name) }
		line := CartLine{}
		switch {
		case l.ItemID == nil:
			v.Add(field("item_id"), "Wajib diisi.")
		case seen[*l.ItemID]:
			v.Add(field("item_id"), "Satu barang hanya boleh muncul sekali.")
		default:
			seen[*l.ItemID] = true
			line.ItemID = *l.ItemID
		}
		if l.Quantity == nil {
			v.Add(field("quantity"), "Wajib diisi.")
		} else {
			v.Range(field("quantity"), int64(*l.Quantity), 1, maxQuantity)
			line.Quantity = *l.Quantity
		}
		if d := l.Discount; d != nil {
			switch {
			case d.AmountIDR == nil:
				v.Add(field("discount.amount_idr"), "Wajib diisi.")
			case d.BorneBy == nil || (*d.BorneBy != BearerStore && *d.BorneBy != BearerConsignor):
				v.Add(field("discount.borne_by"), "Harus store atau consignor.")
			default:
				v.Range(field("discount.amount_idr"), *d.AmountIDR, 1, maxDiscountIDR)
				line.Discount = &Discount{AmountIDR: *d.AmountIDR, BorneBy: *d.BorneBy}
			}
		}
		out.Lines = append(out.Lines, line)
	}
	return out, v.Err()
}

func decodeCart(w http.ResponseWriter, r *http.Request) (Cart, error) {
	var req cartRequest
	if err := api.Decode(w, r, &req); err != nil {
		return Cart{}, err
	}
	return req.cart()
}

type previewLine struct {
	ItemID            uuid.UUID         `json:"item_id"`
	ItemName          string            `json:"item_name"`
	Consignor         item.ConsignorRef `json:"consignor"`
	Quantity          int32             `json:"quantity"`
	UnitPriceIDR      int64             `json:"unit_price_idr"`
	CommissionBps     int32             `json:"commission_bps"`
	Discount          *Discount         `json:"discount"`
	LineTotalIDR      int64             `json:"line_total_idr"`
	CommissionIDR     int64             `json:"commission_idr"`
	ConsignorShareIDR int64             `json:"consignor_share_idr"`
}

func toPreviewLine(l Line) previewLine {
	return previewLine{
		ItemID: l.ItemID, ItemName: l.ItemName, Consignor: l.Consignor, Quantity: l.Quantity,
		UnitPriceIDR: l.UnitPriceIDR, CommissionBps: l.CommissionBps, Discount: l.Discount,
		LineTotalIDR: l.Amounts.LineTotal, CommissionIDR: l.Amounts.Commission, ConsignorShareIDR: l.Amounts.ConsignorShare,
	}
}

type recipientResponse struct {
	RecipientType  string     `json:"recipient_type"`
	ConsignorID    *uuid.UUID `json:"consignor_id"`
	Name           string     `json:"name"`
	WalletAddress  string     `json:"wallet_address"`
	ShareIDR       int64      `json:"share_idr"`
	DeductionIDR   int64      `json:"deduction_idr"`
	RecoveredIDR   int64      `json:"recovered_idr"`
	AmountIDR      int64      `json:"amount_idr"`
	AmountLamports string     `json:"amount_lamports"`
}

func toRecipients(s Split) []recipientResponse {
	out := make([]recipientResponse, 0, len(s.Recipients))
	for _, r := range s.Recipients {
		kind := "consignor"
		if r.ConsignorID == nil {
			kind = "store"
		}
		out = append(out, recipientResponse{
			RecipientType: kind, ConsignorID: r.ConsignorID, Name: r.Name, WalletAddress: r.Wallet,
			ShareIDR: r.ShareIDR, DeductionIDR: r.DeductionIDR, RecoveredIDR: r.RecoveredIDR, AmountIDR: r.AmountIDR,
			AmountLamports: Lamports(r.AmountLamports),
		})
	}
	return out
}

// Lamports menulis lamport sebagai string, karena nilainya bisa melewati presisi angka JSON.
func Lamports(v int64) string {
	return strconv.FormatInt(v, 10)
}

type rateResponse struct {
	IDRPerSOL int64     `json:"idr_per_sol"`
	QuotedAt  time.Time `json:"quoted_at"`
}

// RateView menyiapkan kurs untuk JSON.
func RateView(r Rate) any {
	return rateResponse{IDRPerSOL: r.IDRPerSOL, QuotedAt: r.QuotedAt.UTC()}
}

type previewResponse struct {
	SubtotalIDR   int64               `json:"subtotal_idr"`
	DiscountIDR   int64               `json:"discount_idr"`
	TotalIDR      int64               `json:"total_idr"`
	TotalLamports string              `json:"total_lamports"`
	Rate          any                 `json:"rate"`
	Lines         []previewLine       `json:"lines"`
	Recipients    []recipientResponse `json:"recipients"`
}

// Preview menjawab POST /api/v1/orders/preview.
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	cart, err := decodeCart(w, r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	p, err := h.service.Preview(r.Context(), auth.MustFromContext(r.Context()), cart)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	lines := make([]previewLine, 0, len(p.Lines))
	for _, l := range p.Lines {
		lines = append(lines, toPreviewLine(l))
	}
	api.JSON(w, http.StatusOK, previewResponse{
		SubtotalIDR: p.Totals.SubtotalIDR, DiscountIDR: p.Totals.DiscountIDR, TotalIDR: p.Split.TotalIDR,
		TotalLamports: Lamports(p.Split.TotalLamports), Rate: RateView(p.Rate), Lines: lines, Recipients: toRecipients(p.Split),
	})
}

// CertificateView adalah bentuk JSON sertifikat.
type CertificateView struct {
	Status      string  `json:"status"`
	MintAddress *string `json:"mint_address"`
	ExplorerURL *string `json:"explorer_url"`
}

// ToCertificateView menyiapkan sertifikat untuk JSON.
func ToCertificateView(c *Certificate) *CertificateView {
	if c == nil {
		return nil
	}
	v := &CertificateView{Status: c.Status, MintAddress: c.MintAddress}
	if c.MintAddress != nil {
		u := solana.ExplorerAddressURL(*c.MintAddress)
		v.ExplorerURL = &u
	}
	return v
}

// ExplorerURL mengubah signature opsional menjadi tautan opsional.
func ExplorerURL(signature *string) *string {
	if signature == nil {
		return nil
	}
	u := solana.ExplorerURL(*signature)
	return &u
}

type orderLineResponse struct {
	previewLine
	ID               uuid.UUID        `json:"id"`
	RefundedQuantity int32            `json:"refunded_quantity"`
	Certificate      *CertificateView `json:"certificate"`
}

type paymentResponse struct {
	Network      string  `json:"network"`
	Reference    string  `json:"reference"`
	SolanaPayURL string  `json:"solana_pay_url"`
	PayerWallet  *string `json:"payer_wallet"`
	Signature    *string `json:"signature"`
	ExplorerURL  *string `json:"explorer_url"`
	FailureCode  *string `json:"failure_code"`
}

type deductionView struct {
	Consignor item.ConsignorRef `json:"consignor"`
	AmountIDR int64             `json:"amount_idr"`
}

// RefundView adalah bentuk JSON refund.
type RefundView struct {
	ID                uuid.UUID     `json:"id"`
	OrderID           uuid.UUID     `json:"order_id"`
	OrderLineID       uuid.UUID     `json:"order_line_id"`
	ItemName          string        `json:"item_name"`
	Status            string        `json:"status"`
	AmountIDR         int64         `json:"amount_idr"`
	AmountLamports    string        `json:"amount_lamports"`
	SourceWallet      string        `json:"source_wallet"`
	DestinationWallet string        `json:"destination_wallet"`
	Reference         string        `json:"reference"`
	Deduction         deductionView `json:"deduction"`
	Reason            *string       `json:"reason"`
	Signature         *string       `json:"signature"`
	ExplorerURL       *string       `json:"explorer_url"`
	ExpiresAt         time.Time     `json:"expires_at"`
	CreatedAt         time.Time     `json:"created_at"`
	CompletedAt       *time.Time    `json:"completed_at"`
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.UTC()
	return &v
}

// ToRefundView menyiapkan refund untuk JSON.
func ToRefundView(f Refund) RefundView {
	return RefundView{
		ID: f.ID, OrderID: f.OrderID, OrderLineID: f.OrderLineID, ItemName: f.ItemName, Status: f.Status,
		AmountIDR: f.AmountIDR, AmountLamports: Lamports(f.AmountLamports), SourceWallet: f.SourceWallet,
		DestinationWallet: f.DestinationWallet, Reference: f.Reference,
		Deduction: deductionView{Consignor: f.Consignor, AmountIDR: f.DeductionIDR},
		Reason:    f.Reason, Signature: f.Signature, ExplorerURL: ExplorerURL(f.Signature),
		ExpiresAt: f.ExpiresAt.UTC(), CreatedAt: f.CreatedAt.UTC(), CompletedAt: utc(f.CompletedAt),
	}
}

type orderResponse struct {
	ID            uuid.UUID           `json:"id"`
	Status        string              `json:"status"`
	SubtotalIDR   int64               `json:"subtotal_idr"`
	DiscountIDR   int64               `json:"discount_idr"`
	TotalIDR      int64               `json:"total_idr"`
	TotalLamports string              `json:"total_lamports"`
	Rate          any                 `json:"rate"`
	Lines         []orderLineResponse `json:"lines"`
	Recipients    []recipientResponse `json:"recipients"`
	Payment       paymentResponse     `json:"payment"`
	Refunds       []RefundView        `json:"refunds"`
	CreatedAt     time.Time           `json:"created_at"`
	ExpiresAt     time.Time           `json:"expires_at"`
	ConfirmedAt   *time.Time          `json:"confirmed_at"`
}

func (h *Handler) toResponse(o Order) orderResponse {
	lines := make([]orderLineResponse, 0, len(o.Lines))
	for _, l := range o.Lines {
		lines = append(lines, orderLineResponse{
			previewLine: toPreviewLine(l), ID: l.ID, RefundedQuantity: l.RefundedQuantity, Certificate: ToCertificateView(l.Certificate),
		})
	}
	refunds := make([]RefundView, 0, len(o.Refunds))
	for _, f := range o.Refunds {
		refunds = append(refunds, ToRefundView(f))
	}
	return orderResponse{
		ID: o.ID, Status: o.Status, SubtotalIDR: o.Totals.SubtotalIDR, DiscountIDR: o.Totals.DiscountIDR,
		TotalIDR: o.Split.TotalIDR, TotalLamports: Lamports(o.Split.TotalLamports), Rate: RateView(o.Rate),
		Lines: lines, Recipients: toRecipients(o.Split),
		Payment: paymentResponse{
			Network: Network, Reference: o.Reference, SolanaPayURL: SolanaPayURL(h.publicURL, o.ID),
			PayerWallet: o.PayerWallet, Signature: o.Signature, ExplorerURL: ExplorerURL(o.Signature), FailureCode: o.FailureCode,
		},
		Refunds: refunds, CreatedAt: o.CreatedAt.UTC(), ExpiresAt: o.ExpiresAt.UTC(), ConfirmedAt: utc(o.ConfirmedAt),
	}
}

// Create menjawab POST /api/v1/orders.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	key, err := api.IdempotencyKey(r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	cart, err := decodeCart(w, r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	o, replayed, err := h.service.Create(r.Context(), auth.MustFromContext(r.Context()), key, cart)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	if replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	w.Header().Set("Location", "/api/v1/orders/"+o.ID.String())
	api.JSON(w, http.StatusCreated, h.toResponse(o))
}

// Get menjawab GET /api/v1/orders/{order_id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := api.PathUUID(r, "order_id")
	if err != nil {
		api.Error(w, r, err)
		return
	}
	o, err := h.service.Get(r.Context(), auth.MustFromContext(r.Context()), id)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	api.JSON(w, http.StatusOK, h.toResponse(o))
}

type summaryResponse struct {
	ID            uuid.UUID  `json:"id"`
	Status        string     `json:"status"`
	TotalIDR      int64      `json:"total_idr"`
	TotalLamports string     `json:"total_lamports"`
	CommissionIDR int64      `json:"commission_idr"`
	ItemCount     int32      `json:"item_count"`
	Signature     *string    `json:"signature"`
	ExplorerURL   *string    `json:"explorer_url"`
	CreatedAt     time.Time  `json:"created_at"`
	ConfirmedAt   *time.Time `json:"confirmed_at"`
}

// List menjawab GET /api/v1/orders.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	status, err := api.Enum(r, "status", StatusPending, StatusConfirmed, StatusExpired, StatusFailed)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	from, to, err := api.ParseTimeRange(r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	page, err := api.ParsePage(r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	rows, err := h.service.List(r.Context(), auth.MustFromContext(r.Context()), Filter{Status: status, From: from, To: to}, page)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	rows, next := api.NextCursor(rows, page.Limit, func(s Summary) (time.Time, uuid.UUID) { return s.CreatedAt, s.ID })
	data := make([]summaryResponse, 0, len(rows))
	for _, s := range rows {
		data = append(data, summaryResponse{
			ID: s.ID, Status: s.Status, TotalIDR: s.TotalIDR, TotalLamports: Lamports(s.TotalLamports),
			CommissionIDR: s.CommissionIDR, ItemCount: s.ItemCount, Signature: s.Signature, ExplorerURL: ExplorerURL(s.Signature),
			CreatedAt: s.CreatedAt.UTC(), ConfirmedAt: utc(s.ConfirmedAt),
		})
	}
	api.JSON(w, http.StatusOK, map[string]any{"data": data, "next_cursor": next})
}

type statusResponse struct {
	ID          uuid.UUID `json:"id"`
	Status      string    `json:"status"`
	ExpiresAt   time.Time `json:"expires_at"`
	Signature   *string   `json:"signature"`
	ExplorerURL *string   `json:"explorer_url"`
	FailureCode *string   `json:"failure_code"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Status menjawab GET /api/v1/orders/{order_id}/status.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	id, err := api.PathUUID(r, "order_id")
	if err != nil {
		api.Error(w, r, err)
		return
	}
	v, err := h.service.Status(r.Context(), auth.MustFromContext(r.Context()), id)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	api.JSON(w, http.StatusOK, statusResponse{
		ID: v.ID, Status: v.Status, ExpiresAt: v.ExpiresAt.UTC(), Signature: v.Signature, ExplorerURL: ExplorerURL(v.Signature),
		FailureCode: v.FailureCode, UpdatedAt: v.UpdatedAt.UTC(),
	})
}
