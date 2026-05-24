// K-02 / K-04 / K-05 사전 검증.
//
// K-02: 자신 포함 3달 연속 지정 금지.
// K-04: (M, M+1) 연속 후 cooldown 3달 → M+2, M+3, M+4 동일인 지정 불가.
// K-05: fixed_shift_pattern != NULL인 nurse를 night_keeper로 지정 불가.
package nightkeepers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

// ValidationError — 룰 ID와 메시지.
type ValidationError struct {
	RuleID  string
	Message string
}

func (e *ValidationError) Error() string { return e.RuleID + ": " + e.Message }

// Validate — POST 직전 호출. nurseID + targetYM (YYYY-MM).
func Validate(ctx context.Context, r *Repo, nurseID int, targetYM string) error {
	// K-05: fixed_shift_pattern 충돌
	pat, err := r.FetchFixedPattern(ctx, nurseID)
	if err != nil {
		return err
	}
	if pat != nil && *pat != "" {
		return &ValidationError{
			RuleID:  "K-05",
			Message: fmt.Sprintf("이 간호사는 고정 시프트 패턴(%s)이 설정되어 있어 나이트킵 지정 불가", *pat),
		}
	}

	prev1 := addMonths(targetYM, -1)
	prev2 := addMonths(targetYM, -2)
	prev3 := addMonths(targetYM, -3)
	prev4 := addMonths(targetYM, -4)
	next1 := addMonths(targetYM, 1)
	next2 := addMonths(targetYM, 2)

	has := func(ym string) (bool, error) { return r.HasMonth(ctx, nurseID, ym) }

	// K-02: 자기 자신 포함 3달 연속
	for _, pair := range [][2]string{
		{prev2, prev1}, {prev1, next1}, {next1, next2},
	} {
		a, err := has(pair[0])
		if err != nil {
			return err
		}
		b, err := has(pair[1])
		if err != nil {
			return err
		}
		if a && b {
			return &ValidationError{
				RuleID:  "K-02",
				Message: fmt.Sprintf("3달 연속 지정 금지 (%s, %s, %s 중 둘이 이미 존재)", pair[0], pair[1], targetYM),
			}
		}
	}

	// K-04: 직전 (X-1, X-2) 또는 (X-3, X-4) 연속 후 cooldown 3달 → target=X에선 차단
	for _, pair := range [][2]string{
		{prev2, prev3}, {prev3, prev4},
	} {
		a, err := has(pair[0])
		if err != nil {
			return err
		}
		b, err := has(pair[1])
		if err != nil {
			return err
		}
		if a && b {
			return &ValidationError{
				RuleID:  "K-04",
				Message: fmt.Sprintf("쿨다운 3달 필요 (%s/%s 연속 종료 이후 %s까지 비워야 함)", pair[1], pair[0], addMonths(pair[0], 3)),
			}
		}
	}
	return nil
}

func addMonths(ym string, delta int) string {
	if len(ym) != 7 || ym[4] != '-' {
		return ym
	}
	y, _ := strconv.Atoi(ym[:4])
	m, _ := strconv.Atoi(ym[5:])
	total := y*12 + (m - 1) + delta
	if total < 0 {
		return ym
	}
	ny := total / 12
	nm := total%12 + 1
	return fmt.Sprintf("%04d-%02d", ny, nm)
}

// IsValidYM — 형식 검사 보조.
func IsValidYM(ym string) error {
	if len(ym) != 7 || ym[4] != '-' {
		return errors.New("year_month must be YYYY-MM")
	}
	y, err1 := strconv.Atoi(ym[:4])
	m, err2 := strconv.Atoi(ym[5:])
	if err1 != nil || err2 != nil || m < 1 || m > 12 || y < 2000 || y > 2100 {
		return errors.New("invalid year_month")
	}
	return nil
}
