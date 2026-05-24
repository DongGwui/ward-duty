// Pending 계정 매칭 API — 매니저(head_nurse) 전용.
package auth

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"ward-duty-api/pkg/apierr"
	"ward-duty-api/pkg/httpx"
)

// GET /api/pending-accounts (head_nurse)
func (h *Handler) ListPending(w http.ResponseWriter, r *http.Request) {
	out, err := ListPending(r.Context(), h.Redis)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

// POST /api/nurses/{id}/link-account { email } (head_nurse)
//
// 기존 nurse 행에 email + google_sub 채워 계정 연결.
// 이미 다른 nurse에 같은 email이 있으면 409.
func (h *Handler) LinkAccount(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "invalid id")
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if email == "" {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "email required")
		return
	}

	p, err := GetPending(r.Context(), h.Redis, email)
	if err != nil {
		if errors.Is(err, ErrPendingNotFound) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "pending account not found (expired?)")
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}

	// 대상 nurse 행이 다른 이메일/sub을 들고 있으면 409 (덮어쓰기 방지)
	row := h.PG.QueryRow(r.Context(),
		`SELECT email, google_sub FROM nurses WHERE id = $1`, id)
	var existingEmail, existingSub *string
	if err := row.Scan(&existingEmail, &existingSub); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "nurse not found")
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	if existingEmail != nil && *existingEmail != "" && *existingEmail != email {
		apierr.Write(w, http.StatusConflict, apierr.CodeConflict,
			"해당 nurse는 이미 다른 이메일에 연결되어 있습니다")
		return
	}

	// 다른 nurse가 같은 email을 들고 있는지 (race)
	var otherID int
	err = h.PG.QueryRow(r.Context(),
		`SELECT id FROM nurses WHERE email = $1 AND id <> $2`, email, id).Scan(&otherID)
	if err == nil {
		apierr.Write(w, http.StatusConflict, apierr.CodeConflict,
			"이 이메일은 이미 다른 nurse에 연결되어 있습니다")
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}

	// 연결
	if _, err := h.PG.Exec(r.Context(),
		`UPDATE nurses SET email = $1, google_sub = $2, last_login_at = NOW() WHERE id = $3`,
		email, p.GoogleSub, id); err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	_ = DeletePending(r.Context(), h.Redis, email)
	httpx.JSON(w, http.StatusOK, map[string]any{
		"linked": true, "nurse_id": id, "email": email,
	})
}

// DELETE /api/pending-accounts/{email} (head_nurse) — 거부
func (h *Handler) DismissPending(w http.ResponseWriter, r *http.Request) {
	email := chi.URLParam(r, "email")
	if email == "" {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "email required")
		return
	}
	if err := DeletePending(r.Context(), h.Redis, email); err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
