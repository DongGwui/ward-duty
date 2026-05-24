package notifications

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

// GET /api/notifications?unread_only=1&limit=20
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	sub, _ := auth.FromContext(r.Context())
	unread := r.URL.Query().Get("unread_only") == "1"
	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	rows, err := h.Repo.List(r.Context(), sub.NurseID, unread, limit)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	unreadCount, _ := h.Repo.UnreadCount(r.Context(), sub.NurseID)
	httpx.JSONWithMeta(w, http.StatusOK, rows, map[string]any{"unread_count": unreadCount})
}

// GET /api/notifications/unread-count
func (h *Handler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	sub, _ := auth.FromContext(r.Context())
	c, err := h.Repo.UnreadCount(r.Context(), sub.NurseID)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int{"unread_count": c})
}

// POST /api/notifications/{id}/read
func (h *Handler) Read(w http.ResponseWriter, r *http.Request) {
	sub, _ := auth.FromContext(r.Context())
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid id")
		return
	}
	if err := h.Repo.MarkRead(r.Context(), sub.NurseID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "notification not found")
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/notifications/read-all
func (h *Handler) ReadAll(w http.ResponseWriter, r *http.Request) {
	sub, _ := auth.FromContext(r.Context())
	n, err := h.Repo.MarkAllRead(r.Context(), sub.NurseID)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]int{"marked_read": n})
}

// DELETE /api/notifications/{id}
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	sub, _ := auth.FromContext(r.Context())
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid id")
		return
	}
	if err := h.Repo.Delete(r.Context(), sub.NurseID, id); err != nil {
		if errors.Is(err, ErrNotFound) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "notification not found")
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
