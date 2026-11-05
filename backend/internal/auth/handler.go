package auth

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
)

// Handler berisi endpoint akun dan wallet.
type Handler struct {
	service *Service
}

// NewHandler membuat handler auth.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

type storeRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type walletLink struct {
	Address  string    `json:"address"`
	LinkedAt time.Time `json:"linked_at"`
}

type meResponse struct {
	ID             uuid.UUID   `json:"id"`
	Email          string      `json:"email"`
	Name           string      `json:"name"`
	Role           Role        `json:"role"`
	Store          storeRef    `json:"store"`
	ConsignorID    *uuid.UUID  `json:"consignor_id"`
	Wallet         *walletLink `json:"wallet"`
	WalletRequired bool        `json:"wallet_required"`
}

// Me menjawab GET /api/v1/me.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	p := MustFromContext(r.Context())
	out := meResponse{
		ID:             p.UserID,
		Email:          p.Email,
		Name:           p.Name,
		Role:           p.Role,
		Store:          storeRef{ID: p.StoreID, Name: p.StoreName},
		ConsignorID:    p.ConsignorID,
		WalletRequired: p.WalletRequired(),
	}
	if p.WalletAddress != nil && p.WalletLinkedAt != nil {
		out.Wallet = &walletLink{Address: *p.WalletAddress, LinkedAt: p.WalletLinkedAt.UTC()}
	}
	api.JSON(w, http.StatusOK, out)
}

type nonceRequest struct {
	Address string `json:"address"`
}

type nonceResponse struct {
	Nonce     string    `json:"nonce"`
	Message   string    `json:"message"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CreateNonce menjawab POST /api/v1/wallet/nonce.
func (h *Handler) CreateNonce(w http.ResponseWriter, r *http.Request) {
	var req nonceRequest
	if err := api.Decode(w, r, &req); err != nil {
		api.Error(w, r, err)
		return
	}
	n, message, err := h.service.CreateNonce(r.Context(), MustFromContext(r.Context()), req.Address)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	api.JSON(w, http.StatusCreated, nonceResponse{Nonce: n.Value, Message: message, ExpiresAt: n.ExpiresAt})
}

type linkRequest struct {
	Address   string `json:"address"`
	Nonce     string `json:"nonce"`
	Signature string `json:"signature"`
}

// LinkWallet menjawab POST /api/v1/wallet/link.
func (h *Handler) LinkWallet(w http.ResponseWriter, r *http.Request) {
	var req linkRequest
	if err := api.Decode(w, r, &req); err != nil {
		api.Error(w, r, err)
		return
	}
	var v api.Validator
	v.Address("address", req.Address)
	v.Text("nonce", req.Nonce, 1, 64)
	v.Signature("signature", req.Signature)
	if err := v.Err(); err != nil {
		api.Error(w, r, err)
		return
	}
	address, at, err := h.service.LinkWallet(r.Context(), MustFromContext(r.Context()), req.Address, req.Nonce, req.Signature)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	api.JSON(w, http.StatusOK, walletLink{Address: address, LinkedAt: at})
}
