package swaps

import (
	"errors"
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

// GET /api/swaps?status=&target=me&schedule=ID
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sub, _ := auth.FromContext(r.Context())
	opts := ListOpts{Status: r.URL.Query().Get("status")}
	if v := r.URL.Query().Get("target"); v == "me" {
		opts.TargetID = sub.NurseID
	}
	if v := r.URL.Query().Get("requester"); v == "me" {
		opts.RequesterID = sub.NurseID
	}
	if v := r.URL.Query().Get("schedule"); v != "" {
		if id, err := strconv.Atoi(v); err == nil {
			opts.ScheduleID = id
		}
	}
	out, err := h.Repo.List(r.Context(), opts)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// POST /api/swaps
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in CreateInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	sub, _ := auth.FromContext(r.Context())
	out, err := h.Svc.Create(r.Context(), sub, in)
	if err != nil {
		writeTransitionErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

// PATCH /api/swaps/{id} {action: accept|reject|approve}
func (h *Handler) Patch(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid id")
		return
	}
	var in PatchInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	sub, _ := auth.FromContext(r.Context())

	switch in.Action {
	case "accept":
		out, err := h.Svc.Accept(r.Context(), sub, id)
		if err != nil {
			writeTransitionErr(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, out)
	case "reject":
		out, err := h.Svc.RejectByB(r.Context(), sub, id, in.Reason)
		if err != nil {
			writeTransitionErr(w, err)
			return
		}
		httpx.JSON(w, http.StatusOK, out)
	case "approve":
		out, vOut, err := h.Svc.Approve(r.Context(), sub, id)
		if err != nil {
			if vOut != nil {
				// X-03 거부
				apierr.WriteFull(w, http.StatusUnprocessableEntity, &apierr.Error{
					Code:    apierr.CodeRuleViolation,
					Message: "swap 결과 hard 위반",
					Details: map[string]any{
						"violations": vOut.Violations,
						"hard_count": vOut.HardCount,
					},
				})
				return
			}
			writeTransitionErr(w, err)
			return
		}
		httpx.JSONWithMeta(w, http.StatusOK, out, map[string]any{
			"violations": vOut.Violations,
			"hard_count": vOut.HardCount,
			"soft_count": vOut.SoftCount,
		})
	default:
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "action must be accept|reject|approve")
	}
}

// POST /api/swaps/{id}/cancel
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid id")
		return
	}
	sub, _ := auth.FromContext(r.Context())
	out, err := h.Svc.Cancel(r.Context(), sub, id)
	if err != nil {
		writeTransitionErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func writeTransitionErr(w http.ResponseWriter, err error) {
	var te *TransitionError
	if errors.As(err, &te) {
		switch te.Code {
		case "FORBIDDEN":
			apierr.Write(w, http.StatusForbidden, apierr.CodeForbidden, te.Msg)
		case "CONFLICT":
			apierr.Write(w, http.StatusConflict, apierr.CodeConflict, te.Msg)
		case "VALIDATION_ERROR":
			apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, te.Msg)
		case "RULE_VIOLATION":
			apierr.Rule(w, "X-03", te.Msg, nil)
		default:
			apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, te.Msg)
		}
		return
	}
	if errors.Is(err, ErrNotFound) {
		apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "swap not found")
		return
	}
	apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
}
