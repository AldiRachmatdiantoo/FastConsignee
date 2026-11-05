package dashboard

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/item"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/order"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

// Handler berisi endpoint dasbor.
type Handler struct {
	service *Service
}

// NewHandler membuat handler dasbor.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

// consignorOf membaca penitip dari JWT, jadi tidak ada ID yang bisa ditukar klien.
func consignorOf(r *http.Request) (auth.Principal, uuid.UUID, error) {
	p := auth.MustFromContext(r.Context())
	if p.ConsignorID == nil {
		return p, uuid.Nil, apperr.New(apperr.Forbidden, "Akun ini bukan penitip.")
	}
	return p, *p.ConsignorID, nil
}

type saleResponse struct {
	OrderID               uuid.UUID `json:"order_id"`
	OrderLineID           uuid.UUID `json:"order_line_id"`
	ItemName              string    `json:"item_name"`
	Quantity              int32     `json:"quantity"`
	UnitPriceIDR          int64     `json:"unit_price_idr"`
	LineTotalIDR          int64     `json:"line_total_idr"`
	CommissionIDR         int64     `json:"commission_idr"`
	ConsignorShareIDR     int64     `json:"consignor_share_idr"`
	RefundedQuantity      int32     `json:"refunded_quantity"`
	OrderDeductionIDR     int64     `json:"order_deduction_idr"`
	OrderTransferLamports string    `json:"order_transfer_lamports"`
	SoldAt                time.Time `json:"sold_at"`
	Signature             string    `json:"signature"`
	ExplorerURL           string    `json:"explorer_url"`
}

// Sales menjawab GET /api/v1/dashboard/consignor/sales.
func (h *Handler) Sales(w http.ResponseWriter, r *http.Request) {
	_, consignorID, err := consignorOf(r)
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
	rows, err := h.service.Sales(r.Context(), consignorID, Period{From: from, To: to}, page)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	rows, next := api.NextCursor(rows, page.Limit, func(s Sale) (time.Time, uuid.UUID) { return s.SoldAt, s.LineID })
	data := make([]saleResponse, 0, len(rows))
	for _, s := range rows {
		data = append(data, saleResponse{
			OrderID: s.OrderID, OrderLineID: s.LineID, ItemName: s.ItemName, Quantity: s.Quantity, UnitPriceIDR: s.UnitPriceIDR,
			LineTotalIDR: s.Amounts.LineTotal, CommissionIDR: s.Amounts.Commission, ConsignorShareIDR: s.Amounts.ConsignorShare,
			RefundedQuantity: s.RefundedQuantity, OrderDeductionIDR: s.DeductionIDR,
			OrderTransferLamports: order.Lamports(s.TransferLamports), SoldAt: s.SoldAt.UTC(),
			Signature: s.Signature, ExplorerURL: solana.ExplorerURL(s.Signature),
		})
	}
	api.JSON(w, http.StatusOK, map[string]any{"data": data, "next_cursor": next})
}

type deductionResponse struct {
	ID           uuid.UUID  `json:"id"`
	RefundID     uuid.UUID  `json:"refund_id"`
	OrderID      uuid.UUID  `json:"order_id"`
	ItemName     string     `json:"item_name"`
	Status       string     `json:"status"`
	AmountIDR    int64      `json:"amount_idr"`
	RemainingIDR int64      `json:"remaining_idr"`
	CreatedAt    time.Time  `json:"created_at"`
	SettledAt    *time.Time `json:"settled_at"`
}

func toDeduction(d Deduction) deductionResponse {
	out := deductionResponse{
		ID: d.ID, RefundID: d.RefundID, OrderID: d.OrderID, ItemName: d.ItemName, Status: d.Status(),
		AmountIDR: d.AmountIDR, RemainingIDR: d.RemainingIDR, CreatedAt: d.CreatedAt.UTC(),
	}
	if d.RemainingIDR == 0 && d.LastSettled != nil {
		t := d.LastSettled.UTC()
		out.SettledAt = &t
	}
	return out
}

// Deductions menjawab GET /api/v1/dashboard/consignor/deductions.
func (h *Handler) Deductions(w http.ResponseWriter, r *http.Request) {
	_, consignorID, err := consignorOf(r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	status, err := api.Enum(r, "status", statusOutstanding, statusSettled)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	page, err := api.ParsePage(r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	rows, total, err := h.service.Deductions(r.Context(), consignorID, status, page)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	rows, next := api.NextCursor(rows, page.Limit, func(d Deduction) (time.Time, uuid.UUID) { return d.CreatedAt, d.ID })
	data := make([]deductionResponse, 0, len(rows))
	for _, d := range rows {
		data = append(data, toDeduction(d))
	}
	api.JSON(w, http.StatusOK, map[string]any{"data": data, "next_cursor": next, "outstanding_total_idr": total})
}

type stockResponse struct {
	ItemID         uuid.UUID `json:"item_id"`
	ItemName       string    `json:"item_name"`
	PriceIDR       int64     `json:"price_idr"`
	StockAvailable int32     `json:"stock_available"`
	Active         bool      `json:"active"`
}

// Stock menjawab GET /api/v1/dashboard/consignor/stock.
func (h *Handler) Stock(w http.ResponseWriter, r *http.Request) {
	p, consignorID, err := consignorOf(r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	page, err := api.ParsePage(r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	rows, err := h.service.Stock(r.Context(), p.StoreID, consignorID, page)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	rows, next := api.NextCursor(rows, page.Limit, func(i item.Item) (time.Time, uuid.UUID) { return i.CreatedAt, i.ID })
	data := make([]stockResponse, 0, len(rows))
	for _, i := range rows {
		data = append(data, stockResponse{
			ItemID: i.ID, ItemName: i.Name, PriceIDR: i.PriceIDR, StockAvailable: i.StockAvailable(), Active: i.Active,
		})
	}
	api.JSON(w, http.StatusOK, map[string]any{"data": data, "next_cursor": next})
}

type summaryResponse struct {
	From               *time.Time `json:"from"`
	To                 *time.Time `json:"to"`
	TotalSalesIDR      int64      `json:"total_sales_idr"`
	TotalCommissionIDR int64      `json:"total_commission_idr"`
	TransactionCount   int        `json:"transaction_count"`
	GeneratedAt        time.Time  `json:"generated_at"`
}

// Summary menjawab GET /api/v1/dashboard/store/summary.
func (h *Handler) Summary(w http.ResponseWriter, r *http.Request) {
	from, to, err := api.ParseTimeRange(r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	s, err := h.service.Summary(r.Context(), auth.MustFromContext(r.Context()).StoreID, Period{From: from, To: to})
	if err != nil {
		api.Error(w, r, err)
		return
	}
	api.JSON(w, http.StatusOK, summaryResponse{
		From: s.From, To: s.To, TotalSalesIDR: s.TotalSalesIDR, TotalCommissionIDR: s.TotalCommission,
		TransactionCount: s.TransactionCount, GeneratedAt: s.GeneratedAt,
	})
}
