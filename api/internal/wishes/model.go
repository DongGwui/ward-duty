package wishes

import "time"

// Wish — W-01
type Wish struct {
	ID        int       `json:"id"`
	NurseID   int       `json:"nurse_id"`
	Date      time.Time `json:"date"`
	Type      string    `json:"type"` // off|d|e|n|unavailable
	Reason    *string   `json:"reason,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// UpsertInput — PUT /api/wishes/{date} 본문.
type UpsertInput struct {
	Type   string  `json:"type"`
	Reason *string `json:"reason,omitempty"`
}
