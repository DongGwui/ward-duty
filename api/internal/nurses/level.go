// G-04 — 등급 자동 분류 (override 우선).
//
// Design Ref: §3.3 G-04
package nurses

import (
	"time"

	"ward-duty-api/internal/levels"
)

// ClassifyLevel — nurse의 등급 코드를 결정.
//
//   1) experience_level_override가 비어있지 않으면 그대로
//   2) hire_date가 NULL이면 최저 sort_order 등급
//   3) hire_date 기반 자동 (sort_order 순회)
func ClassifyLevel(n *Nurse, ls []levels.Level, today time.Time) string {
	if n.ExperienceLevelOverride != nil && *n.ExperienceLevelOverride != "" {
		return *n.ExperienceLevelOverride
	}
	if len(ls) == 0 {
		return ""
	}
	if n.HireDate == nil {
		return ls[0].Code
	}
	m := monthsBetween(*n.HireDate, today)
	for _, l := range ls {
		// max NULL = 무제한
		if m >= l.MinMonths && (l.MaxMonths == nil || m < *l.MaxMonths) {
			return l.Code
		}
	}
	return ls[len(ls)-1].Code
}

func monthsBetween(start, end time.Time) int {
	if end.Before(start) {
		return 0
	}
	return (end.Year()-start.Year())*12 + (int(end.Month()) - int(start.Month()))
}
