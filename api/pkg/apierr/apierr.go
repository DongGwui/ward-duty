// Package apierr — Design §6.2 error envelope.
package apierr

import (
	"encoding/json"
	"net/http"
)

// Error envelope (Design §6.2).
type Envelope struct {
	Error *Error `json:"error"`
}

type Error struct {
	Code      string         `json:"code"`
	Message   string         `json:"message"`
	RuleID    string         `json:"rule_id,omitempty"`
	Details   map[string]any `json:"details,omitempty"`
	RequestID string         `json:"request_id,omitempty"`
}

// 자주 쓰는 에러 코드 (Design §6.1).
const (
	CodeValidation        = "VALIDATION_ERROR"
	CodeUnauthorized      = "UNAUTHORIZED"
	CodeForbidden         = "FORBIDDEN"
	CodeNotFound          = "NOT_FOUND"
	CodeConflict          = "CONFLICT"
	CodeRuleViolation     = "RULE_VIOLATION"
	CodeInternal          = "INTERNAL"
	CodeSolverUnavailable = "SOLVER_UNAVAILABLE"
	CodeOAuthState        = "OAUTH_STATE_INVALID"
	CodeNotInvited        = "NOT_INVITED"
)

// Write — 표준 envelope JSON 응답.
func Write(w http.ResponseWriter, status int, code, msg string) {
	WriteFull(w, status, &Error{Code: code, Message: msg})
}

func WriteFull(w http.ResponseWriter, status int, e *Error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Error: e})
}

// Rule — RULE_VIOLATION 전용 헬퍼.
func Rule(w http.ResponseWriter, ruleID, message string, details map[string]any) {
	WriteFull(w, http.StatusUnprocessableEntity, &Error{
		Code:    CodeRuleViolation,
		RuleID:  ruleID,
		Message: message,
		Details: details,
	})
}
