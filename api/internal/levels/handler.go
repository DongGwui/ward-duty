package levels

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ward-duty-api/pkg/apierr"
	"ward-duty-api/pkg/httpx"
)

type Handler struct {
	Repo *Repo
}

func New(pg *pgxpool.Pool) *Handler {
	return &Handler{Repo: NewRepo(pg)}
}

// GET /api/levels
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Repo.List(r.Context())
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, rows)
}

// POST /api/levels (head_nurse)
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if in.Code == "" || in.DisplayName == "" {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "code/display_name required")
		return
	}
	l, err := h.Repo.Create(r.Context(), in)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, l)
}

// PATCH /api/levels/{code}
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	var in UpdateInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	l, err := h.Repo.Update(r.Context(), code, in)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "level not found")
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, l)
}

// DELETE /api/levels/{code}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if err := h.Repo.Delete(r.Context(), code); err != nil {
		if errors.Is(err, ErrNotFound) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "level not found")
			return
		}
		if errors.Is(err, ErrLevelInUse) {
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, "이 등급을 사용하는 간호사가 있습니다")
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
