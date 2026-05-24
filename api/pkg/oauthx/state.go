// Package oauthx — OAuth state·nonce·PKCE 헬퍼.
// HMAC 서명된 state로 CSRF 차단, Redis 없이도 stateless 가능.
//
// Design Ref: §7 Security (state·nonce·PKCE)
package oauthx

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const stateTTL = 10 * time.Minute

// State payload — 콜백에서 검증할 정보.
type State struct {
	Nonce     string `json:"n"`
	IssuedAt  int64  `json:"i"`
	ExpiresAt int64  `json:"e"`
}

// PKCEPair — verifier + S256 challenge.
type PKCEPair struct {
	Verifier  string
	Challenge string
}

// NewPKCE — 32바이트 verifier + SHA256 challenge.
func NewPKCE() (PKCEPair, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return PKCEPair{}, err
	}
	verifier := base64URLEncode(buf)
	h := sha256.Sum256([]byte(verifier))
	return PKCEPair{Verifier: verifier, Challenge: base64URLEncode(h[:])}, nil
}

// NewState — nonce 생성 + HMAC 서명.
func NewState(secret []byte) (token string, nonce string, err error) {
	nBuf := make([]byte, 16)
	if _, err := rand.Read(nBuf); err != nil {
		return "", "", err
	}
	nonce = base64URLEncode(nBuf)
	now := time.Now()
	st := State{
		Nonce:     nonce,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(stateTTL).Unix(),
	}
	payload, _ := json.Marshal(st)
	body := base64URLEncode(payload)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	sig := base64URLEncode(mac.Sum(nil))
	return body + "." + sig, nonce, nil
}

// VerifyState — 서명 + TTL 검증.
func VerifyState(token string, secret []byte) (*State, error) {
	parts := splitDot(token)
	if len(parts) != 2 {
		return nil, errors.New("invalid state format")
	}
	body, sig := parts[0], parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	want := base64URLEncode(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return nil, errors.New("state signature mismatch")
	}
	raw, err := base64URLDecode(body)
	if err != nil {
		return nil, fmt.Errorf("state body decode: %w", err)
	}
	var st State
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	if time.Now().Unix() > st.ExpiresAt {
		return nil, errors.New("state expired")
	}
	return &st, nil
}

func splitDot(s string) []string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

func base64URLEncode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}
func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
