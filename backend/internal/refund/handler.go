package refund

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/order"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
)

// Handler berisi endpoint refund.
type Handler struct {
	service *Service
}

// NewHandler membuat handler refund.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

type createRequest struct {
	OrderLineID *uuid.UUID `json:"order_line_id"`
	Reason      *string    `json:"reason"`
}

type draftResponse struct {
	order.RefundView
	Transaction string `json:"transaction"`
}

// Create menjawab POST /api/v1/orders/{order_id}/refunds.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orderID, err := api.PathUUID(r, "order_id")
	if err != nil {
		api.Error(w, r, err)
		return
	}
	key, err := api.IdempotencyKey(r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	var req createRequest
	if err := api.Decode(w, r, &req); err != nil {
		api.Error(w, r, err)
		return
	}
	var v api.Validator
	if req.OrderLineID == nil {
		v.Add("order_line_id", "Required.")
	}
	if req.Reason != nil {
		v.Text("reason", *req.Reason, 0, 500)
	}
	if err := v.Err(); err != nil {
		api.Error(w, r, err)
		return
	}
	d, replayed, err := h.service.Create(r.Context(), auth.MustFromContext(r.Context()), key, Input{
		OrderID: orderID, LineID: *req.OrderLineID, Reason: req.Reason,
	})
	if err != nil {
		api.Error(w, r, err)
		return
	}
	if replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	w.Header().Set("Location", "/api/v1/refunds/"+d.Refund.ID.String())
	api.JSON(w, http.StatusCreated, draftResponse{RefundView: order.ToRefundView(d.Refund), Transaction: d.Transaction})
}

// List menjawab GET /api/v1/refunds.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	status, err := api.Enum(r, "status", StatusAwaitingSignature, StatusConfirming, StatusCompleted, StatusFailed, StatusExpired)
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
	storeID := auth.MustFromContext(r.Context()).StoreID
	rows, err := h.service.List(r.Context(), storeID, Filter{Status: status, From: from, To: to}, page)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	rows, next := api.NextCursor(rows, page.Limit, func(f order.Refund) (time.Time, uuid.UUID) { return f.CreatedAt, f.ID })
	data := make([]order.RefundView, 0, len(rows))
	for _, f := range rows {
		data = append(data, order.ToRefundView(f))
	}
	api.JSON(w, http.StatusOK, map[string]any{"data": data, "next_cursor": next})
}

// Get menjawab GET /api/v1/refunds/{refund_id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := api.PathUUID(r, "refund_id")
	if err != nil {
		api.Error(w, r, err)
		return
	}
	f, err := h.service.Get(r.Context(), auth.MustFromContext(r.Context()).StoreID, id)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	api.JSON(w, http.StatusOK, order.ToRefundView(f))
}

// Signature menjawab POST /api/v1/refunds/{refund_id}/signature.
func (h *Handler) Signature(w http.ResponseWriter, r *http.Request) {
	id, err := api.PathUUID(r, "refund_id")
	if err != nil {
		api.Error(w, r, err)
		return
	}
	var req struct {
		Signature string `json:"signature"`
	}
	if err := api.Decode(w, r, &req); err != nil {
		api.Error(w, r, err)
		return
	}
	var v api.Validator
	v.Signature("signature", req.Signature)
	if err := v.Err(); err != nil {
		api.Error(w, r, err)
		return
	}
	f, err := h.service.Submit(r.Context(), auth.MustFromContext(r.Context()).StoreID, id, req.Signature)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	api.JSON(w, http.StatusAccepted, order.ToRefundView(f))
}
