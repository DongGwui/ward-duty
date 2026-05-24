---
doc: intermediate-review
project: ward-duty
checkpoint: 1
phase: do (Module 1-5 backend complete)
date: 2026-05-24
agents:
  - bkit:code-analyzer
  - bkit:security-architect
---

# Checkpoint 1 — 백엔드 종합 점검 리포트

> **시점**: Module 1~5 완료 직후 (Go API + Python solver 빌드·`go vet`·`pytest 24/24` 통과 상태).
> **범위**: Web 제외. 인프라/DB/솔버/Go API.
> **방법**: bkit `code-analyzer` + `security-architect` 병렬 + 솔버↔Go JSON 계약 수동 검증.

---

## Executive Summary

| 영역 | 평가 |
|---|---|
| 아키텍처 일관성 | ✅ Vertical Slices 분리·의존방향 정확 |
| 정적 분석 | ✅ `go vet 0` / `pytest 24/24` / `ruff clean` |
| 솔버↔Go JSON 계약 | ⚠️ 1건 불일치 (`previous_month_lookback_days` 누락 — 양쪽 모두) |
| OAuth 보안 | 🔴 **다층 결함 5건** — id_token 서명 미검증·nonce 미검증·access fragment 노출·refresh CSRF·솔버 token 가드 |
| 운영 안정성 | 🔴 schedule `generating` 영구 잔존·nil deref·background ctx 누수 |
| 일반 코드 품질 | 🟡 err.Error() 노출, 보안 헤더 부재 등 |

**결론**: 직접적 모순은 없지만 **Web 단계 진입 전 Critical 5건은 반드시 수정**. 나머지는 Web 작업과 병행 가능.

---

## 🔴 Critical — Web 진입 전 차단 (5건)

각 항목은 인증 우회·세션 탈취·운영 사고로 직결.

### C1 · OAuth id_token 서명 미검증
- 위치: `api/internal/auth/oauth.go:131-138, parseIDToken (205-222)`
- 문제: `base64.RawURLEncoding.DecodeString(parts[1])`로 payload만 디코드. RSA 서명·iss·aud·exp·nonce 검증 0
- 결과: 위조 id_token이 callback에 주입되면 임의 email로 화이트리스트 우회 → 권한 탈취. OIDC §3.1.3.7 위반
- 수정: `google.golang.org/api/idtoken.Validate(ctx, idToken, clientID)` 1줄로 해소 (Google JWKS 캐시·iss·aud·exp 자동)
- C5 (nonce 미검증)도 함께 해소

### C2 · access_token URL fragment 노출
- 위치: `api/internal/auth/handler.go:74-76` (`target := frontPath() + "#access_token=" + access`)
- 문제: fragment는 서버 미전송이지만 brower history·확장·3rd-party JS의 `location.hash` 접근·Referer 헤더(드물게) 통해 추출 가능
- 결과: 15m이지만 XSS 1회로 토큰 탈취
- 수정: access도 httpOnly+Secure 쿠키로 발급. 또는 1회용 30s 교환 코드를 fragment로 주고 `POST /api/auth/exchange`로 실제 access를 받음

### C3 · Refresh 회전 race + Redis SCAN 매칭
- 위치: `api/internal/auth/session.go:90-122`, `handler.go:80-124`
- 문제 ①: `GET key` → 검사 → `DEL key` 비원자 → 동시 2건이 둘 다 새 토큰 발급, 표준 "탈취 탐지" 무력화
- 문제 ②: `FindNurseIDByRefresh`가 `refresh:*` 전체 SCAN → nid 모르는 공격자도 임의 토큰으로 회전 강제 가능 (사용성 DoS)
- 수정 ①: Redis `GETDEL` (6.2+) 또는 Lua script로 atomic consume
- 수정 ②: `refresh:lookup:<sha256> -> nid` 보조 키로 O(1) 매칭. SCAN 폐기

### C4 · Solver internal token 미설정 시 무인증 통과
- 위치: `solver/app/routes.py:25-33`
- 문제: `if not _EXPECTED_TOKEN: return  # 토큰 미설정 환경(테스트)에서는 통과` → 운영에서 env 누락/빈 문자열 시 어떤 요청이든 수락
- 결과: solver는 internal-only 약속이지만 다층 방어가 무너짐
- 수정 ①: `ENV=production`이거나 token이 빈 값이면 startup fail
- 수정 ②: `secrets.compare_digest(x_internal_token or "", _EXPECTED_TOKEN)` 상수시간 비교

### C5 · Schedule `generating` 상태 영구 잔존
- 위치: `api/internal/schedules/service.go:43, repo.go:45-56`
- 문제: `go s.runSolver(context.Background(), ...)` 시작 직후 프로세스 재시작/패닉 시 status='generating'이 영구 잔존. UNIQUE(year_month) + service.go가 confirmed만 차단해 새 dispatch도 어색하게 막힘
- 결과: 해당 월 schedule을 다시 생성할 방법 부재 → DB 직접 접근 필요
- 수정 ①: `InsertGenerating`에서 status='failed'/'draft'/'generating' AND `updated_at < now() - INTERVAL '2 * solver_timeout'`이면 status='generating' UPDATE + 새 dispatch
- 수정 ②: shutdown 시 in-flight runSolver를 WaitGroup으로 대기 (C6과 결합)

---

## 🟠 High — Web 병행 가능, 1차 배포 전 처리 (12건)

| # | 영역 | 위치 | 요약 | 수정 |
|---|---|---|---|---|
| H1 | OIDC nonce 검증 누락 | `pkg/oauthx/state.go`, `oauth.go` | nonce 생성·전달은 하지만 콜백에서 id_token의 nonce 클레임과 비교 없음 | C1 해결 라이브러리 도입 시 `state.Nonce == claims.Nonce` 검증 |
| H2 | ADMIN_EMAIL 영구 자동 시드 | `auth/handler.go:155-196` | `head_nurse` count 무관하게 매번 자동 시드 → env 변경만으로 권한 탈취 | `SELECT COUNT(*) FROM nurses WHERE role='head_nurse' = 0`일 때만 |
| H3 | 솔버 호출 nil deref | `schedules/service.go:61-69` | `out.ElapsedMs` 참조 시 out이 nil이면 panic (goroutine 안) → process crash | `out == nil` 가드 또는 client.Generate가 항상 non-nil out 반환 |
| H4 | background ctx 사용으로 shutdown race | `schedules/service.go:43` | `context.Background()`로 DB·HTTP 호출 → graceful shutdown 시 pool close 후 연결 실패 | appCtx 주입 + WaitGroup |
| H5 | swap approve 비-원자 + 보상 실패 가능 | `swaps/service.go:112-158` | `SwapCellShifts` → validate → 위반 시 다시 `SwapCellShifts`로 롤백. 두 번째 swap이 실패하면 cells 잔존, 동시 PATCH/swap과 race | 단일 TX로 묶거나 advisory lock |
| H6 | confirmed schedule도 PATCH 가능 | `schedules/handler.go:86-136`, `repo.go:154` | `UpdateCell`이 schedule.status 검사 X → 확정 후 임의 수정 | UPDATE에 `EXISTS(... WHERE status IN ('generated','draft'))` 가드 |
| H7 | raw `err.Error()` 응답 노출 | 다수 핸들러 | pgx 에러 메시지(SQL/컬럼) 클라이언트에 노출 | 5xx는 generic + `request_id`만, 상세는 서버 로그 |
| H8 | 보안 헤더 전무 | `cmd/server/main.go:70-76` | HSTS, X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Cache-Control 없음 | secureheaders 미들웨어. 인증 응답엔 `Cache-Control: no-store` + `Referrer-Policy: no-referrer` |
| H9 | `FRONTEND_REDIRECT_PATH` open redirect 위험 | `auth/handler.go:198-203` | env값 검증 없이 Location 헤더에 사용 | `strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//")`만 허용 |
| H10 | rate limit 부재 | 라우터 전체 | /auth/* brute force, /schedules 솔버 스팸 가능 | Chi `httprate` 또는 자작 미들웨어 (IP+nid 2단) |
| H11 | `previous_month_lookback_days` 스키마 불일치 | `solver/app/schemas.py:WardSettingsIn`, `api/internal/solver/schemas.go`, `loaders.go:34-49` | DB에 컬럼·default 7 있고 Go도 로드하지만 **솔버 입력에 전달 안 됨**. Python `model.py`도 `lookback=7` 하드코딩 → settings 변경이 솔버에 무반영. **KPI에 영향** | 양쪽 `WardSettingsIn`에 필드 추가 + `toSolver()`에서 전달 + `model.py`·`validator.py`에서 사용 |
| H12 | infeasibility suggestion 정밀화 부족 | `solver/app/infeasibility.py`, `schedules/service.go` | infeasible 시 위반 룰을 H-04로만 단일 보고. 어떤 등급/날짜에서 막혔는지 정보 부족 → KPI 측정 어려움 | 등급별 인원 미충족 사전 시뮬레이션 + violated_rule_ids에 H-12 후보 등급 명시 |

---

## 🟡 Medium / Low — 누적 부담 회피 차원 (8건, 압축)

- `loaders.go:197`, `auth/handler.go:206`, `swaps/service.go:173`의 `var _ = ...` dead silencers — 스타일
- pkg/oauthx 쿠키 Secure flag 결정이 env prefix(`https://`)만 보는 휴리스틱 → `ENV=production` 명시 권장
- Chi `Timeout(60s)` vs 솔버 300s 불일치 — `/schedules` 라우트는 비동기라 영향 적지만 정합 권장
- JWT secret 길이 검증 부재 — startup에서 `len(JWT_SECRET) < 32` 차단
- JWT iss/aud 검증 누락 — `jwt.WithIssuer("ward-duty-api")` + aud 추가
- `Refresh` 쿠키 `__Host-` prefix 미사용 — 환경 일관성 위해 `__Host-wd_refresh` 검토
- S-05 short_rest 상수 BoolVar 캐싱 부재 (모델 크기 7K 추가 BoolVar) — 성능 미세
- failed schedule 재생성 흐름 부재 (`InsertGenerating`이 failed row를 그대로 반환) — H10 해결과 함께

---

## 솔버 ↔ Go 계약 일치 검증 결과

`grep` 기반 양쪽 schema 1:1 비교 결과:

| Schema | Python 필드 수 | Go 필드 수 | 일치? |
|---|:-:|:-:|:-:|
| `NurseIn` | 4 | 4 | ✅ |
| `WishIn` | 3 | 3 | ✅ |
| `PrevCellIn` | 3 | 3 | ✅ |
| `ExperienceLevelIn` | 8 | 8 | ✅ |
| **`WardSettingsIn`** | **12** | **12** | ⚠️ **양쪽 모두 `previous_month_lookback_days` 누락 → H11** |
| `GenerateInput` | 7 | 7 | ✅ |
| `ValidateInput` | 7 | 7 | ✅ |
| `CellOut` | 3 | 3 | ✅ |
| `GenerateOutput` | 8 | 8 | ✅ |
| `Violation` | 6 | 6 | ✅ |
| `ValidateOutput` | 3 | 3 | ✅ |

JSON 키 네이밍은 완전 일치(snake_case 100%). 단 **H11이 KPI에 영향**.

---

## 권장 수정 순서

```
[Stage A] Critical 5 — Web 진입 전 필수 (~25-35 turn)
  1. C1+H1+C5(=C5 nonce) — idtoken.Validate 도입 (한 번에 3건 해소)
  2. C2 — access_token httpOnly 쿠키 (또는 1회용 교환 코드)
  3. C3 — Redis GETDEL + refresh:lookup:<hash> 보조 키
  4. C4 — solver token startup guard + compare_digest
  5. C5 (schedule generating) — InsertGenerating 재시도 로직 + watchdog 쿼리

[Stage B] High 우선 (~30-40 turn) — Web 작업과 병행 가능
  6. H11 — previous_month_lookback_days 스키마 추가 (KPI 영향)
  7. H2 — ADMIN_EMAIL 1회용 시드
  8. H3, H4 — schedules 안정성
  9. H5 — swap approve 단일 TX
  10. H6 — confirmed schedule lock
  11. H7 — err.Error() 마스킹
  12. H8 — 보안 헤더 미들웨어

[Stage C] Medium — 1차 배포 직후 sprint
  13. H9, H10, H12 + Medium 8건 일괄
```

---

## 다음 액션 (선택)

| 옵션 | 내용 | 예상 turn |
|---|---|:-:|
| **A** | **Stage A 5건 지금 수정** 후 Web 진입 | 25-35 |
| B | Stage A+B 통합 수정 (보안+안정성 모두) | 60-75 |
| C | Web 진입하고 Stage A·B를 별도 sprint로 | 0 (지금) |

권장은 **A** — Web의 OAuth UI를 붙이기 전에 인증 흐름의 가장 기본적 결함(특히 C1)이 해소돼야 frontend 작업의 가정이 무너지지 않습니다.

---

## Appendix — 사용된 agents

| Agent | Tokens | Duration | 산출 |
|---|---:|---:|---|
| code-analyzer | 109K | 218s | 17 findings (5C/12I) |
| security-architect | 67K | 245s | 14 findings (5C/9H) + 5 정상 확인 |
| 수동 계약 검증 | 0 | <1s | 1 불일치 (H11) |

총 발견 항목 → 중복 통합 후 17 (Critical 5 / High 12 / Medium~Low 8). 본 리포트는 중복 제거된 결과.
