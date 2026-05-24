package settings

import (
	"context"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repo struct{ PG *pgxpool.Pool }

func NewRepo(pg *pgxpool.Pool) *Repo { return &Repo{PG: pg} }

const allCols = `min_d, min_e, min_n,
		max_consecutive_n, min_rest_after_n, max_consecutive_workdays,
		balance_off_tolerance, previous_month_lookback_days,
		night_keeper_max_consecutive_months, night_keeper_cooldown_months,
		wish_unavailable_quota_monthly, wish_preference_quota_monthly,
		wish_deadline_days_before_month, swap_deadline_days_before_date,
		weight_balance_off, weight_respect_wishes, weight_weekend_balance,
		weight_same_shift_streak, weight_short_rest_pattern, updated_at`

func (r *Repo) Get(ctx context.Context) (*WardSettings, error) {
	row := r.PG.QueryRow(ctx, "SELECT "+allCols+" FROM ward_settings WHERE id = 1")
	var s WardSettings
	if err := row.Scan(
		&s.MinD, &s.MinE, &s.MinN,
		&s.MaxConsecutiveN, &s.MinRestAfterN, &s.MaxConsecutiveWorkdays,
		&s.BalanceOffTolerance, &s.PreviousMonthLookbackDays,
		&s.NightKeeperMaxConsecutiveMonths, &s.NightKeeperCooldownMonths,
		&s.WishUnavailableQuotaMonthly, &s.WishPreferenceQuotaMonthly,
		&s.WishDeadlineDaysBeforeMonth, &s.SwapDeadlineDaysBeforeDate,
		&s.WeightBalanceOff, &s.WeightRespectWishes, &s.WeightWeekendBalance,
		&s.WeightSameShiftStreak, &s.WeightShortRestPattern, &s.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &s, nil
}

func (r *Repo) Update(ctx context.Context, in UpdateInput) (*WardSettings, error) {
	sets := []string{}
	args := []any{}
	add := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, col+" = $"+strconv.Itoa(len(args)))
	}
	if in.MinD != nil { add("min_d", *in.MinD) }
	if in.MinE != nil { add("min_e", *in.MinE) }
	if in.MinN != nil { add("min_n", *in.MinN) }
	if in.MaxConsecutiveN != nil { add("max_consecutive_n", *in.MaxConsecutiveN) }
	if in.MinRestAfterN != nil { add("min_rest_after_n", *in.MinRestAfterN) }
	if in.MaxConsecutiveWorkdays != nil { add("max_consecutive_workdays", *in.MaxConsecutiveWorkdays) }
	if in.BalanceOffTolerance != nil { add("balance_off_tolerance", *in.BalanceOffTolerance) }
	if in.PreviousMonthLookbackDays != nil { add("previous_month_lookback_days", *in.PreviousMonthLookbackDays) }
	if in.NightKeeperMaxConsecutiveMonths != nil { add("night_keeper_max_consecutive_months", *in.NightKeeperMaxConsecutiveMonths) }
	if in.NightKeeperCooldownMonths != nil { add("night_keeper_cooldown_months", *in.NightKeeperCooldownMonths) }
	if in.WishUnavailableQuotaMonthly != nil { add("wish_unavailable_quota_monthly", *in.WishUnavailableQuotaMonthly) }
	if in.WishPreferenceQuotaMonthly != nil { add("wish_preference_quota_monthly", *in.WishPreferenceQuotaMonthly) }
	if in.WishDeadlineDaysBeforeMonth != nil { add("wish_deadline_days_before_month", *in.WishDeadlineDaysBeforeMonth) }
	if in.SwapDeadlineDaysBeforeDate != nil { add("swap_deadline_days_before_date", *in.SwapDeadlineDaysBeforeDate) }
	if in.WeightBalanceOff != nil { add("weight_balance_off", *in.WeightBalanceOff) }
	if in.WeightRespectWishes != nil { add("weight_respect_wishes", *in.WeightRespectWishes) }
	if in.WeightWeekendBalance != nil { add("weight_weekend_balance", *in.WeightWeekendBalance) }
	if in.WeightSameShiftStreak != nil { add("weight_same_shift_streak", *in.WeightSameShiftStreak) }
	if in.WeightShortRestPattern != nil { add("weight_short_rest_pattern", *in.WeightShortRestPattern) }

	if len(sets) > 0 {
		q := "UPDATE ward_settings SET " + strings.Join(sets, ", ") + ", updated_at = NOW() WHERE id = 1"
		if _, err := r.PG.Exec(ctx, q, args...); err != nil {
			return nil, err
		}
	}
	return r.Get(ctx)
}
