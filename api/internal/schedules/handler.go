package schedules

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"ward-duty-api/internal/auth"
	"ward-duty-api/internal/solver"
	"ward-duty-api/pkg/apierr"
	"ward-duty-api/pkg/httpx"
)

type Handler struct {
	Repo *Repo
	Svc  *Service
}

func New(pg *pgxpool.Pool, sc *solver.Client) *Handler {
	return &Handler{Repo: NewRepo(pg), Svc: NewService(pg, sc)}
}

// POST /api/schedules { year_month }  → 202
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if err := validYM(in.YearMonth); err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, err.Error())
		return
	}
	sch, err := h.Svc.Generate(r.Context(), in.YearMonth)
	if err != nil {
		if errors.Is(err, ErrAlreadyConfirmed) {
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, "이미 확정된 schedule입니다")
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusAccepted, sch)
}

// GET /api/schedules?ym=YYYY-MM
func (h *Handler) GetByYM(w http.ResponseWriter, r *http.Request) {
	ym := r.URL.Query().Get("ym")
	if err := validYM(ym); err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, err.Error())
		return
	}
	sch, err := h.Repo.GetByYM(r.Context(), ym)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "schedule not found")
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, sch)
}

// GET /api/schedules/{id}/cells
func (h *Handler) ListCells(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid id")
		return
	}
	cells, err := h.Repo.ListCells(r.Context(), id)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, cells)
}

// PATCH /api/schedules/{id}/cells/{cellId}  { shift }
//
// Design §2.2 B: 수정 → solver /validate → violations meta로 반환.
func (h *Handler) PatchCell(w http.ResponseWriter, r *http.Request) {
	scheduleID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid id")
		return
	}
	cellID, err := strconv.Atoi(chi.URLParam(r, "cellId"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid cellId")
		return
	}
	var in PatchCellInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if !validShift(in.Shift) {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "shift must be D|E|N|O|DE")
		return
	}
	sub, _ := auth.FromContext(r.Context())
	modifiedBy := 0
	if sub != nil {
		modifiedBy = sub.NurseID
	}
	cell, err := h.Repo.UpdateCell(r.Context(), cellID, in.Shift, modifiedBy)
	if err != nil {
		if errors.Is(err, ErrCellNotFound) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "cell not found")
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	// 검증
	sch, err := h.Repo.Get(r.Context(), scheduleID)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	vOut, err := h.Svc.ValidateAfterChange(r.Context(), scheduleID, sch.YearMonth)
	meta := map[string]any{}
	if err != nil {
		// 솔버 단점 — 변경은 이미 적용. 위반은 unknown.
		meta["validate_error"] = err.Error()
	} else {
		meta["violations"] = vOut.Violations
		meta["hard_count"] = vOut.HardCount
		meta["soft_count"] = vOut.SoftCount
	}
	httpx.JSONWithMeta(w, http.StatusOK, cell, meta)
}

// POST /api/schedules/{id}/reset — 'generating'/'failed' 상태를 'draft'로 되돌림 (C5 보조).
func (h *Handler) Reset(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid id")
		return
	}
	if err := h.Svc.ForceReset(r.Context(), id); err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	sch, _ := h.Repo.Get(r.Context(), id)
	httpx.JSON(w, http.StatusOK, sch)
}

// POST /api/schedules/{id}/confirm
func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid id")
		return
	}
	err = h.Svc.Confirm(r.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "schedule not found")
			return
		}
		if errors.Is(err, ErrCannotConfirm) {
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, "only 'generated' status can be confirmed")
			return
		}
		var hv *HardViolationsError
		if errors.As(err, &hv) {
			apierr.WriteFull(w, http.StatusUnprocessableEntity, &apierr.Error{
				Code:    apierr.CodeRuleViolation,
				Message: "hard violations exist",
				Details: map[string]any{
					"hard_count": hv.HardCount,
					"violations": hv.Violations,
				},
			})
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	sch, _ := h.Repo.Get(r.Context(), id)
	httpx.JSON(w, http.StatusOK, sch)
}

// ----- helpers -----

func validYM(ym string) error {
	if len(ym) != 7 || ym[4] != '-' {
		return fmt.Errorf("year_month must be YYYY-MM")
	}
	y, err1 := strconv.Atoi(ym[:4])
	m, err2 := strconv.Atoi(ym[5:])
	if err1 != nil || err2 != nil || m < 1 || m > 12 || y < 2000 || y > 2100 {
		return fmt.Errorf("invalid year_month")
	}
	return nil
}

func validShift(s string) bool {
	switch s {
	case "D", "E", "N", "O", "DE":
		return true
	}
	return false
}
