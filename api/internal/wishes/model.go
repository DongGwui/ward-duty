package wishes

import (
	"time"

	"ward-duty-api/pkg/dateonly"
)

// Wish — W-01
type Wish struct {
	ID        int           `json:"id"`
	NurseID   int           `json:"nurse_id"`
	Date      dateonly.Date `json:"date"`
	Type      string        `json:"type"` // off|d|e|n|unavailable
	Reason    *string       `json:"reason,omitempty"`
	CreatedAt time.Time     `json:"created_at"`
}

// UpsertInput — PUT /api/wishes/{date} 본문.
type UpsertInput struct {
	Type   string  `json:"type"`
	Reason *string `json:"reason,omitempty"`
}
