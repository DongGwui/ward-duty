package settings

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"ward-duty-api/pkg/apierr"
	"ward-duty-api/pkg/httpx"
)

type Handler struct{ Repo *Repo }

func New(pg *pgxpool.Pool) *Handler { return &Handler{Repo: NewRepo(pg)} }

// GET /api/settings
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	s, err := h.Repo.Get(r.Context())
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

// PATCH /api/settings (head_nurse)
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var in UpdateInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	s, err := h.Repo.Update(r.Context(), in)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}
