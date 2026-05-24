// G-04 (v0.5) — head_nurse가 각 간호사에게 직접 등급을 부여.
//
// 이전 v0.4: hire_date 기반 자동 분류 + override.
// 변경 사유: 자동 분류는 운영 유연성이 떨어지고 임상 현실(직책·역량 등급)을 반영 못함.
// 현재 v0.5:
//   - nurses.experience_level_override 가 유일한 등급 결정자
//   - hire_date는 참고 정보 (자동 분류 미사용)
//   - experience_levels.min_months/max_months는 deprecated (스키마는 호환 유지)
//   - 미지정 nurse는 sort_order가 가장 낮은 등급으로 fallback (솔버 input 안전성)
package nurses

import (
	"time"

	"ward-duty-api/internal/levels"
)

// ClassifyLevel — override 우선, 없으면 가장 낮은 등급.
//
// today 인자는 시그니처 호환을 위해 유지하지만 더 이상 사용하지 않음.
func ClassifyLevel(n *Nurse, ls []levels.Level, _ time.Time) string {
	if n.ExperienceLevelOverride != nil && *n.ExperienceLevelOverride != "" {
		return *n.ExperienceLevelOverride
	}
	if len(ls) == 0 {
		return ""
	}
	// 미지정 → sort_order가 가장 낮은 등급 (loadLevels가 sort_order 오름차순으로 정렬)
	return ls[0].Code
}
