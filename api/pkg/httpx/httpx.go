// Package httpx — response envelope + request decode helpers.
package httpx

import (
	"encoding/json"
	"net/http"

	"ward-duty-api/pkg/apierr"
)

// Response envelope (Design §4.1, §6.2): { data?, error?, meta? }.
type Resp struct {
	Data any            `json:"data,omitempty"`
	Meta map[string]any `json:"meta,omitempty"`
}

// JSON — 성공 응답.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Resp{Data: data})
}

// JSONWithMeta — meta 포함 (예: violations).
func JSONWithMeta(w http.ResponseWriter, status int, data any, meta map[string]any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Resp{Data: data, Meta: meta})
}

// DecodeJSON — 요청 본문 디코드 + 400 자동 응답.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		apierr.Write(w, http.StatusBadRequest, apierr.CodeValidation, err.Error())
		return false
	}
	return true
}
