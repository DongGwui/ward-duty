package notifications

import (
	"encoding/json"
	"time"
)

// Type 상수.
const (
	TypeAccountPendingApproval = "account_pending_approval"
	TypeSwapRequestReceived    = "swap_request_received"
	TypeSwapBAccepted          = "swap_b_accepted"
	TypeSwapApproved           = "swap_approved"
	TypeSwapRejected           = "swap_rejected"
	TypeLevelChanged           = "level_changed"
	TypeFixedPatternChanged    = "fixed_pattern_changed"
	TypeNightKeeperAssigned    = "nightkeeper_assigned"
	TypeScheduleConfirmed      = "schedule_confirmed"
)

type Notification struct {
	ID         int             `json:"id"`
	Type       string          `json:"type"`
	Title      string          `json:"title"`
	Body       *string         `json:"body,omitempty"`
	Link       *string         `json:"link,omitempty"`
	Meta       json.RawMessage `json:"meta,omitempty"`
	ReadAt     *time.Time      `json:"read_at,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

// Create — service에서 사용할 입력.
type Create struct {
	RecipientNurseID int
	Type             string
	Title            string
	Body             string
	Link             string
	Meta             map[string]any
}
