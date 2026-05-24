package swaps

import (
	"time"

	"ward-duty-api/pkg/dateonly"
)

// Status 상수.
const (
	StatusPending        = "pending"
	StatusBAccepted      = "b_accepted"
	StatusApproved       = "approved"
	StatusRejectedByB    = "rejected_by_b"
	StatusRejectedByHead = "rejected_by_head"
	StatusCancelled      = "cancelled"
)

type SwapRequest struct {
	ID                  int           `json:"id"`
	ScheduleID          int           `json:"schedule_id"`
	RequesterNurseID    int           `json:"requester_nurse_id"`
	TargetNurseID       int           `json:"target_nurse_id"`
	RequesterDate       dateonly.Date `json:"requester_date"`
	TargetDate          dateonly.Date `json:"target_date"`
	Status              string        `json:"status"`
	Reason              *string       `json:"reason,omitempty"`
	RejectedReason      *string       `json:"rejected_reason,omitempty"`
	CreatedAt           time.Time     `json:"created_at"`
	UpdatedAt           time.Time     `json:"updated_at"`
}

type CreateInput struct {
	ScheduleID     int           `json:"schedule_id"`
	TargetNurseID  int           `json:"target_nurse_id"`
	RequesterDate  dateonly.Date `json:"requester_date"`
	TargetDate     dateonly.Date `json:"target_date"`
	Reason         *string       `json:"reason,omitempty"`
}

type PatchInput struct {
	Action string  `json:"action"` // accept|reject|approve
	Reason *string `json:"reason,omitempty"`
}
