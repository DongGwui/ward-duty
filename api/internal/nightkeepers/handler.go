package nightkeepers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ward-duty-api/internal/auth"
	"ward-duty-api/pkg/apierr"
	"ward-duty-api/pkg/httpx"
)

type Handler struct{ Repo *Repo }

func New(pg *pgxpool.Pool) *Handler { return &Handler{Repo: NewRepo(pg)} }

// GET /api/night-keepers?ym=YYYY-MM
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ym := r.URL.Query().Get("ym")
	if err := IsValidYM(ym); err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, err.Error())
		return
	}
	rows, err := h.Repo.ListByMonth(r.Context(), ym)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, rows)
}

// POST /api/night-keepers (head_nurse)
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if err := IsValidYM(in.YearMonth); err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, err.Error())
		return
	}
	if in.NurseID <= 0 {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "nurse_id required")
		return
	}
	// K-02 / K-04 / K-05 사전 검증
	if err := Validate(r.Context(), h.Repo, in.NurseID, in.YearMonth); err != nil {
		var ve *ValidationError
		if errors.As(err, &ve) {
			apierr.Rule(w, ve.RuleID, ve.Message, map[string]any{
				"nurse_id":   in.NurseID,
				"year_month": in.YearMonth,
			})
			return
		}
		if errors.Is(err, ErrNurseNotFound) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "nurse not found")
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}

	sub, _ := auth.FromContext(r.Context())
	assignedBy := 0
	if sub != nil {
		assignedBy = sub.NurseID
	}
	a, err := h.Repo.Create(r.Context(), in, assignedBy)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusCreated, a)
}

// DELETE /api/night-keepers/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid id")
		return
	}
	if err := h.Repo.Delete(r.Context(), id); err != nil {
		if errors.Is(err, ErrNotFound) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "assignment not found")
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
