package httpx

import (
	"net/http"
	"os"
	"strings"
)

// CORS — dev에선 ALLOWED_ORIGINS env로 origin 화이트리스트.
// production에선 same-origin만 사용하므로 통과.
//
// 사용:
//
//	r.Use(httpx.CORS())
//
// env 예:
//
//	ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001
func CORS() func(http.Handler) http.Handler {
	allowed := parseOrigins(os.Getenv("ALLOWED_ORIGINS"))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && isAllowed(origin, allowed) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Internal-Token")
				w.Header().Set("Access-Control-Max-Age", "300")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseOrigins(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func isAllowed(origin string, allowed []string) bool {
	for _, a := range allowed {
		if a == origin {
			return true
		}
	}
	return false
}
