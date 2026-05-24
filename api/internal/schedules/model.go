package schedules

import (
	"encoding/json"
	"time"

	"ward-duty-api/pkg/dateonly"
)

type Schedule struct {
	ID             int             `json:"id"`
	YearMonth      string          `json:"year_month"`
	Status         string          `json:"status"` // draft|generating|generated|confirmed|failed
	GeneratedAt    *time.Time      `json:"generated_at,omitempty"`
	ConfirmedAt    *time.Time      `json:"confirmed_at,omitempty"`
	GenerationLog  json.RawMessage `json:"generation_log,omitempty"`
}

type ScheduleCell struct {
	ID                  int           `json:"id"`
	ScheduleID          int           `json:"schedule_id"`
	NurseID             int           `json:"nurse_id"`
	Date                dateonly.Date `json:"date"`
	Shift               string        `json:"shift"`  // D|E|N|O|DE
	Source              string        `json:"source"` // auto|manual
	ModifiedByNurseID   *int          `json:"modified_by_nurse_id,omitempty"`
	UpdatedAt           time.Time     `json:"updated_at"`
}

type CreateInput struct {
	YearMonth string `json:"year_month"` // YYYY-MM
}

type PatchCellInput struct {
	Shift string `json:"shift"` // D|E|N|O|DE
}

// GenerationLog — JSONB body (§10 Infeasibility Policy).
type GenerationLog struct {
	SolverStatus    string         `json:"solver_status"`
	ViolatedRuleIDs []string       `json:"violated_rule_ids,omitempty"`
	Suggestion      string         `json:"suggestion,omitempty"`
	ElapsedMs       int            `json:"elapsed_ms,omitempty"`
	AppliedRules    []string       `json:"applied_rules,omitempty"`
	InputSummary    map[string]any `json:"input_summary,omitempty"`
}
