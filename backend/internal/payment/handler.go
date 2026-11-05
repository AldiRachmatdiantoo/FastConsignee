package payment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/order"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

// Failure adalah error Solana Pay. Wallet hanya membaca `message`, jadi bentuknya bukan problem+json.
type Failure struct {
	Status  int
	Message string
}

func (f *Failure) Error() string { return f.Message }

// WriteError menulis error Solana Pay.
func WriteError(w http.ResponseWriter, status int, message string) {
	api.JSON(w, status, map[string]string{"message": message})
}

func writeFailure(w http.ResponseWriter, r *http.Request, err error) {
	var f *Failure
	if errors.As(err, &f) {
		WriteError(w, f.Status, f.Message)
		return
	}
	slog.Error("Solana Pay gagal", "path", r.URL.Path, "request_id", api.RequestID(r.Context()), "error", err)
	WriteError(w, http.StatusInternalServerError, "Terjadi kesalahan di server.")
}

// Service memegang aturan layar bayar dan Solana Pay.
type Service struct {
	repo    *Repository
	orders  *order.Repository
	solana  solana.Client
	iconURL string
	now     func() time.Time
}

// NewService membuat service pembayaran.
func NewService(repo *Repository, orders *order.Repository, client solana.Client, iconURL string, now func() time.Time) *Service {
	return &Service{repo: repo, orders: orders, solana: client, iconURL: iconURL, now: now}
}

func (s *Service) metadata(ctx context.Context, id uuid.UUID) (metadata, error) {
	m, err := s.repo.metadata(ctx, id)
	if err != nil {
		return metadata{}, err
	}
	if m == nil {
		return metadata{}, &Failure{http.StatusNotFound, "Order tidak ditemukan."}
	}
	if m.Icon == nil {
		m.Icon = &s.iconURL
	}
	return *m, nil
}

// transaction menyusun transaksi untuk wallet pembayar. Server tidak menandatanganinya.
func (s *Service) transaction(ctx context.Context, id uuid.UUID, account string) (string, string, error) {
	o, err := s.orders.Find(ctx, id)
	if err != nil {
		return "", "", err
	}
	if o == nil {
		return "", "", &Failure{http.StatusNotFound, "Order tidak ditemukan."}
	}
	switch {
	case o.Status == order.StatusConfirmed:
		return "", "", &Failure{http.StatusConflict, "Order ini sudah dibayar."}
	case o.Status == order.StatusFailed:
		return "", "", &Failure{http.StatusConflict, "Order ini gagal. Minta kasir membuat order baru."}
	case o.Status == order.StatusExpired || !s.now().Before(o.ExpiresAt):
		return "", "", &Failure{http.StatusGone, "Order ini sudah kedaluwarsa."}
	}
	transfers := make([]solana.Transfer, 0, len(o.Split.Recipients))
	for _, r := range o.Split.Recipients {
		if r.AmountLamports > 0 {
			transfers = append(transfers, solana.Transfer{From: account, To: r.Wallet, Lamports: uint64(r.AmountLamports)})
		}
	}
	blockhash, err := s.solana.LatestBlockhash(ctx)
	if err != nil {
		slog.Warn("blockhash tidak terbaca", "error", err)
		return "", "", &Failure{http.StatusServiceUnavailable, "Jaringan Solana tidak bisa dihubungi. Coba lagi."}
	}
	encoded, err := solana.BuildTransfers(account, blockhash, o.Reference, transfers)
	if err != nil {
		return "", "", fmt.Errorf("susun transaksi: %w", err)
	}
	return encoded, "Pembayaran di " + o.StoreName, nil
}

// Handler berisi endpoint publik pembayaran.
type Handler struct {
	service   *Service
	orders    *order.Repository
	publicURL string
}

// NewHandler membuat handler pembayaran.
func NewHandler(s *Service, orders *order.Repository, publicURL string) *Handler {
	return &Handler{service: s, orders: orders, publicURL: publicURL}
}

type publicLine struct {
	ItemName     string                 `json:"item_name"`
	Quantity     int32                  `json:"quantity"`
	LineTotalIDR int64                  `json:"line_total_idr"`
	Certificate  *order.CertificateView `json:"certificate"`
}

type publicOrder struct {
	ID            uuid.UUID    `json:"id"`
	Status        string       `json:"status"`
	StoreName     string       `json:"store_name"`
	Network       string       `json:"network"`
	Lines         []publicLine `json:"lines"`
	TotalIDR      int64        `json:"total_idr"`
	TotalLamports string       `json:"total_lamports"`
	Rate          any          `json:"rate"`
	SolanaPayURL  string       `json:"solana_pay_url"`
	ExpiresAt     time.Time    `json:"expires_at"`
	ConfirmedAt   *time.Time   `json:"confirmed_at"`
	Signature     *string      `json:"signature"`
	ExplorerURL   *string      `json:"explorer_url"`
	FailureCode   *string      `json:"failure_code"`
}

// PublicOrder menjawab GET /api/v1/public/orders/{order_id}. Nama penitip dan semua wallet tidak
// ikut dikirim, karena tautan ini dibuka tanpa login.
func (h *Handler) PublicOrder(w http.ResponseWriter, r *http.Request) {
	id, err := api.PathUUID(r, "order_id")
	if err != nil {
		api.Error(w, r, err)
		return
	}
	o, err := h.orders.Find(r.Context(), id)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	if o == nil {
		api.Error(w, r, apperr.New(apperr.NotFound, "Order tidak ditemukan."))
		return
	}
	lines := make([]publicLine, 0, len(o.Lines))
	for _, l := range o.Lines {
		lines = append(lines, publicLine{
			ItemName: l.ItemName, Quantity: l.Quantity, LineTotalIDR: l.Amounts.LineTotal,
			Certificate: order.ToCertificateView(l.Certificate),
		})
	}
	var confirmedAt *time.Time
	if o.ConfirmedAt != nil {
		t := o.ConfirmedAt.UTC()
		confirmedAt = &t
	}
	api.JSON(w, http.StatusOK, publicOrder{
		ID: o.ID, Status: o.Status, StoreName: o.StoreName, Network: order.Network, Lines: lines,
		TotalIDR: o.Split.TotalIDR, TotalLamports: order.Lamports(o.Split.TotalLamports), Rate: order.RateView(o.Rate),
		SolanaPayURL: order.SolanaPayURL(h.publicURL, o.ID), ExpiresAt: o.ExpiresAt.UTC(), ConfirmedAt: confirmedAt,
		Signature: o.Signature, ExplorerURL: order.ExplorerURL(o.Signature), FailureCode: o.FailureCode,
	})
}

func orderID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue("order_id"))
	if err != nil {
		WriteError(w, http.StatusBadRequest, "ID order tidak valid.")
		return uuid.Nil, false
	}
	return id, true
}

// Metadata menjawab GET Solana Pay.
func (h *Handler) Metadata(w http.ResponseWriter, r *http.Request) {
	id, ok := orderID(w, r)
	if !ok {
		return
	}
	m, err := h.service.metadata(r.Context(), id)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	api.JSON(w, http.StatusOK, map[string]string{"label": m.Label, "icon": *m.Icon})
}

// Transaction menjawab POST Solana Pay.
func (h *Handler) Transaction(w http.ResponseWriter, r *http.Request) {
	id, ok := orderID(w, r)
	if !ok {
		return
	}
	var req struct {
		Account string `json:"account"`
	}
	if err := api.DecodeLoose(w, r, &req); err != nil {
		WriteError(w, http.StatusBadRequest, "Isi permintaan tidak valid.")
		return
	}
	if !solana.ValidAddress(req.Account) {
		WriteError(w, http.StatusBadRequest, "Alamat wallet tidak valid.")
		return
	}
	encoded, message, err := h.service.transaction(r.Context(), id, req.Account)
	if err != nil {
		writeFailure(w, r, err)
		return
	}
	api.JSON(w, http.StatusOK, map[string]string{"transaction": encoded, "message": message})
}
