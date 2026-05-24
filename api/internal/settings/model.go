package settings

import "time"

// WardSettings — Design §3.1 single-row, 18 fields.
type WardSettings struct {
	MinD                              int       `json:"min_d"`
	MinE                              int       `json:"min_e"`
	MinN                              int       `json:"min_n"`
	MaxConsecutiveN                   int       `json:"max_consecutive_n"`
	MinRestAfterN                     int       `json:"min_rest_after_n"`
	MaxConsecutiveWorkdays            int       `json:"max_consecutive_workdays"`
	BalanceOffTolerance               int       `json:"balance_off_tolerance"`
	PreviousMonthLookbackDays         int       `json:"previous_month_lookback_days"`
	NightKeeperMaxConsecutiveMonths   int       `json:"night_keeper_max_consecutive_months"`
	NightKeeperCooldownMonths         int       `json:"night_keeper_cooldown_months"`
	WishUnavailableQuotaMonthly       *int      `json:"wish_unavailable_quota_monthly,omitempty"`
	WishPreferenceQuotaMonthly        *int      `json:"wish_preference_quota_monthly,omitempty"`
	WishDeadlineDaysBeforeMonth       int       `json:"wish_deadline_days_before_month"`
	SwapDeadlineDaysBeforeDate        int       `json:"swap_deadline_days_before_date"`
	WeightBalanceOff                  int       `json:"weight_balance_off"`
	WeightRespectWishes               int       `json:"weight_respect_wishes"`
	WeightWeekendBalance              int       `json:"weight_weekend_balance"`
	WeightSameShiftStreak             int       `json:"weight_same_shift_streak"`
	WeightShortRestPattern            int       `json:"weight_short_rest_pattern"`
	UpdatedAt                         time.Time `json:"updated_at"`
}

// UpdateInput — 모든 필드 pointer (부분 업데이트).
type UpdateInput struct {
	MinD                              *int `json:"min_d,omitempty"`
	MinE                              *int `json:"min_e,omitempty"`
	MinN                              *int `json:"min_n,omitempty"`
	MaxConsecutiveN                   *int `json:"max_consecutive_n,omitempty"`
	MinRestAfterN                     *int `json:"min_rest_after_n,omitempty"`
	MaxConsecutiveWorkdays            *int `json:"max_consecutive_workdays,omitempty"`
	BalanceOffTolerance               *int `json:"balance_off_tolerance,omitempty"`
	PreviousMonthLookbackDays         *int `json:"previous_month_lookback_days,omitempty"`
	NightKeeperMaxConsecutiveMonths   *int `json:"night_keeper_max_consecutive_months,omitempty"`
	NightKeeperCooldownMonths         *int `json:"night_keeper_cooldown_months,omitempty"`
	WishUnavailableQuotaMonthly       *int `json:"wish_unavailable_quota_monthly,omitempty"`
	WishPreferenceQuotaMonthly        *int `json:"wish_preference_quota_monthly,omitempty"`
	WishDeadlineDaysBeforeMonth       *int `json:"wish_deadline_days_before_month,omitempty"`
	SwapDeadlineDaysBeforeDate        *int `json:"swap_deadline_days_before_date,omitempty"`
	WeightBalanceOff                  *int `json:"weight_balance_off,omitempty"`
	WeightRespectWishes               *int `json:"weight_respect_wishes,omitempty"`
	WeightWeekendBalance              *int `json:"weight_weekend_balance,omitempty"`
	WeightSameShiftStreak             *int `json:"weight_same_shift_streak,omitempty"`
	WeightShortRestPattern            *int `json:"weight_short_rest_pattern,omitempty"`
}
