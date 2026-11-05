package item

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
)

const (
	maxPriceIDR = 10_000_000_000
	maxStock    = 1_000_000
	maxBps      = 10_000
)

// Handler berisi endpoint barang.
type Handler struct {
	service *Service
}

// NewHandler membuat handler barang.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

type itemResponse struct {
	ID                 uuid.UUID    `json:"id"`
	Name               string       `json:"name"`
	PriceIDR           int64        `json:"price_idr"`
	CommissionBps      int32        `json:"commission_bps"`
	Consignor          ConsignorRef `json:"consignor"`
	StockOnHand        int32        `json:"stock_on_hand"`
	StockReserved      int32        `json:"stock_reserved"`
	StockAvailable     int32        `json:"stock_available"`
	Active             bool         `json:"active"`
	CertificateEnabled bool         `json:"certificate_enabled"`
	CreatedAt          time.Time    `json:"created_at"`
	UpdatedAt          time.Time    `json:"updated_at"`
}

func toResponse(i Item) itemResponse {
	return itemResponse{
		ID: i.ID, Name: i.Name, PriceIDR: i.PriceIDR, CommissionBps: i.CommissionBps, Consignor: i.Consignor,
		StockOnHand: i.StockOnHand, StockReserved: i.StockReserved, StockAvailable: i.StockAvailable(),
		Active: i.Active, CertificateEnabled: i.CertificateEnabled,
		CreatedAt: i.CreatedAt.UTC(), UpdatedAt: i.UpdatedAt.UTC(),
	}
}

type listResponse struct {
	Data       []itemResponse `json:"data"`
	NextCursor *string        `json:"next_cursor"`
}

func parseFilter(r *http.Request) (Filter, error) {
	q := r.URL.Query()
	var f Filter
	var v api.Validator
	if s := q.Get("q"); s != "" {
		v.Text("q", s, 1, 100)
		f.Query = &s
	}
	if s := q.Get("consignor_id"); s != "" {
		id, err := uuid.Parse(s)
		if err != nil {
			v.Add("consignor_id", "Harus berupa UUID.")
		}
		f.ConsignorID = &id
	}
	active := true
	if s := q.Get("active"); s != "" {
		b, err := strconv.ParseBool(s)
		if err != nil {
			v.Add("active", "Harus true atau false.")
		}
		active = b
	}
	f.Active = &active
	return f, v.Err()
}

// List menjawab GET /api/v1/items.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	f, err := parseFilter(r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	page, err := api.ParsePage(r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	items, err := h.service.List(r.Context(), auth.MustFromContext(r.Context()).StoreID, f, page)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	items, next := api.NextCursor(items, page.Limit, func(i Item) (time.Time, uuid.UUID) { return i.CreatedAt, i.ID })
	out := listResponse{Data: make([]itemResponse, 0, len(items)), NextCursor: next}
	for _, i := range items {
		out.Data = append(out.Data, toResponse(i))
	}
	api.JSON(w, http.StatusOK, out)
}

// Get menjawab GET /api/v1/items/{item_id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := api.PathUUID(r, "item_id")
	if err != nil {
		api.Error(w, r, err)
		return
	}
	i, err := h.service.Get(r.Context(), auth.MustFromContext(r.Context()).StoreID, id)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	api.JSON(w, http.StatusOK, toResponse(i))
}

type createRequest struct {
	Name               *string    `json:"name"`
	PriceIDR           *int64     `json:"price_idr"`
	CommissionBps      *int32     `json:"commission_bps"`
	ConsignorID        *uuid.UUID `json:"consignor_id"`
	StockOnHand        *int32     `json:"stock_on_hand"`
	CertificateEnabled bool       `json:"certificate_enabled"`
}

func (c createRequest) validate() error {
	var v api.Validator
	if c.Name == nil {
		v.Add("name", "Wajib diisi.")
	} else {
		v.Text("name", *c.Name, 1, 200)
	}
	if c.PriceIDR == nil {
		v.Add("price_idr", "Wajib diisi.")
	} else {
		v.Range("price_idr", *c.PriceIDR, 1, maxPriceIDR)
	}
	if c.CommissionBps == nil {
		v.Add("commission_bps", "Wajib diisi.")
	} else {
		v.Range("commission_bps", int64(*c.CommissionBps), 0, maxBps)
	}
	if c.ConsignorID == nil {
		v.Add("consignor_id", "Wajib diisi.")
	}
	if c.StockOnHand == nil {
		v.Add("stock_on_hand", "Wajib diisi.")
	} else {
		v.Range("stock_on_hand", int64(*c.StockOnHand), 0, maxStock)
	}
	return v.Err()
}

// Create menjawab POST /api/v1/items.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := api.Decode(w, r, &req); err != nil {
		api.Error(w, r, err)
		return
	}
	if err := req.validate(); err != nil {
		api.Error(w, r, err)
		return
	}
	i, err := h.service.Create(r.Context(), auth.MustFromContext(r.Context()).StoreID, Create{
		Name: *req.Name, PriceIDR: *req.PriceIDR, CommissionBps: *req.CommissionBps,
		ConsignorID: *req.ConsignorID, StockOnHand: *req.StockOnHand, CertificateEnabled: req.CertificateEnabled,
	})
	if err != nil {
		api.Error(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/items/"+i.ID.String())
	api.JSON(w, http.StatusCreated, toResponse(i))
}

type updateRequest struct {
	Name               *string `json:"name"`
	PriceIDR           *int64  `json:"price_idr"`
	CommissionBps      *int32  `json:"commission_bps"`
	StockOnHand        *int32  `json:"stock_on_hand"`
	Active             *bool   `json:"active"`
	CertificateEnabled *bool   `json:"certificate_enabled"`
}

func (u updateRequest) validate() error {
	var v api.Validator
	empty := true
	if u.Name != nil {
		empty = false
		v.Text("name", *u.Name, 1, 200)
	}
	if u.PriceIDR != nil {
		empty = false
		v.Range("price_idr", *u.PriceIDR, 1, maxPriceIDR)
	}
	if u.CommissionBps != nil {
		empty = false
		v.Range("commission_bps", int64(*u.CommissionBps), 0, maxBps)
	}
	if u.StockOnHand != nil {
		empty = false
		v.Range("stock_on_hand", int64(*u.StockOnHand), 0, maxStock)
	}
	if u.Active != nil || u.CertificateEnabled != nil {
		empty = false
	}
	if empty {
		v.Add("body", "Kirim minimal satu kolom.")
	}
	return v.Err()
}

// Update menjawab PATCH /api/v1/items/{item_id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := api.PathUUID(r, "item_id")
	if err != nil {
		api.Error(w, r, err)
		return
	}
	var req updateRequest
	if err := api.Decode(w, r, &req); err != nil {
		api.Error(w, r, err)
		return
	}
	if err := req.validate(); err != nil {
		api.Error(w, r, err)
		return
	}
	i, err := h.service.Update(r.Context(), auth.MustFromContext(r.Context()).StoreID, id, Update(req))
	if err != nil {
		api.Error(w, r, err)
		return
	}
	api.JSON(w, http.StatusOK, toResponse(i))
}
