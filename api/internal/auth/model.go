package auth

import "time"

// Subject — JWT 안에 들어가는 우리 시스템 사용자 정보.
type Subject struct {
	NurseID int    `json:"nid"`
	Email   string `json:"em"`
	Role    string `json:"rl"` // head_nurse | nurse
}

// Tokens — 발급 결과.
type Tokens struct {
	AccessToken      string    `json:"access_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshToken     string    `json:"-"` // 쿠키로만 전달
	RefreshExpiresAt time.Time `json:"-"`
}

// GoogleUserInfo — id_token에서 추출.
type GoogleUserInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}
