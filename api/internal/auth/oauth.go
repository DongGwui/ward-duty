// Google OAuth — start/callback 흐름.
//
// Design Ref: §4.1 auth 엔드포인트, §7 Security
//
// 흐름:
//   1. GET /api/auth/oauth/google/start
//      - HMAC 서명된 state + nonce + PKCE 발급
//      - state·verifier·nonce를 httpOnly 쿠키 3개에 임시 보관 (10분)
//      - Google 인가 URL로 302
//   2. Google 콜백 → GET /api/auth/oauth/google/callback?code=&state=
//      - state 쿠키 일치 검증 + HMAC 검증
//      - code → token exchange (PKCE verifier 쿠키 사용)
//      - id_token → email/sub 추출
//      - email로 nurses 화이트리스트 조회 + google_sub upsert
//      - JWT(access) + refresh 쿠키 발급 → /로 리다이렉트
package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"

	"ward-duty-api/pkg/oauthx"
)

const (
	cookieState    = "wd_state"
	cookieVerifier = "wd_pkce"
	cookieRefresh  = "wd_refresh"
	cookieAccess   = "wd_access" // C2 fix: access도 httpOnly 쿠키로
)

type OAuthConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
	StateSecret  []byte
	AdminEmail   string
}

func LoadOAuthConfig() (*OAuthConfig, error) {
	cid := os.Getenv("GOOGLE_CLIENT_ID")
	csec := os.Getenv("GOOGLE_CLIENT_SECRET")
	redir := os.Getenv("OAUTH_REDIRECT_URL")
	secret := os.Getenv("OAUTH_STATE_SECRET")
	admin := os.Getenv("ADMIN_EMAIL")
	if cid == "" || csec == "" || redir == "" || secret == "" {
		return nil, errors.New("missing GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET / OAUTH_REDIRECT_URL / OAUTH_STATE_SECRET")
	}
	return &OAuthConfig{
		ClientID:     cid,
		ClientSecret: csec,
		RedirectURL:  redir,
		StateSecret:  []byte(secret),
		AdminEmail:   strings.ToLower(strings.TrimSpace(admin)),
	}, nil
}

func (c *OAuthConfig) endpoint() *oauth2.Config {
	return &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		RedirectURL:  c.RedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

// StartURL — 인가 URL + 쿠키 3개 세팅.
func (c *OAuthConfig) StartURL(w http.ResponseWriter) (string, error) {
	pkce, err := oauthx.NewPKCE()
	if err != nil {
		return "", err
	}
	stateTok, nonce, err := oauthx.NewState(c.StateSecret)
	if err != nil {
		return "", err
	}
	setShortCookie(w, cookieState, stateTok)
	setShortCookie(w, cookieVerifier, pkce.Verifier)

	url := c.endpoint().AuthCodeURL(stateTok,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("code_challenge", pkce.Challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("nonce", nonce),
		oauth2.SetAuthURLParam("prompt", "select_account"),
	)
	return url, nil
}

// HandleCallback — code 교환 + id_token 서명/iss/aud/exp/nonce 검증 + email 반환.
//
// C1·C5 (Checkpoint 1 Critical): google idtoken 라이브러리로 RS256 서명 + iss + aud + exp
// 자동 검증. nonce는 state 토큰에서 추출해 claims와 비교.
func (c *OAuthConfig) HandleCallback(ctx context.Context, r *http.Request) (*GoogleUserInfo, error) {
	q := r.URL.Query()
	if errParam := q.Get("error"); errParam != "" {
		return nil, fmt.Errorf("google error: %s", errParam)
	}
	code := q.Get("code")
	if code == "" {
		return nil, errors.New("missing code")
	}
	state := q.Get("state")
	stateCookie, err := r.Cookie(cookieState)
	if err != nil || stateCookie.Value != state {
		return nil, errors.New("state cookie mismatch")
	}
	st, err := oauthx.VerifyState(state, c.StateSecret)
	if err != nil {
		return nil, fmt.Errorf("state verify: %w", err)
	}
	verifierCookie, err := r.Cookie(cookieVerifier)
	if err != nil {
		return nil, errors.New("missing pkce cookie")
	}

	tok, err := c.endpoint().Exchange(ctx, code,
		oauth2.SetAuthURLParam("code_verifier", verifierCookie.Value),
	)
	if err != nil {
		return nil, fmt.Errorf("token exchange: %w", err)
	}

	idTokenStr, _ := tok.Extra("id_token").(string)
	if idTokenStr == "" {
		return nil, errors.New("missing id_token from google")
	}

	// 서명·iss·aud·exp 검증 (idtoken.Validate가 자동)
	payload, err := idtoken.Validate(ctx, idTokenStr, c.ClientID)
	if err != nil {
		return nil, fmt.Errorf("id_token validate: %w", err)
	}

	// nonce 검증 (state의 nonce와 claims.nonce 일치)
	claimNonce, _ := payload.Claims["nonce"].(string)
	if claimNonce == "" || claimNonce != st.Nonce {
		return nil, errors.New("nonce mismatch")
	}

	emailVerified, _ := payload.Claims["email_verified"].(bool)
	if !emailVerified {
		return nil, errors.New("email not verified")
	}
	email, _ := payload.Claims["email"].(string)
	if email == "" {
		return nil, errors.New("missing email claim")
	}
	name, _ := payload.Claims["name"].(string)
	picture, _ := payload.Claims["picture"].(string)

	return &GoogleUserInfo{
		Sub:           payload.Subject,
		Email:         email,
		EmailVerified: emailVerified,
		Name:          name,
		Picture:       picture,
	}, nil
}

// ClearOAuthCookies — 콜백 처리 후 임시 쿠키 정리.
func ClearOAuthCookies(w http.ResponseWriter) {
	clearCookie(w, cookieState)
	clearCookie(w, cookieVerifier)
}

// SetRefreshCookie — refresh 토큰을 httpOnly 쿠키로.
//
// C3 fix: SameSite=Strict (refresh는 same-site에서만 의미).
func SetRefreshCookie(w http.ResponseWriter, raw string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieRefresh,
		Value:    raw,
		Path:     "/api/auth",
		Expires:  expires,
		HttpOnly: true,
		Secure:   isSecure(),
		SameSite: http.SameSiteStrictMode,
	})
}

// GetRefreshCookie — refresh 토큰 읽기.
func GetRefreshCookie(r *http.Request) string {
	c, err := r.Cookie(cookieRefresh)
	if err != nil {
		return ""
	}
	return c.Value
}

// ClearRefreshCookie — 로그아웃.
func ClearRefreshCookie(w http.ResponseWriter) {
	clearCookie(w, cookieRefresh)
}

// SetAccessCookie — access JWT를 httpOnly 쿠키로 (C2 fix). API 전체 path.
func SetAccessCookie(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieAccess,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		HttpOnly: true,
		Secure:   isSecure(),
		SameSite: http.SameSiteLaxMode, // /api/auth/callback 후 첫 navigation에서 동작해야 하므로 Lax
	})
}

// GetAccessCookie — middleware에서 Authorization 헤더 없을 때 fallback.
func GetAccessCookie(r *http.Request) string {
	c, err := r.Cookie(cookieAccess)
	if err != nil {
		return ""
	}
	return c.Value
}

// ClearAccessCookie — 로그아웃.
func ClearAccessCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieAccess,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

// ----- internal -----

func setShortCookie(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/api/auth",
		MaxAge:   600, // 10분
		HttpOnly: true,
		Secure:   isSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func clearCookie(w http.ResponseWriter, name string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/api/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func isSecure() bool {
	// 운영(Cloudflare Tunnel)은 항상 HTTPS. 로컬 dev에선 false로 둘 수 있음.
	return strings.HasPrefix(os.Getenv("OAUTH_REDIRECT_URL"), "https://")
}

// parseIDToken / fetchUserInfo는 idtoken.Validate 도입으로 제거됨 (C1 fix).
