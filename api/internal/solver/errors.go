package solver

import "errors"

// 솔버 호출 결과를 분기할 때 쓰는 sentinel error.
var (
	ErrUnavailable = errors.New("solver: unavailable")    // 503 — 컨테이너 다운/네트워크
	ErrTimeout     = errors.New("solver: timeout")        // 504 — solver max_time 초과
	ErrInfeasible  = errors.New("solver: infeasible")     // 422 — §10 정책에 따라 사유와 함께 반환
	ErrUnknown     = errors.New("solver: unknown status") // 500
)
