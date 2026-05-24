// JWT 발급/검증 — Access(15m).
// Design Ref: §7 Security
package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type jwtClaims struct {
	jwt.RegisteredClaims
	NurseID int    `json:"nid"`
	Email   string `json:"em"`
	Role    string `json:"rl"`
}

func accessTTL() time.Duration {
	if v := os.Getenv("JWT_ACCESS_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 15 * time.Minute
}

func refreshTTL() time.Duration {
	if v := os.Getenv("JWT_REFRESH_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return 7 * 24 * time.Hour
}

func jwtSecret() ([]byte, error) {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		return nil, errors.New("JWT_SECRET not set")
	}
	return []byte(s), nil
}

// IssueAccess — JWT(HS256) 발급.
func IssueAccess(sub Subject) (string, time.Time, error) {
	secret, err := jwtSecret()
	if err != nil {
		return "", time.Time{}, err
	}
	exp := time.Now().Add(accessTTL())
	claims := jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "ward-duty-api",
			Subject:   fmt.Sprintf("%d", sub.NurseID),
		},
		NurseID: sub.NurseID,
		Email:   sub.Email,
		Role:    sub.Role,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// ParseAccess — 토큰 검증 후 Subject 추출.
func ParseAccess(tokenStr string) (*Subject, error) {
	secret, err := jwtSecret()
	if err != nil {
		return nil, err
	}
	t, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected alg: %v", t.Method.Alg())
		}
		return secret, nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := t.Claims.(*jwtClaims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return &Subject{
		NurseID: claims.NurseID,
		Email:   claims.Email,
		Role:    claims.Role,
	}, nil
}
