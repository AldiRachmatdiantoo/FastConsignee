package certificate

import (
	"fmt"
	"net/http"
	"time"

	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/api"
	"github.com/AldiRachmatdiantoo/FastConsignee/backend/internal/platform/solana"
)

// Handler menyajikan metadata yang dibaca wallet dan Explorer.
type Handler struct {
	repo    *Repository
	iconURL string
}

// NewHandler membuat handler sertifikat.
func NewHandler(repo *Repository, iconURL string) *Handler {
	return &Handler{repo: repo, iconURL: iconURL}
}

type attribute struct {
	TraitType string `json:"trait_type"`
	Value     string `json:"value"`
}

type metadataResponse struct {
	Name        string      `json:"name"`
	Symbol      string      `json:"symbol"`
	Description string      `json:"description"`
	Image       string      `json:"image"`
	Attributes  []attribute `json:"attributes"`
}

// Metadata menjawab format JSON standar Token Metadata. Endpoint ini publik, jadi tidak memuat nama
// penitip atau wallet pembeli.
func (h *Handler) Metadata(w http.ResponseWriter, r *http.Request) {
	id, err := api.PathUUID(r, "certificate_id")
	if err != nil {
		api.Error(w, r, err)
		return
	}
	d, err := h.repo.details(r.Context(), id)
	if err != nil {
		api.Error(w, r, err)
		return
	}
	attributes := []attribute{{"Barang", d.Item}, {"Toko", d.Store}}
	if d.ConfirmedAt != nil {
		attributes = append(attributes, attribute{"Purchase date", d.ConfirmedAt.UTC().Format(time.DateOnly)})
	}
	if d.Signature != nil {
		attributes = append(attributes, attribute{"Payment proof", *d.Signature})
	}
	api.JSON(w, http.StatusOK, metadataResponse{
		Name:        solana.CertificateName(d.Item),
		Symbol:      solana.CertificateSymbol,
		Description: fmt.Sprintf("Certificate of authenticity for %s from %s.", d.Item, d.Store),
		Image:       h.iconURL,
		Attributes:  attributes,
	})
}
