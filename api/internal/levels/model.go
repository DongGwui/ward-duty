package levels

import "time"

// Level — experience_levels 1행.
// Design Ref: §3.1, G-04, H-12, S-10
type Level struct {
	ID                  int       `json:"id"`
	Code                string    `json:"code"`
	DisplayName         string    `json:"display_name"`
	MinMonths           int       `json:"min_months"`
	MaxMonths           *int      `json:"max_months,omitempty"`
	MinD                int       `json:"min_d"`
	MinE                int       `json:"min_e"`
	MinN                int       `json:"min_n"`
	WeightCoverage      int       `json:"weight_coverage"`
	WeightDAssignment   int       `json:"weight_d_assignment"`
	WeightEAssignment   int       `json:"weight_e_assignment"`
	WeightNAssignment   int       `json:"weight_n_assignment"`
	SortOrder           int       `json:"sort_order"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type CreateInput struct {
	Code              string `json:"code"`
	DisplayName       string `json:"display_name"`
	MinMonths         int    `json:"min_months"`
	MaxMonths         *int   `json:"max_months"`
	MinD              int    `json:"min_d"`
	MinE              int    `json:"min_e"`
	MinN              int    `json:"min_n"`
	WeightCoverage    int    `json:"weight_coverage"`
	WeightDAssignment int    `json:"weight_d_assignment"`
	WeightEAssignment int    `json:"weight_e_assignment"`
	WeightNAssignment int    `json:"weight_n_assignment"`
	SortOrder         int    `json:"sort_order"`
}

type UpdateInput struct {
	DisplayName       *string `json:"display_name,omitempty"`
	MinMonths         *int    `json:"min_months,omitempty"`
	MaxMonths         *int    `json:"max_months,omitempty"`
	MinD              *int    `json:"min_d,omitempty"`
	MinE              *int    `json:"min_e,omitempty"`
	MinN              *int    `json:"min_n,omitempty"`
	WeightCoverage    *int    `json:"weight_coverage,omitempty"`
	WeightDAssignment *int    `json:"weight_d_assignment,omitempty"`
	WeightEAssignment *int    `json:"weight_e_assignment,omitempty"`
	WeightNAssignment *int    `json:"weight_n_assignment,omitempty"`
	SortOrder         *int    `json:"sort_order,omitempty"`
}
