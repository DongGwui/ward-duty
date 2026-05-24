// dev-login bypass — ENV=development 환경에서만 활성.
//
// 운영(production) 빌드/실행에선 라우트 자체가 등록되지 않으므로 노출 0.
// 화이트리스트(nurses) 또는 ADMIN_EMAIL 분기로 동일하게 처리 — OAuth와 같은 보안 모델 유지.
package auth

import (
	"context"
	"net/http"
	"os"
	"strings"

	"ward-duty-api/pkg/apierr"
	"ward-duty-api/pkg/httpx"
)

// IsDevMode — main.go가 라우트 등록 시 가드로 사용.
func IsDevMode() bool {
	v := strings.ToLower(os.Getenv("ENV"))
	return v == "" || v == "development" || v == "dev" || v == "local"
}

// DevLogin — POST /api/auth/dev-login { email }
//
// OAuth와 동일한 화이트리스트 체크 + JWT/refresh 쿠키 발급.
// production에선 main.go가 이 라우트를 등록하지 않음.
func (h *Handler) DevLogin(w http.ResponseWriter, r *http.Request) {
	if !IsDevMode() {
		// 방어적 — 라우트 등록 가드를 우회한 경우
		apierr.Write(w, http.StatusNotFound, apierr.CodeNotFound, "not available in production")
		return
	}

	var body struct {
		Email string `json:"email"`
		Name  string `json:"name,omitempty"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	email := strings.ToLower(strings.TrimSpace(body.Email))
	if email == "" {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, "email required")
		return
	}

	// OAuth callback과 동일 로직 재사용
	user := &GoogleUserInfo{
		Sub:           "dev-" + email,
		Email:         email,
		EmailVerified: true,
		Name:          body.Name,
	}
	sub, err := h.upsertNurseByEmail(r.Context(), user)
	if err != nil {
		if err == errNotInvited {
			// Stage 2: dev-login도 pending 큐에 저장 (매니저 매칭 흐름 테스트)
			_ = SavePending(r.Context(), h.Redis, &PendingAccount{
				Email:     user.Email,
				GoogleSub: user.Sub,
				Name:      user.Name,
				Picture:   user.Picture,
			})
			apierr.WriteFull(w, http.StatusForbidden, &apierr.Error{
				Code:    "PENDING_APPROVAL",
				Message: "매니저가 명단에 연결할 때까지 대기 중입니다.",
				Details: map[string]any{"email": user.Email},
			})
			return
		}
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}

	access, accessExp, err := IssueAccess(*sub)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	refresh, refreshExp, err := IssueRefresh(r.Context(), h.Redis, *sub)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	SetAccessCookie(w, access, accessExp)
	SetRefreshCookie(w, refresh, refreshExp)

	httpx.JSON(w, http.StatusOK, map[string]any{
		"subject": sub,
		"mode":    "dev-login",
	})
}

// silence unused import for older builds
var _ = context.TODO
