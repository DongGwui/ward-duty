package nurses

import "time"

// Nurse — DB 행.
type Nurse struct {
	ID                       int        `json:"id"`
	Name                     string     `json:"name"`
	Role                     string     `json:"role"` // head_nurse | nurse
	Email                    string     `json:"email"`
	GoogleSub                *string    `json:"google_sub,omitempty"`
	HireDate                 *time.Time `json:"hire_date,omitempty"`
	ExperienceLevelOverride  *string    `json:"experience_level_override,omitempty"`
	FixedShiftPattern        *string    `json:"fixed_shift_pattern,omitempty"`
	Active                   bool       `json:"active"`
	LastLoginAt              *time.Time `json:"last_login_at,omitempty"`
	CreatedAt                time.Time  `json:"created_at"`
	// 응답 시 derived
	ResolvedLevel string `json:"resolved_level,omitempty"`
}

type CreateInput struct {
	Name                     string     `json:"name"`
	Email                    string     `json:"email"`
	Role                     string     `json:"role"`               // 생략 시 'nurse'
	HireDate                 *time.Time `json:"hire_date,omitempty"`
	ExperienceLevelOverride  *string    `json:"experience_level_override,omitempty"`
	FixedShiftPattern        *string    `json:"fixed_shift_pattern,omitempty"`
}

type UpdateInput struct {
	Name                     *string    `json:"name,omitempty"`
	Role                     *string    `json:"role,omitempty"`
	HireDate                 *time.Time `json:"hire_date,omitempty"`
	ExperienceLevelOverride  *string    `json:"experience_level_override,omitempty"`
	FixedShiftPattern        *string    `json:"fixed_shift_pattern,omitempty"`
	Active                   *bool      `json:"active,omitempty"`
}
