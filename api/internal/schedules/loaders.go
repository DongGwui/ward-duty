// Service에서 호출하는 컨텍스트 로더 + 솔버 입력 변환 함수.
package schedules

import (
	"context"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ward-duty-api/internal/levels"
	"ward-duty-api/internal/nurses"
	"ward-duty-api/internal/solver"
)

// ----- settings -----

type settingsBundle struct {
	MinD                            int
	MinE                            int
	MinN                            int
	MaxConsecutiveN                 int
	MinRestAfterN                   int
	MaxConsecutiveWorkdays          int
	BalanceOffTolerance             int
	PreviousMonthLookbackDays       int
	WeightBalanceOff                int
	WeightRespectWishes             int
	WeightWeekendBalance            int
	WeightSameShiftStreak           int
	WeightShortRestPattern          int
}

func (s *settingsBundle) toSolver() solver.WardSettingsIn {
	return solver.WardSettingsIn{
		MinD:                      s.MinD,
		MinE:                      s.MinE,
		MinN:                      s.MinN,
		MaxConsecutiveN:           s.MaxConsecutiveN,
		MinRestAfterN:             s.MinRestAfterN,
		MaxConsecutiveWorkdays:    s.MaxConsecutiveWorkdays,
		BalanceOffTolerance:       s.BalanceOffTolerance,
		PreviousMonthLookbackDays: s.PreviousMonthLookbackDays, // H11 fix
		WeightBalanceOff:          s.WeightBalanceOff,
		WeightRespectWishes:       s.WeightRespectWishes,
		WeightWeekendBalance:      s.WeightWeekendBalance,
		WeightSameShiftStreak:     s.WeightSameShiftStreak,
		WeightShortRestPattern:    s.WeightShortRestPattern,
	}
}

func loadSettings(ctx context.Context, pg *pgxpool.Pool) (*settingsBundle, error) {
	row := pg.QueryRow(ctx, `
		SELECT min_d, min_e, min_n,
		       max_consecutive_n, min_rest_after_n, max_consecutive_workdays,
		       balance_off_tolerance, previous_month_lookback_days,
		       weight_balance_off, weight_respect_wishes,
		       weight_weekend_balance, weight_same_shift_streak, weight_short_rest_pattern
		FROM ward_settings WHERE id = 1`)
	var s settingsBundle
	if err := row.Scan(
		&s.MinD, &s.MinE, &s.MinN,
		&s.MaxConsecutiveN, &s.MinRestAfterN, &s.MaxConsecutiveWorkdays,
		&s.BalanceOffTolerance, &s.PreviousMonthLookbackDays,
		&s.WeightBalanceOff, &s.WeightRespectWishes,
		&s.WeightWeekendBalance, &s.WeightSameShiftStreak, &s.WeightShortRestPattern,
	); err != nil {
		return nil, err
	}
	return &s, nil
}

// ----- levels -----

type levelsBundle struct {
	Rows []levels.Level
}

func (b *levelsBundle) toLevels() []levels.Level { return b.Rows }

func (b *levelsBundle) toSolver() []solver.ExperienceLevelIn {
	out := make([]solver.ExperienceLevelIn, 0, len(b.Rows))
	for _, l := range b.Rows {
		out = append(out, solver.ExperienceLevelIn{
			Code:              l.Code,
			MinD:              l.MinD,
			MinE:              l.MinE,
			MinN:              l.MinN,
			WeightCoverage:    l.WeightCoverage,
			WeightDAssignment: l.WeightDAssignment,
			WeightEAssignment: l.WeightEAssignment,
			WeightNAssignment: l.WeightNAssignment,
		})
	}
	return out
}

func loadLevels(ctx context.Context, pg *pgxpool.Pool) (*levelsBundle, error) {
	rows, err := pg.Query(ctx, `
		SELECT id, code, display_name, min_months, max_months,
		       min_d, min_e, min_n,
		       weight_coverage, weight_d_assignment, weight_e_assignment, weight_n_assignment,
		       sort_order, updated_at
		FROM experience_levels ORDER BY sort_order, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []levels.Level
	for rows.Next() {
		var l levels.Level
		if err := rows.Scan(&l.ID, &l.Code, &l.DisplayName, &l.MinMonths, &l.MaxMonths,
			&l.MinD, &l.MinE, &l.MinN,
			&l.WeightCoverage, &l.WeightDAssignment, &l.WeightEAssignment, &l.WeightNAssignment,
			&l.SortOrder, &l.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return &levelsBundle{Rows: out}, nil
}

// ----- nurses (active) -----

type activeNurse struct {
	id         int
	nurse      nurses.Nurse
	fixedShift *string
}

func loadActiveNurses(ctx context.Context, pg *pgxpool.Pool) ([]activeNurse, error) {
	rows, err := pg.Query(ctx, `
		SELECT id, name, role, email, hire_date, experience_level_override, fixed_shift_pattern, active
		FROM nurses WHERE active = TRUE
		ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []activeNurse
	for rows.Next() {
		var n nurses.Nurse
		var fp *string
		if err := rows.Scan(&n.ID, &n.Name, &n.Role, &n.Email, &n.HireDate,
			&n.ExperienceLevelOverride, &fp, &n.Active); err != nil {
			return nil, err
		}
		out = append(out, activeNurse{id: n.ID, nurse: n, fixedShift: fp})
	}
	return out, rows.Err()
}

// ----- night_keepers -----

func loadNightKeeperIDs(ctx context.Context, pg *pgxpool.Pool, ym string) (map[int]bool, error) {
	rows, err := pg.Query(ctx, `SELECT nurse_id FROM night_keeper_assignments WHERE year_month = $1`, ym)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]bool{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// ----- wishes -----

func loadWishesForMonth(ctx context.Context, pg *pgxpool.Pool, ym string) ([]solver.WishIn, error) {
	t, err := time.Parse("2006-01-02", ym+"-01")
	if err != nil {
		return nil, err
	}
	to := t.AddDate(0, 1, 0)
	rows, err := pg.Query(ctx, `
		SELECT nurse_id, date, type FROM wishes WHERE date >= $1 AND date < $2`, t, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	// nil 슬라이스를 반환하면 JSON marshal 시 `null` 이 되어 solver pydantic
	// validation에서 "Input should be a valid list" 으로 reject 됨.
	out := []solver.WishIn{}
	for rows.Next() {
		var w solver.WishIn
		if err := rows.Scan(&w.NurseID, &w.Date, &w.Type); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// 사용 안 함 — 미래 유틸용
var _ = strconv.Atoi
