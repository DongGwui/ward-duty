package wishes

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ward-duty-api/internal/auth"
	"ward-duty-api/pkg/apierr"
	"ward-duty-api/pkg/httpx"
)

type Handler struct{ Repo *Repo }

func New(pg *pgxpool.Pool) *Handler { return &Handler{Repo: NewRepo(pg)} }

var validTypes = map[string]bool{
	"off": true, "d": true, "e": true, "n": true, "unavailable": true,
}

// GET /api/wishes?ym=YYYY-MM&nurse=ID
//
// nurse 권한: 본인 데이터만. head_nurse: 전체 또는 ?nurse= 필터.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sub, _ := auth.FromContext(r.Context())
	ym := r.URL.Query().Get("ym")
	y, m, err := parseYM(ym)
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "ym=YYYY-MM required")
		return
	}
	var nurseFilter int
	if q := r.URL.Query().Get("nurse"); q != "" {
		n, err := strconv.Atoi(q)
		if err != nil || n <= 0 {
			apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid nurse")
			return
		}
		nurseFilter = n
	}
	// 권한: nurse는 본인만
	if sub != nil && sub.Role != "head_nurse" {
		nurseFilter = sub.NurseID
	}
	out, err := h.Repo.ListByMonth(r.Context(), y, m, nurseFilter)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// PUT /api/wishes/{date}
//
// date는 YYYY-MM-DD. 본인 wishes만 upsert (head_nurse는 자기 자신만).
func (h *Handler) Upsert(w http.ResponseWriter, r *http.Request) {
	sub, _ := auth.FromContext(r.Context())
	dateStr := chi.URLParam(r, "date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "date YYYY-MM-DD required")
		return
	}
	var in UpsertInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if !validTypes[in.Type] {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation,
			"type must be off|d|e|n|unavailable")
		return
	}
	wsh, err := h.Repo.Upsert(r.Context(), sub.NurseID, date, in)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, wsh)
}

// DELETE /api/wishes/{date}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	sub, _ := auth.FromContext(r.Context())
	dateStr := chi.URLParam(r, "date")
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "date YYYY-MM-DD required")
		return
	}
	if err := h.Repo.Delete(r.Context(), sub.NurseID, date); err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func parseYM(ym string) (year, month int, err error) {
	if len(ym) != 7 || ym[4] != '-' {
		return 0, 0, fmt.Errorf("invalid format")
	}
	y, err1 := strconv.Atoi(ym[:4])
	m, err2 := strconv.Atoi(ym[5:])
	if err1 != nil || err2 != nil || m < 1 || m > 12 {
		return 0, 0, fmt.Errorf("invalid value")
	}
	return y, m, nil
}
