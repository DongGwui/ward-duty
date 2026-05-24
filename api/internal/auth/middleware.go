package auth

import (
	"context"
	"net/http"
	"strings"

	"ward-duty-api/pkg/apierr"
)

type ctxKey string

const ctxSubjectKey ctxKey = "auth.subject"

// FromContext — 핸들러에서 현재 사용자 얻기.
func FromContext(ctx context.Context) (*Subject, bool) {
	v, ok := ctx.Value(ctxSubjectKey).(*Subject)
	return v, ok
}

// RequireAuth — Bearer 헤더 또는 wd_access 쿠키로 인증 (C2 fix).
//
// 우선순위: Authorization 헤더 > wd_access 쿠키.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := ""
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token = strings.TrimPrefix(h, "Bearer ")
		} else {
			token = GetAccessCookie(r)
		}
		if token == "" {
			apierr.Write(w, http.StatusUnauthorized, apierr.CodeUnauthorized, "missing access token")
			return
		}
		sub, err := ParseAccess(token)
		if err != nil {
			apierr.Write(w, http.StatusUnauthorized, apierr.CodeUnauthorized, "invalid token")
			return
		}
		ctx := context.WithValue(r.Context(), ctxSubjectKey, sub)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireRole — 특정 역할(들) 중 하나여야 통과.
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sub, ok := FromContext(r.Context())
			if !ok {
				apierr.Write(w, http.StatusUnauthorized, apierr.CodeUnauthorized, "not authenticated")
				return
			}
			if _, ok := allowed[sub.Role]; !ok {
				apierr.Write(w, http.StatusForbidden, apierr.CodeForbidden, "insufficient role")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
