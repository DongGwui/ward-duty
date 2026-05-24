package nightkeepers

import "time"

type Assignment struct {
	ID                 int       `json:"id"`
	NurseID            int       `json:"nurse_id"`
	YearMonth          string    `json:"year_month"`
	AssignedByNurseID  *int      `json:"assigned_by_nurse_id,omitempty"`
	Reason             *string   `json:"reason,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
}

type CreateInput struct {
	NurseID   int     `json:"nurse_id"`
	YearMonth string  `json:"year_month"` // YYYY-MM
	Reason    *string `json:"reason,omitempty"`
}
