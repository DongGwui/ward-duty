// Redis refresh 토큰 저장/회수 — atomic consume + O(1) lookup.
//
// C3 fix (Checkpoint 1 Critical):
//   - GETDEL로 atomic consume → 회전 race condition 제거
//   - refresh:lookup:<sha256> -> nurse_id 보조 키 → SCAN 폐기
//   - refresh:byUser:<nid> -> SET(hashes) → 일괄 회수 O(N) 직접 조회
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type refreshRecord struct {
	NurseID   int       `json:"nid"`
	Email     string    `json:"em"`
	Role      string    `json:"rl"`
	IssuedAt  time.Time `json:"iat"`
	ExpiresAt time.Time `json:"exp"`
}

func newRefreshToken() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(buf)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	return raw, hash, nil
}

// HashRefresh — 외부에서 raw → hash 변환 (디버깅·테스트용).
func HashRefresh(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func refreshKey(hash string) string      { return "refresh:" + hash }
func refreshLookupKey(hash string) string { return "refresh:lookup:" + hash }
func refreshByUserKey(nid int) string     { return fmt.Sprintf("refresh:byUser:%d", nid) }

// IssueRefresh — refresh 발급 + Redis 저장 (3개 키 동시 갱신).
func IssueRefresh(ctx context.Context, rc *redis.Client, sub Subject) (raw string, exp time.Time, err error) {
	raw, hash, err := newRefreshToken()
	if err != nil {
		return "", time.Time{}, err
	}
	ttl := refreshTTL()
	exp = time.Now().Add(ttl)
	rec := refreshRecord{
		NurseID:   sub.NurseID,
		Email:     sub.Email,
		Role:      sub.Role,
		IssuedAt:  time.Now(),
		ExpiresAt: exp,
	}
	payload, _ := json.Marshal(rec)

	pipe := rc.TxPipeline()
	pipe.Set(ctx, refreshKey(hash), payload, ttl)
	pipe.Set(ctx, refreshLookupKey(hash), sub.NurseID, ttl)
	pipe.SAdd(ctx, refreshByUserKey(sub.NurseID), hash)
	pipe.Expire(ctx, refreshByUserKey(sub.NurseID), ttl)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", time.Time{}, err
	}
	return raw, exp, nil
}

// ConsumeRefresh — atomic GETDEL → 1회용 회전 race 제거 (C3).
//
// hint(nurseID)는 무시됨 — lookup 키로 직접 nid 확인.
// 동시 두 호출 중 한쪽만 성공, 다른 쪽은 ErrRefreshConsumed.
func ConsumeRefresh(ctx context.Context, rc *redis.Client, raw string, _hint int) (*Subject, error) {
	hash := HashRefresh(raw)

	// atomic consume — 한쪽만 토큰 값 받음
	val, err := rc.GetDel(ctx, refreshKey(hash)).Result()
	if errors.Is(err, redis.Nil) {
		return nil, ErrRefreshConsumed
	}
	if err != nil {
		return nil, err
	}
	var rec refreshRecord
	if err := json.Unmarshal([]byte(val), &rec); err != nil {
		return nil, err
	}
	if time.Now().After(rec.ExpiresAt) {
		// lookup도 정리
		_ = rc.Del(ctx, refreshLookupKey(hash)).Err()
		return nil, ErrRefreshExpired
	}
	// 보조 키 정리
	pipe := rc.TxPipeline()
	pipe.Del(ctx, refreshLookupKey(hash))
	pipe.SRem(ctx, refreshByUserKey(rec.NurseID), hash)
	_, _ = pipe.Exec(ctx)

	return &Subject{NurseID: rec.NurseID, Email: rec.Email, Role: rec.Role}, nil
}

// RevokeAllRefresh — 로그아웃 시 사용자의 모든 refresh 일괄 회수 (O(N) 직접).
func RevokeAllRefresh(ctx context.Context, rc *redis.Client, nurseID int) error {
	hashes, err := rc.SMembers(ctx, refreshByUserKey(nurseID)).Result()
	if err != nil {
		return err
	}
	if len(hashes) == 0 {
		return nil
	}
	pipe := rc.TxPipeline()
	for _, h := range hashes {
		pipe.Del(ctx, refreshKey(h))
		pipe.Del(ctx, refreshLookupKey(h))
	}
	pipe.Del(ctx, refreshByUserKey(nurseID))
	_, err = pipe.Exec(ctx)
	return err
}

// LookupNurseIDByRefresh — refresh 쿠키로 직접 nid 조회 (O(1)).
//
// SCAN 기반 FindNurseIDByRefresh를 대체 (C3). consume 전이라 lookup 키만 본다.
func LookupNurseIDByRefresh(ctx context.Context, rc *redis.Client, raw string) (int, error) {
	hash := HashRefresh(raw)
	v, err := rc.Get(ctx, refreshLookupKey(hash)).Int()
	if errors.Is(err, redis.Nil) {
		return 0, ErrRefreshNotFound
	}
	if err != nil {
		return 0, err
	}
	return v, nil
}

// ----- errors -----

var (
	ErrRefreshNotFound = errors.New("refresh not found")
	ErrRefreshConsumed = errors.New("refresh already consumed")
	ErrRefreshExpired  = errors.New("refresh expired")
)
