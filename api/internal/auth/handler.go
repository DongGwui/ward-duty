// /api/auth/* 핸들러.
// Design Ref: §4.1, §7
package auth

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"ward-duty-api/pkg/apierr"
	"ward-duty-api/pkg/httpx"
)

// strings는 frontPath 검증에서 사용
var _ = strings.HasPrefix

type Handler struct {
	PG    *pgxpool.Pool
	Redis *redis.Client
	OAuth *OAuthConfig
}

func New(pg *pgxpool.Pool, rc *redis.Client, oc *OAuthConfig) *Handler {
	return &Handler{PG: pg, Redis: rc, OAuth: oc}
}

// GET /api/auth/oauth/google/start
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	url, err := h.OAuth.StartURL(w)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	http.Redirect(w, r, url, http.StatusFound)
}

// GET /api/auth/oauth/google/callback
func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer ClearOAuthCookies(w)

	user, err := h.OAuth.HandleCallback(ctx, r)
	if err != nil {
		apierr.Write(w, http.StatusUnauthorized, apierr.CodeOAuthState, err.Error())
		return
	}

	sub, err := h.upsertNurseByEmail(ctx, user)
	if err != nil {
		if errors.Is(err, errNotInvited) {
			apierr.Write(w, http.StatusForbidden, apierr.CodeNotInvited,
				"초대받지 않은 이메일입니다. 수간호사에게 등록을 요청하세요.")
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
	refresh, refreshExp, err := IssueRefresh(ctx, h.Redis, *sub)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	// C2 fix: access도 httpOnly 쿠키 (fragment 노출 제거)
	SetAccessCookie(w, access, accessExp)
	SetRefreshCookie(w, refresh, refreshExp)

	http.Redirect(w, r, frontPath(), http.StatusFound)
}

// POST /api/auth/refresh
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	raw := GetRefreshCookie(r)
	if raw == "" {
		apierr.Write(w, http.StatusUnauthorized, apierr.CodeUnauthorized, "missing refresh cookie")
		return
	}

	// C3 fix: GETDEL atomic consume — hint 불필요
	sub, err := ConsumeRefresh(ctx, h.Redis, raw, 0)
	if err != nil {
		apierr.Write(w, http.StatusUnauthorized, apierr.CodeUnauthorized, err.Error())
		return
	}
	access, accessExp, err := IssueAccess(*sub)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	newRefresh, refreshExp, err := IssueRefresh(ctx, h.Redis, *sub)
	if err != nil {
		apierr.Write(w, http.StatusInternalServerError, apierr.CodeInternal, err.Error())
		return
	}
	SetAccessCookie(w, access, accessExp)
	SetRefreshCookie(w, newRefresh, refreshExp)
	httpx.JSON(w, http.StatusOK, map[string]any{"subject": sub})
}

// POST /api/auth/logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	sub, _ := FromContext(r.Context())
	if sub != nil {
		_ = RevokeAllRefresh(r.Context(), h.Redis, sub.NurseID)
	}
	ClearAccessCookie(w)
	ClearRefreshCookie(w)
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GET /api/auth/me
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	sub, ok := FromContext(r.Context())
	if !ok {
		apierr.Write(w, http.StatusUnauthorized, apierr.CodeUnauthorized, "not authenticated")
		return
	}
	httpx.JSON(w, http.StatusOK, sub)
}

// ----- internal -----

var errNotInvited = errors.New("not invited")

// upsertNurseByEmail — FR-10 화이트리스트.
//
//   1) email 일치 + active → google_sub upsert + last_login_at 갱신
//   2) email 없음 + email == ADMIN_EMAIL → head_nurse 자동 시드
//   3) 그 외 → errNotInvited
func (h *Handler) upsertNurseByEmail(ctx context.Context, u *GoogleUserInfo) (*Subject, error) {
	email := strings.ToLower(strings.TrimSpace(u.Email))

	row := h.PG.QueryRow(ctx, `SELECT id, role, active FROM nurses WHERE email = $1`, email)
	var id int
	var role string
	var active bool
	err := row.Scan(&id, &role, &active)
	switch {
	case err == nil:
		if !active {
			return nil, errors.New("nurse deactivated")
		}
		if _, err := h.PG.Exec(ctx,
			`UPDATE nurses SET google_sub = $1, last_login_at = NOW() WHERE id = $2`,
			u.Sub, id); err != nil {
			return nil, err
		}
		return &Subject{NurseID: id, Email: email, Role: role}, nil

	case errors.Is(err, pgx.ErrNoRows):
		// 화이트리스트에 없음 → ADMIN_EMAIL 자동 시드만 허용
		if h.OAuth.AdminEmail != "" && email == h.OAuth.AdminEmail {
			name := u.Name
			if name == "" {
				name = "Admin"
			}
			if err := h.PG.QueryRow(ctx,
				`INSERT INTO nurses (name, role, email, google_sub, active, last_login_at)
				 VALUES ($1, 'head_nurse', $2, $3, TRUE, NOW())
				 RETURNING id`,
				name, email, u.Sub).Scan(&id); err != nil {
				return nil, err
			}
			return &Subject{NurseID: id, Email: email, Role: "head_nurse"}, nil
		}
		return nil, errNotInvited

	default:
		return nil, err
	}
}

func frontPath() string {
	if v := os.Getenv("FRONTEND_REDIRECT_PATH"); v != "" {
		return v
	}
	return "/"
}

// silence — context unused 방지
var _ = context.TODO
