// Package solver — Python solver와 1:1 JSON 스키마.
// Design Ref: §4.3, §9.1 (solver 슬라이스)
//
// solver/app/schemas.py의 Pydantic 모델과 필드명 일치해야 함.
package solver

import "time"

type NurseIn struct {
	ID            int     `json:"id"`
	Level         string  `json:"level"`
	FixedPattern  *string `json:"fixed_pattern,omitempty"` // H-11
	IsNightKeeper bool    `json:"is_night_keeper"`         // K-01
}

type WishIn struct {
	NurseID int       `json:"nurse_id"`
	Date    time.Time `json:"date"` // YYYY-MM-DD 마샬
	Type    string    `json:"type"` // off|d|e|n|unavailable
}

type PrevCellIn struct {
	NurseID int       `json:"nurse_id"`
	Date    time.Time `json:"date"`
	Shift   string    `json:"shift"` // D|E|N|O|DE
}

type ExperienceLevelIn struct {
	Code              string `json:"code"`
	MinD              int    `json:"min_d"`
	MinE              int    `json:"min_e"`
	MinN              int    `json:"min_n"`
	WeightCoverage    int    `json:"weight_coverage"`
	WeightDAssignment int    `json:"weight_d_assignment"`
	WeightEAssignment int    `json:"weight_e_assignment"`
	WeightNAssignment int    `json:"weight_n_assignment"`
}

type WardSettingsIn struct {
	MinD                     int `json:"min_d"`
	MinE                     int `json:"min_e"`
	MinN                     int `json:"min_n"`
	MaxConsecutiveN          int `json:"max_consecutive_n"`
	MinRestAfterN            int `json:"min_rest_after_n"`
	MaxConsecutiveWorkdays   int `json:"max_consecutive_workdays"`
	BalanceOffTolerance      int `json:"balance_off_tolerance"`
	WeightBalanceOff         int `json:"weight_balance_off"`
	WeightRespectWishes      int `json:"weight_respect_wishes"`
	WeightWeekendBalance     int `json:"weight_weekend_balance"`
	WeightSameShiftStreak    int `json:"weight_same_shift_streak"`
	WeightShortRestPattern   int `json:"weight_short_rest_pattern"`
}

// ----- Input -----

type GenerateInput struct {
	YearMonth                  string              `json:"year_month"`
	Nurses                     []NurseIn           `json:"nurses"`
	Wishes                     []WishIn            `json:"wishes"`
	PreviousMonthLastWeekCells []PrevCellIn        `json:"previous_month_last_week_cells"` // H-10
	ExperienceLevels           []ExperienceLevelIn `json:"experience_levels"`
	WardSettings               WardSettingsIn      `json:"ward_settings"`
	MaxTimeSeconds             int                 `json:"max_time_seconds"`
}

type CellOut struct {
	NurseID int       `json:"nurse_id"`
	Date    time.Time `json:"date"`
	Shift   string    `json:"shift"`
}

type ValidateInput struct {
	YearMonth                  string              `json:"year_month"`
	Nurses                     []NurseIn           `json:"nurses"`
	Wishes                     []WishIn            `json:"wishes"`
	PreviousMonthLastWeekCells []PrevCellIn        `json:"previous_month_last_week_cells"`
	ExperienceLevels           []ExperienceLevelIn `json:"experience_levels"`
	WardSettings               WardSettingsIn      `json:"ward_settings"`
	AllCells                   []CellOut           `json:"all_cells"`
}

// ----- Output -----

type GenerateOutput struct {
	Status           string    `json:"status"` // ok|infeasible|timeout|error
	SolverStatus     string    `json:"solver_status"`
	ObjectiveValue   *int      `json:"objective_value,omitempty"`
	Cells            []CellOut `json:"cells"`
	AppliedRules     []string  `json:"applied_rules"`
	ElapsedMs        int       `json:"elapsed_ms"`
	ViolatedRuleIDs  []string  `json:"violated_rule_ids,omitempty"`
	Suggestion       *string   `json:"suggestion,omitempty"`
}

type Violation struct {
	RuleID   string     `json:"rule_id"`
	Severity string     `json:"severity"` // hard|soft
	Message  string     `json:"message"`
	CellIDs  []int      `json:"cell_ids,omitempty"`
	NurseID  *int       `json:"nurse_id,omitempty"`
	Date     *time.Time `json:"date,omitempty"`
}

type ValidateOutput struct {
	Violations []Violation `json:"violations"`
	HardCount  int         `json:"hard_count"`
	SoftCount  int         `json:"soft_count"`
}
