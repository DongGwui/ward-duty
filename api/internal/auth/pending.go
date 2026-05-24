// Stage 2 — OAuth callback이 nurses에 매칭되지 못한 사용자를 임시 보관.
// 매니저가 /nurses 페이지에서 명단의 어느 행에 연결할지 선택.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	pendingTTL   = 7 * 24 * time.Hour // 7일 후 자동 만료
	pendingIndex = "pending:index"     // SET 멤버: 이메일 목록
)

// PendingAccount — Redis에 보관되는 미매칭 OAuth 사용자 정보.
type PendingAccount struct {
	Email     string    `json:"email"`
	GoogleSub string    `json:"google_sub"`
	Name      string    `json:"name,omitempty"`
	Picture   string    `json:"picture,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func pendingKey(email string) string {
	return "pending:" + strings.ToLower(strings.TrimSpace(email))
}

// SavePending — OAuth callback에서 매칭 실패 시 호출.
func SavePending(ctx context.Context, rc *redis.Client, p *PendingAccount) error {
	p.Email = strings.ToLower(strings.TrimSpace(p.Email))
	if p.Email == "" {
		return errors.New("pending: email required")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	payload, _ := json.Marshal(p)
	pipe := rc.TxPipeline()
	pipe.Set(ctx, pendingKey(p.Email), payload, pendingTTL)
	pipe.SAdd(ctx, pendingIndex, p.Email)
	pipe.Expire(ctx, pendingIndex, pendingTTL+24*time.Hour)
	_, err := pipe.Exec(ctx)
	return err
}

// ListPending — 매니저용 — 현재 매칭 대기 사용자 전체.
//
// 만료된 키는 자동으로 제외 (Get이 nil이면 index에서도 정리).
func ListPending(ctx context.Context, rc *redis.Client) ([]PendingAccount, error) {
	emails, err := rc.SMembers(ctx, pendingIndex).Result()
	if err != nil {
		return nil, err
	}
	out := make([]PendingAccount, 0, len(emails))
	for _, e := range emails {
		v, err := rc.Get(ctx, pendingKey(e)).Result()
		if errors.Is(err, redis.Nil) {
			// stale → index에서 제거
			_ = rc.SRem(ctx, pendingIndex, e).Err()
			continue
		}
		if err != nil {
			return nil, err
		}
		var p PendingAccount
		if err := json.Unmarshal([]byte(v), &p); err != nil {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// GetPending — 매칭 확정 시점에 사용.
func GetPending(ctx context.Context, rc *redis.Client, email string) (*PendingAccount, error) {
	v, err := rc.Get(ctx, pendingKey(email)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrPendingNotFound
	}
	if err != nil {
		return nil, err
	}
	var p PendingAccount
	if err := json.Unmarshal([]byte(v), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// DeletePending — 매칭 확정/거부 시 제거.
func DeletePending(ctx context.Context, rc *redis.Client, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	pipe := rc.TxPipeline()
	pipe.Del(ctx, pendingKey(email))
	pipe.SRem(ctx, pendingIndex, email)
	_, err := pipe.Exec(ctx)
	return err
}

var ErrPendingNotFound = errors.New("pending account not found")

// helper for handler — front URL에 ?status=pending&email=... 식 전달 시 사용
func PendingRedirectQuery(email string) string {
	return fmt.Sprintf("pending=1&email=%s", strings.ToLower(strings.TrimSpace(email)))
}
