// Patch 시 변경 필드 감지 → 본인에게 알림 발사.
// Phase B — level_changed / fixed_pattern_changed.
package nurses

import (
	"context"
	"log/slog"

	"ward-duty-api/internal/notifications"
)

// notifyChanges — Update 전후의 Nurse를 비교해 의미 있는 변경에 대해 알림 발송.
// 본인이 inactive이거나 Notif 미주입이면 no-op.
func notifyChanges(ctx context.Context, notif *notifications.Repo, before, after *Nurse) {
	if notif == nil || before == nil || after == nil || !after.Active {
		return
	}

	if !ptrEq(before.ExperienceLevelOverride, after.ExperienceLevelOverride) {
		old := ptrStr(before.ExperienceLevelOverride, "(자동)")
		neu := ptrStr(after.ExperienceLevelOverride, "(자동)")
		err := notif.Insert(ctx, notifications.Create{
			RecipientNurseID: after.ID,
			Type:             notifications.TypeLevelChanged,
			Title:            "등급이 변경되었습니다",
			Body:             old + " → " + neu,
			Link:             "/nurses",
			Meta: map[string]any{
				"old": ptrStr(before.ExperienceLevelOverride, ""),
				"new": ptrStr(after.ExperienceLevelOverride, ""),
			},
		})
		if err != nil {
			slog.Warn("notify level_changed", "err", err, "nurse_id", after.ID)
		}
	}

	if !ptrEq(before.FixedShiftPattern, after.FixedShiftPattern) {
		old := ptrStr(before.FixedShiftPattern, "일반 로테이션")
		neu := ptrStr(after.FixedShiftPattern, "일반 로테이션")
		err := notif.Insert(ctx, notifications.Create{
			RecipientNurseID: after.ID,
			Type:             notifications.TypeFixedPatternChanged,
			Title:            "고정 근무 패턴이 변경되었습니다",
			Body:             old + " → " + neu,
			Link:             "/nurses",
			Meta: map[string]any{
				"old": ptrStr(before.FixedShiftPattern, ""),
				"new": ptrStr(after.FixedShiftPattern, ""),
			},
		})
		if err != nil {
			slog.Warn("notify fixed_pattern_changed", "err", err, "nurse_id", after.ID)
		}
	}
}

func ptrEq(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ptrStr(p *string, fallback string) string {
	if p == nil || *p == "" {
		return fallback
	}
	return *p
}
