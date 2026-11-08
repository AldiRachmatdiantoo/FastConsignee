// Package consignor mengurus pendaftaran penitip oleh pemilik toko.
package consignor

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/auth"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/apperr"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/database"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

// Consignor adalah penitip beserta email akunnya.
type Consignor struct {
	ID               uuid.UUID
	Name             string
	Email            string
	WalletAddress    *string
	WalletVerifiedAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// WalletStatus dihitung dari alamat dan bukti tanda tangan.
func (c Consignor) WalletStatus() string {
	switch {
	case c.WalletAddress == nil:
		return "unlinked"
	case c.WalletVerifiedAt == nil:
		return "unverified"
	default:
		return "verified"
	}
}

// Repository berisi query penitip. Semua query dibatasi per toko.
type Repository struct {
	db *database.DB
}

// NewRepository membuat repository Postgres.
func NewRepository(db *database.DB) *Repository {
	return &Repository{db: db}
}

const selectConsignor = `
	SELECT c.id, c.name, u.email, c.wallet_address, c.wallet_verified_at, c.created_at, c.updated_at
	FROM consignors c
	JOIN users u ON u.id = c.user_id
`

func scan(row pgx.Row) (Consignor, error) {
	var c Consignor
	err := row.Scan(&c.ID, &c.Name, &c.Email, &c.WalletAddress, &c.WalletVerifiedAt, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (r *Repository) list(ctx context.Context, storeID uuid.UUID, p api.Page) ([]Consignor, error) {
	sql := selectConsignor + ` WHERE c.store_id = $1`
	args := []any{storeID}
	if p.Cursor != nil {
		sql += ` AND (c.created_at, c.id) < ($2, $3)`
		args = append(args, p.Cursor.Time, p.Cursor.ID)
	}
	args = append(args, p.Limit+1)
	sql += fmt.Sprintf(` ORDER BY c.created_at DESC, c.id DESC LIMIT $%d`, len(args))
	rows, err := r.db.Q(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("daftar penitip: %w", err)
	}
	defer rows.Close()
	out := []Consignor{}
	for rows.Next() {
		c, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("baca penitip: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *Repository) get(ctx context.Context, storeID, id uuid.UUID) (*Consignor, error) {
	c, err := scan(r.db.Q(ctx).QueryRow(ctx, selectConsignor+` WHERE c.store_id = $1 AND c.id = $2`, storeID, id))
	if database.IsNoRows(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("baca penitip: %w", err)
	}
	return &c, nil
}

func (r *Repository) walletTaken(ctx context.Context, address string) (bool, error) {
	var taken bool
	err := r.db.Q(ctx).QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM stores WHERE wallet_address = $1)
		    OR EXISTS (SELECT 1 FROM consignors WHERE wallet_address = $1)`, address).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("cek wallet: %w", err)
	}
	return taken, nil
}

func (r *Repository) insert(ctx context.Context, storeID, id uuid.UUID, in createInput) error {
	userID := uuid.New()
	q := r.db.Q(ctx)
	if _, err := q.Exec(ctx,
		`INSERT INTO users (id, store_id, role, email, name) VALUES ($1, $2, 'consignor', $3, $4)`,
		userID, storeID, in.Email, in.Name); err != nil {
		return fmt.Errorf("simpan akun penitip: %w", err)
	}
	if _, err := q.Exec(ctx,
		`INSERT INTO consignors (id, store_id, user_id, name, wallet_address) VALUES ($1, $2, $3, $4, $5)`,
		id, storeID, userID, in.Name, in.WalletAddress); err != nil {
		return fmt.Errorf("simpan penitip: %w", err)
	}
	return nil
}

func (r *Repository) rename(ctx context.Context, storeID, id uuid.UUID, name string) (bool, error) {
	tag, err := r.db.Q(ctx).Exec(ctx,
		`UPDATE consignors SET name = $3, updated_at = now() WHERE id = $2 AND store_id = $1`, storeID, id, name)
	if err != nil {
		return false, fmt.Errorf("ubah penitip: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// Service memegang aturan pendaftaran penitip.
type Service struct {
	repo *Repository
	tx   database.TxRunner
}

// NewService membuat service penitip.
func NewService(repo *Repository, tx database.TxRunner) *Service {
	return &Service{repo: repo, tx: tx}
}

type createInput struct {
	Name          string
	Email         string
	WalletAddress *string
}

func notFound() error {
	return apperr.New(apperr.NotFound, "Consignor not found.")
}

func (s *Service) get(ctx context.Context, storeID, id uuid.UUID) (Consignor, error) {
	c, err := s.repo.get(ctx, storeID, id)
	if err != nil {
		return Consignor{}, err
	}
	if c == nil {
		return Consignor{}, notFound()
	}
	return *c, nil
}

// Akun dan profil penitip disimpan dalam satu transaksi. Alamat wallet berstatus unverified
// sampai penitip menandatanganinya sendiri.
func (s *Service) create(ctx context.Context, storeID uuid.UUID, in createInput) (Consignor, error) {
	id := uuid.New()
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		if in.WalletAddress != nil {
			taken, err := s.repo.walletTaken(ctx, *in.WalletAddress)
			if err != nil {
				return err
			}
			if taken {
				return apperr.New(apperr.WalletAlreadyLinked, "This wallet is already linked to someone else.")
			}
		}
		return s.repo.insert(ctx, storeID, id, in)
	})
	switch {
	case database.IsUniqueViolation(err, "users_email_key"):
		return Consignor{}, apperr.New(apperr.ConsignorEmailTaken, "This email is already registered.")
	case database.IsUniqueViolation(err, "consignors_wallet_address_key"):
		return Consignor{}, apperr.New(apperr.WalletAlreadyLinked, "This wallet is already linked to someone else.")
	case err != nil:
		return Consignor{}, err
	}
	return s.get(ctx, storeID, id)
}

func (s *Service) rename(ctx context.Context, storeID, id uuid.UUID, name string) (Consignor, error) {
	ok, err := s.repo.rename(ctx, storeID, id, name)
	if err != nil {
		return Consignor{}, err
	}
	if !ok {
		return Consignor{}, notFound()
	}
	return s.get(ctx, storeID, id)
}

// Handler berisi endpoint penitip.
type Handler struct {
	service *Service
}

// NewHandler membuat handler penitip.
func NewHandler(s *Service) *Handler {
	return &Handler{service: s}
}

type response struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	WalletAddress *string   `json:"wallet_address"`
	WalletStatus  string    `json:"wallet_status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toResponse(c Consignor) response {
	return response{
		ID: c.ID, Name: c.Name, Email: c.Email, WalletAddress: c.WalletAddress, WalletStatus: c.WalletStatus(),
		CreatedAt: c.CreatedAt.UTC(), UpdatedAt: c.UpdatedAt.UTC(),
	}
}

// List menjawab GET /api/v1/consignors.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	page, err := api.ParsePage(r)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	rows, err := h.service.repo.list(r.Context(), auth.MustFromContext(r.Context()).StoreID, page)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	rows, next := api.NextCursor(rows, page.Limit, func(c Consignor) (time.Time, uuid.UUID) { return c.CreatedAt, c.ID })
	data := make([]response, 0, len(rows))
	for _, c := range rows {
		data = append(data, toResponse(c))
	}
	api.JSON(w, http.StatusOK, map[string]any{"data": data, "next_cursor": next})
}

// Get menjawab GET /api/v1/consignors/{consignor_id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := api.PathUUID(r, "consignor_id")
	if err != nil {
		api.Error(w, r, err)
		return
	}
	c, err := h.service.get(r.Context(), auth.MustFromContext(r.Context()).StoreID, id)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	api.JSON(w, http.StatusOK, toResponse(c))
}

type createRequest struct {
	Name          string  `json:"name"`
	Email         string  `json:"email"`
	WalletAddress *string `json:"wallet_address"`
}

// Create menjawab POST /api/v1/consignors.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := api.Decode(w, r, &req); err != nil {
		api.Error(w, r, err)
		return
	}
	var v api.Validator
	v.Text("name", req.Name, 1, 200)
	v.Email("email", req.Email)
	if req.WalletAddress != nil && !solana.ValidAddress(*req.WalletAddress) {
		v.Add("wallet_address", "Alamat Solana tidak valid.")
	}
	if err := v.Err(); err != nil {
		api.Error(w, r, err)
		return
	}
	c, err := h.service.create(r.Context(), auth.MustFromContext(r.Context()).StoreID, createInput{
		Name: req.Name, Email: strings.ToLower(req.Email), WalletAddress: req.WalletAddress,
	})
	if err != nil {
		api.Error(w, r, err)
		return
	}
	w.Header().Set("Location", "/api/v1/consignors/"+c.ID.String())
	api.JSON(w, http.StatusCreated, toResponse(c))
}

type updateRequest struct {
	Name string `json:"name"`
}

// Update menjawab PATCH /api/v1/consignors/{consignor_id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := api.PathUUID(r, "consignor_id")
	if err != nil {
		api.Error(w, r, err)
		return
	}
	var req updateRequest
	if err := api.Decode(w, r, &req); err != nil {
		api.Error(w, r, err)
		return
	}
	var v api.Validator
	v.Text("name", req.Name, 1, 200)
	if err := v.Err(); err != nil {
		api.Error(w, r, err)
		return
	}
	c, err := h.service.rename(r.Context(), auth.MustFromContext(r.Context()).StoreID, id, req.Name)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	api.JSON(w, http.StatusOK, toResponse(c))
}
