---
template: design
version: 1.3
feature: ward-duty-mvp
date: 2026-05-24
author: yang-donggwui
project: ward-duty
version_project: 0.1.0
selected_architecture: C-Pragmatic
---

# ward-duty-mvp Design Document

> **Summary**: Pragmatic Vertical Slices 채택. Go API 도메인 패키지 + Python 솔버 순수 함수(룰 ID 1:1 매핑) + Next.js feature 폴더 구조로 룰 추적성과 KPI 튜닝 사이클을 동시에 확보.
>
> **Project**: ward-duty
> **Version**: 0.1.0
> **Author**: yang-donggwui
> **Date**: 2026-05-24
> **Status**: Draft
> **Planning Doc**: [ward-duty-mvp.plan.md](../../01-plan/features/ward-duty-mvp.plan.md)
> **Rules Doc**: [scheduling-rules.md](../../rules/scheduling-rules.md) v0.4

---

## Context Anchor

| Key | Value |
|-----|-------|
| **WHY** | 수간호사의 듀티 작성 부담(월 수 시간 수기) + 팀원 희망일 수집 누락·갈등 해소 |
| **WHO** | head_nurse (작성·수정·확정·등급/나이트킵 관리) / nurse (희망일·조회·swap) |
| **RISK** | 자동 생성 KPI 80% 미달 / Cloudflare 100s 타임아웃 / 솔버 infeasible / 1인 의존 |
| **SUCCESS** | 자동 초안 80%+ 무수정 / 30명×31일 ≤60s / hard 위반 0 / 모바일 희망일 3-탭 |
| **SCOPE** | Next.js + Go + Python solver / 한 병동 10~30명 / 홈서버 self-host / 월 0원 |

---

## 1. Overview

### 1.1 Design Goals

1. **룰 추적성** — 룰 v0.4의 41개 룰이 코드(솔버 함수명·테스트 이름·UI 에러 코드)와 1:1 매핑
2. **KPI 튜닝 사이클** — 솔버 핵심부가 순수 함수라 회귀 테스트 위에서 가중치를 빠르게 흔들 수 있음
3. **장애 격리** — 솔버 OOM/CPU 폭주가 API/web에 영향 없음 (별도 컨테이너)
4. **운영 단순성** — 홈서버 `docker compose up -d` 한 줄로 끝
5. **월 경계 정확성** — 이전 달 마지막 7일을 상수 입력으로 받는 솔버 계약

### 1.2 Design Principles

- **Vertical Slice over Horizontal Layer** — 도메인 패키지(`schedules/`, `nurses/`, ...) 안에 handler/service/repo 응집
- **Pure Solver Core** — `model.py`의 CP-SAT 모델 빌드 함수는 입력 → 모델 객체 반환, I/O·DB 접근 없음
- **Rule ID = First-class Identifier** — 솔버 함수·테스트·UI 에러 메시지·DB 컬럼 코멘트에 룰 ID 명시
- **Async by Default** — 솔버 호출은 비동기·폴링 (Cloudflare 100s 타임아웃 회피)
- **Self-host First** — bkend.ai/Vercel 등 외부 의존 0, 홈서버 shared-postgres·Traefik 활용

---

## 2. Architecture

### 2.0 Architecture Comparison

| Criteria | Option A: Lean | Option B: Hexagonal | **Option C: Pragmatic** |
|----------|:-:|:-:|:-:|
| **Approach** | 3-layer flat | Port-Adapter 완전 분리 | 도메인 수직 슬라이스 |
| **New Files** | ~20 | ~45 | **~28** |
| **Complexity** | Low | High | Medium |
| **룰 추적성** | △ | ✅ | ✅ (ID 매핑) |
| **솔버 테스트 용이도** | △ | ✅ | ✅ |
| **Effort** | Low | High | Medium |
| **Risk** | 룰 분산 | 1인 MVP 과함 | 균형 |
| **Recommendation** | 빠른 PoC | 장기 대규모 | **Default choice** |

**Selected**: **Option C — Pragmatic Vertical Slices**
**Rationale**: 살아있는 룰 문서(v0.4 41개)와 코드 1:1 매핑이 핵심 요건. 솔버 순수 함수 + 도메인별 패키지로 KPI 튜닝 사이클과 룰 변경 영향 분석을 모두 단순화.

### 2.1 Component Diagram

```
인터넷
  └─ Cloudflare Tunnel ─────────────► Traefik (호스트 :80)
                                        │
                       ┌────────────────┼────────────────┐
                       ▼                ▼                ▼
              ward-duty-web      ward-duty-api    (no public route)
              (Next.js :3000)    (Go Chi :8080)
                       │                │
                       │           ┌────┴────┐
                       │           ▼         ▼
                       │     shared-postgres ward-duty-redis
                       │     (DB: ward_duty) (전용)
                       │           │
                       │           ▼ (internal-only HTTP)
                       │     ward-duty-solver (FastAPI :8000)
                       │           │
                       │           └─ OR-Tools CP-SAT
                       │
                       └─ static assets (Next.js standalone)

Networks:
  proxy             : Traefik ↔ web/api
  shared            : api ↔ shared-postgres / shared-minio (미사용)
  ward-duty-internal: api ↔ solver ↔ ward-duty-redis
```

### 2.2 Data Flow (핵심 3시나리오)

#### A. 자동 생성 (비동기 + 폴링)
```
Browser → POST /api/schedules { year_month }
api:
  1. INSERT schedules(status='generating')
  2. load nurses → 등급 분류 (G-04, experience_levels 매핑)
  3. load wishes (year_month)
  4. load previous_month_last_week cells (H-10)
  5. load fixed_shift_pattern per nurse (H-11)
  6. load night_keeper_assignments (K-01)
  7. load ward_settings + experience_levels (H-12, S-10)
  8. async goroutine: POST solver:/generate { input JSON }
  9. → 202 { schedule_id, status: 'generating' }
Browser ← poll GET /api/schedules/:id (3s 간격)
solver: CP-SAT 풀이 (10~50s)
  → cells JSON 반환
api: UPSERT schedule_cells (source='auto') + UPDATE status='generated'
Browser: status=='generated' → GET /cells → grid render
```

#### B. 수동 수정 + 실시간 검증
```
드래그 → PATCH /api/schedules/:id/cells/:cellId { shift }
api:
  1. UPDATE cell (source='manual', modified_by_nurse_id)
  2. POST solver:/validate { all_cells_after_change, context }
  3. solver → { violations: [{ rule_id, message, severity }] }
  4. 응답
Browser: 위반 셀에 빨간 테두리 + 툴팁(rule_id 표시), hard 위반이면 확정 차단
```

#### C. Swap 워크플로우
```
A → POST /api/swaps                                  status=pending
B → PATCH /api/swaps/:id { accept }                  status=b_accepted
head_nurse → PATCH /api/swaps/:id { approve }
  TX BEGIN
    1. 가상 cells 적용 (A↔B 시프트 교환)
    2. POST solver:/validate (X-03 검증 범위: 9개 hard 룰)
    3. violations ≠ ∅ → ROLLBACK, status='rejected_by_head', reason 기록
    4. else → 실제 UPDATE schedule_cells × 2 + UPDATE swap status='approved'
  TX COMMIT
```

### 2.3 Dependencies

| Component | Depends On | Purpose |
|---|---|---|
| ward-duty-web | ward-duty-api (HTTPS) | UI 요청 |
| ward-duty-api | shared-postgres | 영속 저장 |
| ward-duty-api | ward-duty-redis | 세션·refresh 토큰·솔버 결과 캐시 |
| ward-duty-api | ward-duty-solver (internal) | 자동 생성·검증 |
| ward-duty-solver | OR-Tools (Python pkg) | CP-SAT 풀이 |
| Traefik | proxy 네트워크 | 라우팅 |
| Cloudflare Tunnel | dltmxm.link | 외부 노출 |

---

## 3. Data Model

### 3.1 DDL (PostgreSQL, DB `ward_duty`)

```sql
-- 0001_init.sql

CREATE TYPE nurse_role AS ENUM ('head_nurse', 'nurse');
CREATE TYPE fixed_pattern AS ENUM ('D_ONLY', 'E_ONLY', 'N_ONLY', 'WEEKDAY_D', 'WEEKDAY_E');
CREATE TYPE wish_type AS ENUM ('off', 'd', 'e', 'n', 'unavailable');
CREATE TYPE shift_code AS ENUM ('D', 'E', 'N', 'O', 'DE');
CREATE TYPE schedule_status AS ENUM ('draft', 'generating', 'generated', 'confirmed', 'failed');
CREATE TYPE cell_source AS ENUM ('auto', 'manual');
CREATE TYPE swap_status AS ENUM (
  'pending', 'b_accepted', 'approved',
  'rejected_by_b', 'rejected_by_head', 'cancelled'
);

-- 등급 시스템 (G-04, H-12, S-10)
CREATE TABLE experience_levels (
  id              SERIAL PRIMARY KEY,
  code            TEXT NOT NULL UNIQUE,
  display_name    TEXT NOT NULL,
  min_months      INT NOT NULL DEFAULT 0,
  max_months      INT,
  min_d           INT NOT NULL DEFAULT 0,
  min_e           INT NOT NULL DEFAULT 0,
  min_n           INT NOT NULL DEFAULT 0,
  weight_coverage INT NOT NULL DEFAULT 1,
  weight_d_assignment INT NOT NULL DEFAULT 0,
  weight_e_assignment INT NOT NULL DEFAULT 0,
  weight_n_assignment INT NOT NULL DEFAULT 0,
  sort_order      INT NOT NULL DEFAULT 0,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE nurses (
  id              SERIAL PRIMARY KEY,
  name            TEXT NOT NULL,
  role            nurse_role NOT NULL DEFAULT 'nurse',
  email           TEXT NOT NULL UNIQUE,              -- 화이트리스트 키 (FR-10)
  google_sub      TEXT UNIQUE,                       -- Google subject ID, 첫 로그인 시 upsert
  hire_date       DATE,                              -- G-04 자동 분류 기준
  experience_level_override TEXT REFERENCES experience_levels(code),
  fixed_shift_pattern fixed_pattern,                 -- H-11
  active          BOOLEAN NOT NULL DEFAULT TRUE,
  last_login_at   TIMESTAMPTZ,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_nurses_active ON nurses(active);

CREATE TABLE wishes (
  id              SERIAL PRIMARY KEY,
  nurse_id        INT NOT NULL REFERENCES nurses(id),
  date            DATE NOT NULL,
  type            wish_type NOT NULL,
  reason          TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(nurse_id, date)
);
CREATE INDEX idx_wishes_date ON wishes(date);

CREATE TABLE schedules (
  id              SERIAL PRIMARY KEY,
  year_month      CHAR(7) NOT NULL UNIQUE,           -- '2026-06'
  status          schedule_status NOT NULL DEFAULT 'draft',
  generated_at    TIMESTAMPTZ,
  confirmed_at    TIMESTAMPTZ,
  generation_log  JSONB                              -- §10 Infeasibility Policy
);

CREATE TABLE schedule_cells (
  id              SERIAL PRIMARY KEY,
  schedule_id     INT NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
  nurse_id        INT NOT NULL REFERENCES nurses(id),
  date            DATE NOT NULL,
  shift           shift_code NOT NULL,               -- G-05 (DE 포함)
  source          cell_source NOT NULL DEFAULT 'auto',
  modified_by_nurse_id INT REFERENCES nurses(id),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(schedule_id, nurse_id, date)
);
CREATE INDEX idx_cells_schedule_date ON schedule_cells(schedule_id, date);

CREATE TABLE swap_requests (
  id                   SERIAL PRIMARY KEY,
  schedule_id          INT NOT NULL REFERENCES schedules(id),
  requester_nurse_id   INT NOT NULL REFERENCES nurses(id),
  target_nurse_id      INT NOT NULL REFERENCES nurses(id),
  requester_date       DATE NOT NULL,
  target_date          DATE NOT NULL,
  status               swap_status NOT NULL DEFAULT 'pending',
  reason               TEXT,
  rejected_reason      TEXT,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (requester_nurse_id <> target_nurse_id)
);
CREATE INDEX idx_swaps_status ON swap_requests(status);

CREATE TABLE night_keeper_assignments (
  id                   SERIAL PRIMARY KEY,
  nurse_id             INT NOT NULL REFERENCES nurses(id),
  year_month           CHAR(7) NOT NULL,
  assigned_by_nurse_id INT REFERENCES nurses(id),
  reason               TEXT,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(nurse_id, year_month)
);
CREATE INDEX idx_nk_year_month ON night_keeper_assignments(year_month);

CREATE TABLE ward_settings (
  id                                   INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  min_d                                INT NOT NULL DEFAULT 3,
  min_e                                INT NOT NULL DEFAULT 2,
  min_n                                INT NOT NULL DEFAULT 2,
  max_consecutive_n                    INT NOT NULL DEFAULT 3,
  min_rest_after_n                     INT NOT NULL DEFAULT 1,
  max_consecutive_workdays             INT NOT NULL DEFAULT 5,
  balance_off_tolerance                INT NOT NULL DEFAULT 1,
  previous_month_lookback_days         INT NOT NULL DEFAULT 7,
  night_keeper_max_consecutive_months  INT NOT NULL DEFAULT 2,
  night_keeper_cooldown_months         INT NOT NULL DEFAULT 3,
  wish_unavailable_quota_monthly       INT,
  wish_preference_quota_monthly        INT,
  wish_deadline_days_before_month      INT NOT NULL DEFAULT 5,
  swap_deadline_days_before_date       INT NOT NULL DEFAULT 1,
  weight_balance_off                   INT NOT NULL DEFAULT 10,
  weight_respect_wishes                INT NOT NULL DEFAULT 8,
  weight_weekend_balance               INT NOT NULL DEFAULT 5,
  weight_same_shift_streak             INT NOT NULL DEFAULT 3,
  weight_short_rest_pattern            INT NOT NULL DEFAULT 4,
  updated_at                           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO ward_settings (id) VALUES (1);

-- 초기 등급 seed (head_nurse가 UI에서 자유 변경)
INSERT INTO experience_levels (code, display_name, min_months, max_months, min_d, min_e, min_n, sort_order) VALUES
  ('L1', '신입', 0, 12, 0, 0, 0, 1),
  ('L2', '주니어', 12, 36, 0, 0, 0, 2),
  ('L3', '중급', 36, 84, 1, 1, 1, 3),
  ('L4', '시니어', 84, NULL, 0, 0, 0, 4);
```

### 3.2 Entity Relationships

```
experience_levels  ──┐
                     ▼
                  nurses ──┬─── wishes (n)
                            ├─── schedule_cells (n)
                            ├─── swap_requests (requester/target)
                            └─── night_keeper_assignments (n)

schedules 1 ────── n schedule_cells
          1 ────── n swap_requests
```

### 3.3 등급 자동 분류 함수 (Go)

```go
// internal/nurses/level.go
// G-04: hire_date 기반 자동 분류 + override 우선
func ClassifyLevel(n *Nurse, levels []ExperienceLevel) string {
    if n.ExperienceLevelOverride != nil {
        return *n.ExperienceLevelOverride
    }
    if n.HireDate == nil {
        return levels[0].Code // 가장 낮은 등급 기본
    }
    months := monthsSince(*n.HireDate)
    for _, l := range sortBySort(levels) {
        if months >= l.MinMonths && (l.MaxMonths == nil || months < *l.MaxMonths) {
            return l.Code
        }
    }
    return levels[len(levels)-1].Code
}
```

---

## 4. API Specification

### 4.1 Endpoint List

> 모든 응답은 `{ data?, error?, meta? }` envelope. 보호 엔드포인트는 `Authorization: Bearer <jwt>`.

| Group | Method | Path | Description | Auth | Role |
|---|---|---|---|---|---|
| **auth** | GET | `/api/auth/oauth/google/start` | Google OAuth 인가 시작 (state·nonce·PKCE 발급) | - | - |
| | GET | `/api/auth/oauth/google/callback` | Google 콜백 → email 추출 → 화이트리스트 검증 → JWT 발급 | - | - |
| | POST | `/api/auth/refresh` | refresh 토큰으로 access 재발급 | cookie | - |
| | POST | `/api/auth/logout` | 세션 종료 | ✅ | any |
| | GET | `/api/auth/me` | 현재 사용자 | ✅ | any |
| **nurses** | GET | `/api/nurses` | 목록 (활성/비활성) | ✅ | any |
| | POST | `/api/nurses` | 추가 | ✅ | head_nurse |
| | PATCH | `/api/nurses/:id` | 수정 (name, role, hire_date, level_override, fixed_pattern, active) | ✅ | head_nurse |
| **levels** | GET | `/api/levels` | 등급 목록 | ✅ | any |
| | POST | `/api/levels` | 등급 추가 | ✅ | head_nurse |
| | PATCH | `/api/levels/:code` | 등급 수정 | ✅ | head_nurse |
| | DELETE | `/api/levels/:code` | 등급 삭제 (소속 nurse 0명이어야) | ✅ | head_nurse |
| **wishes** | GET | `/api/wishes?ym=YYYY-MM&nurse=ID?` | 본인은 본인 것, head는 전체 | ✅ | any |
| | PUT | `/api/wishes/:date` | upsert 본인 희망일 | ✅ | nurse (self) |
| | DELETE | `/api/wishes/:date` | 삭제 | ✅ | nurse (self) |
| **schedules** | POST | `/api/schedules` | 자동 생성 dispatch | ✅ | head_nurse |
| | GET | `/api/schedules?ym=YYYY-MM` | 조회 (status 폴링) | ✅ | any |
| | GET | `/api/schedules/:id/cells` | 셀 전체 | ✅ | any |
| | PATCH | `/api/schedules/:id/cells/:cellId` | 셀 수정 + 검증 | ✅ | head_nurse |
| | POST | `/api/schedules/:id/confirm` | 확정 (status='confirmed') | ✅ | head_nurse |
| **swaps** | GET | `/api/swaps?status=&target=me?` | 목록 | ✅ | any |
| | POST | `/api/swaps` | 요청 (A) | ✅ | nurse |
| | PATCH | `/api/swaps/:id` | accept/reject (B) / approve/reject (head) | ✅ | nurse/head |
| | POST | `/api/swaps/:id/cancel` | 요청자 취소 | ✅ | nurse (requester) |
| **night-keepers** | GET | `/api/night-keepers?ym=YYYY-MM` | 월별 목록 | ✅ | any |
| | POST | `/api/night-keepers` | 지정 (K-02·K-04·K-05 검증) | ✅ | head_nurse |
| | DELETE | `/api/night-keepers/:id` | 해제 | ✅ | head_nurse |
| **settings** | GET | `/api/settings` | ward_settings 조회 | ✅ | any |
| | PATCH | `/api/settings` | 수정 | ✅ | head_nurse |

### 4.2 핵심 엔드포인트 상세

#### `POST /api/schedules` — 자동 생성

**Request**:
```json
{ "year_month": "2026-06" }
```
**Response (202)**:
```json
{ "data": { "schedule_id": 12, "status": "generating", "estimated_seconds": 45 } }
```
**비동기 흐름**: goroutine이 솔버 호출 → 결과 cells UPSERT → status='generated'.

#### `PATCH /api/schedules/:id/cells/:cellId` — 셀 수정 + 검증

**Request**:
```json
{ "shift": "E" }
```
**Response (200)**:
```json
{
  "data": { "cell": { "id": 234, "shift": "E", "source": "manual" } },
  "meta": {
    "violations": [
      { "rule_id": "H-01", "severity": "hard", "message": "N 다음날 D 금지", "cell_ids": [234] },
      { "rule_id": "S-04", "severity": "soft", "message": "D 4일 연속", "cell_ids": [231,232,233,234] }
    ]
  }
}
```

#### `POST /api/night-keepers` — 나이트킵 지정

**Request**:
```json
{ "nurse_id": 7, "year_month": "2026-07", "reason": "본인 요청" }
```
**Validation** (사전 차단):
- K-02: 자신 포함 3달 연속 검사
- K-04: cooldown 3달 검사
- K-05: 동일 nurse의 fixed_shift_pattern IS NULL인지 검사

**Response 422 (위반)**:
```json
{ "error": { "code": "RULE_VIOLATION", "rule_id": "K-04", "message": "쿨다운 3달 필요 (M+5부터 가능)" } }
```

### 4.3 솔버 API (internal-only)

#### `POST http://ward-duty-solver:8000/generate`

**Request**:
```json
{
  "year_month": "2026-06",
  "nurses": [
    { "id": 1, "level": "L3", "fixed_pattern": null, "is_night_keeper": false },
    { "id": 2, "level": "L1", "fixed_pattern": "D_ONLY", "is_night_keeper": false }
  ],
  "wishes": [
    { "nurse_id": 1, "date": "2026-06-05", "type": "off" },
    { "nurse_id": 1, "date": "2026-06-10", "type": "unavailable" }
  ],
  "previous_month_last_week_cells": [
    { "nurse_id": 1, "date": "2026-05-25", "shift": "N" },
    { "nurse_id": 1, "date": "2026-05-26", "shift": "N" },
    { "nurse_id": 1, "date": "2026-05-27", "shift": "O" }
  ],
  "experience_levels": [
    { "code": "L3", "min_d": 1, "min_e": 1, "min_n": 1,
      "weight_coverage": 1, "weight_d_assignment": 0, "weight_e_assignment": 0, "weight_n_assignment": 0 }
  ],
  "ward_settings": {
    "min_d": 3, "min_e": 2, "min_n": 2,
    "max_consecutive_n": 3, "min_rest_after_n": 1,
    "max_consecutive_workdays": 5,
    "weight_balance_off": 10, "weight_respect_wishes": 8,
    "weight_weekend_balance": 5, "weight_same_shift_streak": 3,
    "weight_short_rest_pattern": 4
  },
  "max_time_seconds": 55
}
```

**Response 200**:
```json
{
  "status": "ok",
  "solver_status": "optimal",
  "objective_value": 142,
  "cells": [{ "nurse_id": 1, "date": "2026-06-01", "shift": "D" }, ...],
  "applied_rules": ["H-01", "H-02", "H-03", "H-04", "H-05", "H-06", "H-10", "H-11", "H-12", "K-01", "S-01", "S-02", "S-03", "S-04", "S-05", "S-10"],
  "elapsed_ms": 23410
}
```

**Response 422 (infeasible)** — §10 Infeasibility Policy:
```json
{
  "status": "infeasible",
  "solver_status": "infeasible",
  "violated_rule_ids": ["H-04", "H-12"],
  "suggestion": "L3 등급의 min_n을 1로 낮추거나 L3 시니어를 1명 추가하세요",
  "elapsed_ms": 8120
}
```

#### `POST http://ward-duty-solver:8000/validate`

**Request**: 동일한 입력 + `all_cells` 배열. **Response**: `violations[]`.

---

## 5. UI/UX Design

### 5.1 Page Map

```
/                       (auth 자동 리다이렉트)
/login                  로그인
/duty/[yyyymm]          ★ 핵심: 월별 듀티 그리드
/wishes/[yyyymm]        본인 희망일 캘린더
/nurses                 명단 (head_nurse 전용)
/levels                 등급 관리 (head_nurse 전용)
/settings               ward_settings + 가중치 (head_nurse 전용)
/night-keepers          나이트킵 지정 (head_nurse 전용)
/swaps                  swap 인박스/요청
```

### 5.2 User Flow (핵심 2개)

```
[수간호사 월말 워크플로우]
login → /duty/2026-06 → 'Generate' 클릭 → 폴링 → 그리드 표시
    → 빨간 셀(위반) 검토 → 드래그 수정 → 모두 클린
    → 'Confirm' 클릭 → status='confirmed'

[팀원 월중 워크플로우]
login → /wishes/2026-06 → 캘린더에서 날짜 클릭 → off/d/e/n/unavailable 선택
    → 자동 저장 (PUT /api/wishes/:date)
```

### 5.3 Component List (핵심)

| Component | Location | Responsibility |
|-----------|----------|----------------|
| `SchedulerGrid` | `web/features/duty/components/` | 30명 × 31일 그리드. 행=간호사, 열=날짜. 드래그 가능 |
| `CellTooltip` | `web/features/duty/components/` | 셀 hover 시 위반 룰 ID·메시지 표시 |
| `RuleViolationBadge` | `web/components/` | 셀에 빨간 테두리 + 룰 ID 뱃지 |
| `GenerationStatusBar` | `web/features/duty/components/` | "Generating... 12s 경과" / 실패 시 suggestion |
| `WishCalendar` | `web/features/wishes/components/` | 월 캘린더, 날짜 클릭 시 type 선택 모달 |
| `NurseTable` | `web/features/nurses/components/` | 명단 + 등급 자동 분류 표시 + override |
| `LevelEditor` | `web/features/levels/components/` | 등급 추가/삭제, min_d/min_e/min_n + weight |
| `NightKeeperPicker` | `web/features/night-keepers/components/` | 월별 nurse 선택, K-02/K-04/K-05 사전 검증 표시 |
| `SwapInbox` | `web/features/swaps/components/` | 들어온 swap 요청, accept/reject |
| `PdfExportButton` | `web/features/duty/components/` | html2canvas + jsPDF |

### 5.4 Page UI Checklist

#### `/duty/[yyyymm]` (수간호사 화면)
- [ ] Header: 월 선택 prev/next + 현재 status (`draft|generating|generated|confirmed|failed`) 뱃지
- [ ] Button: "Generate" (status가 draft 또는 failed일 때만 활성)
- [ ] Button: "Confirm" (status='generated'이고 hard 위반 0일 때만)
- [ ] Button: "Export PDF" / "Export PNG"
- [ ] Grid: 30~31열(날짜) × N행(간호사). 헤더에 요일·공휴일은 표시 안 함(G-06)
- [ ] Cell: 시프트 코드(D/E/N/O/DE) + 색상(D:파랑/E:주황/N:보라/O:회색/DE:빨강)
- [ ] Cell: 빨간 테두리(hard 위반) / 노란 테두리(soft 위반) / 자물쇠(고정 패턴) / 달(나이트킵 월)
- [ ] Sidebar: 위반 목록(룰 ID + 메시지 + 셀 클릭 시 grid 포커스)
- [ ] Loading: GenerationStatusBar (생성 중 경과 시간 + estimated)
- [ ] Failed: suggestion 텍스트 + "설정으로" 링크

#### `/wishes/[yyyymm]` (팀원 화면)
- [ ] Header: 월 선택 + 마감일 D-day 카운트다운
- [ ] Calendar: 월 그리드, 날짜 셀에 본인이 입력한 type 표시 (이모지: 🛌off/☀️d/🌆e/🌙n/🚫unavailable)
- [ ] Modal: 날짜 클릭 시 type 선택 5종 + reason 입력 (unavailable만 필수)
- [ ] Quota indicator: "이번 달 unavailable 3/5 사용" (W-02·W-03)
- [ ] Toast: 마감 후 unavailable은 head_nurse 승인 필요 안내 (W-05)

#### `/nurses` (head_nurse)
- [ ] Table: 이름, 이메일, 역할, 입사일, 자동 등급(자동/override), 고정 패턴, 활성
- [ ] Button: "Add" / row별 "Edit" / "Deactivate"
- [ ] Modal: 등급 override 선택(드롭다운: experience_levels.code) + 고정 패턴 선택

#### `/levels` (head_nurse)
- [ ] Table: code, 표시명, min_months, max_months, min_d/min_e/min_n, 가중치 4종, 정렬
- [ ] Button: "Add level" / row별 "Edit" / "Delete"
- [ ] Warning: min_d 합계 vs ward_settings.min_d 비교 경고 (보강 #4 — 단순 안내)
- [ ] Delete 차단: 소속 nurse > 0이면 비활성

#### `/settings` (head_nurse)
- [ ] Form: ward_settings 18개 필드 (그룹: 최소인원/연속/희망/swap/가중치)
- [ ] Save 버튼

#### `/night-keepers` (head_nurse)
- [ ] Calendar 월별: 각 nurse의 night_keeper 지정 현황 시각화 (12개월 표시)
- [ ] Button: "Assign" → modal (nurse 선택, 월 선택)
- [ ] Validation: K-02/K-04/K-05 위반 시 모달 안에서 빨간 메시지
- [ ] Indicator: 표준 패턴(K-03 연속 2달) 벗어나는 지정은 노란 경고

#### `/swaps`
- [ ] Tab: 받은 요청(nurse 시점), 보낸 요청(nurse), 승인 대기(head_nurse), 완료
- [ ] Card: requester ↔ target, requester_date ↔ target_date, status, action 버튼
- [ ] Modal: head_nurse approve 시 검증 결과 미리보기(violations[])

---

## 6. Error Handling

### 6.1 Error Code

| HTTP | Code | 발생 | 처리 |
|---|---|---|---|
| 400 | `VALIDATION_ERROR` | request body 검증 실패 | UI에서 fieldErrors 표시 |
| 401 | `UNAUTHORIZED` | access 만료 | refresh → 재시도, 실패 시 /login |
| 403 | `FORBIDDEN` | RBAC 위반 | UI 토스트 "권한 없음" |
| 404 | `NOT_FOUND` | 리소스 없음 | UI 빈 상태 |
| 409 | `CONFLICT` | unique 위반 (예: 동월 schedule 중복) | UI 안내 |
| 422 | `RULE_VIOLATION` | 룰 위반 (K-02·K-04·K-05·X-03 등) | rule_id 명시, UI 메시지 표시 |
| 500 | `INTERNAL` | 예기치 못한 서버 오류 | request_id 로깅 + 사용자 안내 |
| 503 | `SOLVER_UNAVAILABLE` | 솔버 컨테이너 다운 | 5분 후 재시도 안내 |

### 6.2 Error Response Format

```json
{
  "error": {
    "code": "RULE_VIOLATION",
    "rule_id": "K-04",
    "message": "쿨다운 3달 필요 (M+5부터 가능)",
    "details": { "nurse_id": 7, "target_ym": "2026-09", "last_consecutive_end": "2026-07" },
    "request_id": "req_8x2k"
  }
}
```

### 6.3 솔버 Infeasible (§10)

`schedules.status='failed'`, `generation_log`:
```json
{
  "solver_status": "infeasible",
  "violated_rule_ids": ["H-04"],
  "suggestion": "min_n을 1로 낮추세요",
  "input_summary": { "nurses": 28, "wishes_unavailable": 12 }
}
```

---

## 7. Security Considerations

- [x] **Google OAuth + 화이트리스트** (FR-10) — 비밀번호 미저장, 사전 등록 이메일만 통과
- [x] **OAuth 보안** — `state`/`nonce`/PKCE 모두 사용, `OAUTH_STATE_SECRET`으로 HMAC 서명, 10분 만료
- [x] **JWT 짧은 access (15m) + Redis refresh (7d, httpOnly+Secure+SameSite=Lax 쿠키)**
- [x] **RBAC 미들웨어** — head_nurse 전용 엔드포인트는 `requireRole("head_nurse")`
- [x] **Input validation** — `go-playground/validator` (Go) + Pydantic (solver)
- [x] **SQL injection** — pgx parameterized queries만 사용, `fmt.Sprintf` 금지
- [x] **솔버 외부 노출 금지** — Traefik labels 없음, `ward-duty-internal` 네트워크만, internal 인증 헤더(`X-Internal-Token`)
- [x] **CSRF** — refresh 쿠키 SameSite=Lax + state-changing 요청은 JWT bearer 헤더(쿠키 아님)
- [x] **HTTPS 강제** — Cloudflare Tunnel 자동
- [x] **Rate limiting** — Traefik middleware (IP당 100 req/min) — 로그인은 5 req/min
- [x] **시크릿 관리** — `.env`만, git ignore, 백업은 별도 암호화
- [x] **로깅** — 비밀번호·토큰·이메일 마스킹 (`yan***@cocone.co.jp`)

---

## 8. Test Plan

### 8.1 Test Scope

| Type | Target | Tool | Phase |
|---|---|---|---|
| **L0: Solver Unit** | 룰 ID별 솔버 함수 (`H_01_no_n_to_d` 등) | pytest | Do |
| **L0: Solver Regression** | 대표 케이스 30명/31일 5종 → KPI 80% 검증 | pytest | Do |
| L1: API Tests | Go API endpoints | testify + httptest | Do |
| L2: UI Action Tests | 페이지 인터랙션 | Playwright | Do |
| L3: E2E Scenario | 다중 페이지 흐름 | Playwright | Do |

### 8.2 L0 — Solver Unit Tests (룰 ID 1:1)

| # | Test | Rule | Expected |
|---|---|---|---|
| 1 | `test_h01_no_n_to_d` | H-01 | N(t) → D(t+1) 0건 |
| 2 | `test_h02_max_consecutive_n` | H-02 | N 연속 > max_consecutive_n 0건 |
| 3 | `test_h03_max_workdays` | H-03 | 연속 근무 > 5일 0건 |
| 4 | `test_h04_shift_min` | H-04 | 각 t·s에 ≥ min_s |
| 5 | `test_h05_unavailable` | H-05 | unavailable 셀에 어떤 시프트도 X |
| 6 | `test_h06_rest_after_n` | H-06 | N(t) → O(t+1) 보장 |
| 7 | `test_h10_month_boundary` | H-10 | 이전 달 N(31)→D(1) 위반 catch |
| 8 | `test_h11_fixed_pattern` | H-11 | D_ONLY는 D/O만 |
| 9 | `test_h12_level_min` | H-12 | 등급별 min_s 보장 |
| 10 | `test_h13_no_de_in_auto` | H-13 | 자동 생성 결과에 DE 0건 |
| 11 | `test_h14_de_then_no_d` | H-14 | DE(t) → D(t+1) 0건 |
| 12 | `test_k01_night_keeper_only_no` | K-01 | nk는 N/O만 |
| 13 | `test_s01_off_balance_minimized` | S-01 | Off 편차 ≤ tolerance + 가중치 효과 |
| 14 | `test_s02_wishes_respected` | S-02 | 희망 반영률 ≥ 70% |
| 15 | `test_validate_returns_violations` | validate | 위반 셀 정확히 식별 |

### 8.3 L0 — Regression Set (KPI 검증)

`solver/tests/fixtures/`에 5가지 시드:

| Fixture | 인원 | 특이점 | KPI 기대 |
|---|:-:|---|:-:|
| `simple_30_jun` | 30 | 평범한 6월 (희망 5건) | optimal ≤30s |
| `tight_15_jul` | 15 | 인원 적음, 희망 많음 | feasible ≤60s |
| `night_keepers_25_aug` | 25 | nk 2명 | optimal ≤45s |
| `fixed_patterns_20_sep` | 20 | D_ONLY 2명, WEEKDAY_E 1명 | optimal ≤45s |
| `infeasible_oct` | 10 | 의도적 infeasible | violated_rule_ids 정확 |

### 8.4 L1 — API Tests (대표)

| # | Endpoint | Test | Expected |
|---|---|---|---|
| 1 | POST /api/auth/login | 정상 | 200 + access + refresh 쿠키 |
| 2 | POST /api/auth/login | 잘못된 pw | 401 |
| 3 | GET /api/nurses | 비로그인 | 401 |
| 4 | POST /api/nurses | nurse 권한 | 403 |
| 5 | POST /api/schedules | head_nurse | 202 + schedule_id |
| 6 | POST /api/night-keepers | K-04 위반 케이스 | 422 + rule_id=K-04 |
| 7 | PATCH /api/schedules/:id/cells/:cid | hard 위반 발생 셀 | 200 + meta.violations |

### 8.5 L2/L3 — UI/E2E (대표)

| # | Scenario | Steps | Success |
|---|---|---|---|
| 1 | 듀티 자동 생성 | login(head) → /duty/2026-06 → Generate → 폴링 → grid 렌더 | hard 위반 0 셀 표시 |
| 2 | 셀 수동 수정 | grid에서 N→D 드래그 | 빨간 테두리 + H-01 메시지 |
| 3 | 희망일 입력 | login(nurse) → /wishes → 날짜 클릭 → unavailable | PUT 200 + 캘린더 갱신 |
| 4 | 나이트킵 K-04 차단 | 6,7월 지정 후 8월 시도 | 모달 빨간 메시지 |
| 5 | swap 승인 | A 요청 → B 수락 → head approve | DB에서 cells 교환 확인 |

### 8.6 Seed Data Requirements

| Entity | Min | 비고 |
|---|:-:|---|
| experience_levels | 4 (L1~L4) | 시드 자동 |
| nurses | 30 | 다양한 입사일 + 1명 head_nurse |
| wishes | 30 | 희망 시나리오 |
| schedule_cells (전월) | 30×7 | H-10 입력용 |
| ward_settings | 1 | id=1 시드 |

---

## 9. Clean Architecture — Vertical Slices 매핑

### 9.1 Go API 디렉토리 구조 (Vertical Slices)

```
api/
├── cmd/server/main.go              # router 조립, DI
├── internal/
│   ├── auth/                       # 인증 슬라이스
│   │   ├── handler.go              # POST /login, /refresh, /logout, GET /me
│   │   ├── service.go              # bcrypt, JWT 발급/검증
│   │   ├── middleware.go           # RequireAuth, RequireRole
│   │   └── model.go
│   ├── nurses/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repo.go
│   │   ├── level.go                # G-04 ClassifyLevel
│   │   └── model.go
│   ├── levels/                     # experience_levels
│   ├── wishes/
│   ├── schedules/
│   │   ├── handler.go
│   │   ├── service.go              # generate dispatch, cells PATCH, confirm
│   │   ├── repo.go
│   │   └── model.go
│   ├── swaps/
│   │   ├── handler.go
│   │   ├── service.go              # 상태머신, X-03 검증
│   │   ├── repo.go
│   │   └── model.go
│   ├── nightkeepers/
│   │   ├── handler.go
│   │   ├── service.go              # K-02/K-04/K-05 사전 검증
│   │   └── repo.go
│   ├── settings/
│   ├── solver/                     # 솔버 HTTP 클라이언트
│   │   ├── client.go               # Generate(), Validate()
│   │   └── schemas.go              # solver와 동일한 JSON 계약
│   └── db/
│       ├── conn.go                 # pgx pool
│       └── goose/                  # migrations 디렉토리 마운트
├── migrations/
│   └── 0001_init.sql               # §3.1
├── pkg/
│   └── apierr/                     # error envelope
├── go.mod
└── Dockerfile
```

### 9.2 Python Solver 디렉토리 구조

```
solver/
├── app/
│   ├── main.py                     # FastAPI 앱 시작
│   ├── routes.py                   # POST /generate, /validate
│   ├── schemas.py                  # Pydantic (Go solver/schemas.go와 1:1)
│   ├── model.py                    # ★ 순수 CP-SAT 모델 빌더
│   │   - build_model(input) -> cp_model.CpModel
│   │   - 룰 함수: H_01_no_n_to_d, H_02_max_consecutive_n, ...
│   ├── validator.py                # cells + context → violations[]
│   ├── classify.py                 # 등급 분류 (Python 측)
│   └── infeasibility.py            # §10 정책: unsat core → suggestion
├── tests/
│   ├── conftest.py
│   ├── fixtures/                   # L0 regression 5종
│   ├── test_h_rules.py             # H-01 ~ H-14
│   ├── test_k_rules.py             # K-01
│   ├── test_s_rules.py             # S-01 ~ S-05, S-10
│   └── test_regression.py          # L0 KPI 검증
├── requirements.txt                # fastapi, uvicorn, ortools, pydantic
└── Dockerfile
```

### 9.3 Next.js 디렉토리 구조

```
web/
├── app/
│   ├── (auth)/login/page.tsx
│   ├── (app)/
│   │   ├── layout.tsx              # 인증 가드 + 네비
│   │   ├── duty/[yyyymm]/page.tsx
│   │   ├── wishes/[yyyymm]/page.tsx
│   │   ├── nurses/page.tsx
│   │   ├── levels/page.tsx
│   │   ├── settings/page.tsx
│   │   ├── night-keepers/page.tsx
│   │   └── swaps/page.tsx
│   └── layout.tsx
├── features/
│   ├── auth/                       # useAuth, AuthProvider
│   ├── duty/                       # SchedulerGrid, CellTooltip, GenerationStatusBar, PdfExportButton
│   ├── wishes/                     # WishCalendar
│   ├── nurses/                     # NurseTable
│   ├── levels/                     # LevelEditor
│   ├── settings/                   # SettingsForm
│   ├── night-keepers/              # NightKeeperPicker
│   └── swaps/                      # SwapInbox
├── components/                     # 공통 UI (Button, Badge, Modal, Toast)
├── lib/
│   ├── api.ts                      # JWT 자동 첨부, refresh
│   ├── violations.ts               # rule_id → 한국어 메시지 매핑
│   └── shifts.ts                   # 시프트 색상·이모지
└── Dockerfile
```

### 9.4 Dependency Rules

| From | Can Import | Cannot |
|---|---|---|
| `internal/<slice>/handler.go` | service, model, apierr | 다른 slice handler |
| `internal/<slice>/service.go` | repo, model, solver, db | 다른 slice service (해당 slice의 repo·model만) |
| `internal/<slice>/repo.go` | model, db | service, handler |
| `solver/app/routes.py` | model, validator, schemas | 외부 I/O |
| `solver/app/model.py` | OR-Tools, schemas (순수) | I/O, DB, FastAPI |
| `web/features/*/components/` | features/<self>/hooks, lib, components | 다른 feature 직접 |

### 9.5 룰 ID 매핑 (코드 위치)

| Rule | 솔버 함수 (`solver/app/model.py`) | API 검증 (Go) | UI 메시지 (`web/lib/violations.ts`) |
|---|---|---|---|
| H-01 | `H_01_no_n_to_d(model, x)` | — | "N 다음날 D 금지" |
| H-02 | `H_02_max_consecutive_n(...)` | — | "N 연속 한도 초과" |
| H-03 | `H_03_max_workdays(...)` | — | "연속 근무 5일 초과" |
| H-04 | `H_04_shift_min(...)` | — | "시프트 최소 인원 미달" |
| H-05 | `H_05_unavailable(...)` | — | "불가일에 배정됨" |
| H-06 | `H_06_rest_after_n(...)` | — | "N 후 휴식 부족" |
| H-10 | `H_10_load_prev_month(...)` | `solver.Client.BuildInput()` | — |
| H-11 | `H_11_fixed_pattern(...)` | `nurses.service` | "고정 패턴 위반" |
| H-12 | `H_12_level_per_shift(...)` | — | "등급 {L} 최소 인원 미달" |
| H-13 | `H_13_exclude_de_auto(...)` | — | — |
| H-14 | `H_14_de_then_no_d(...)` | — | "DE 다음날 D 금지" |
| K-01 | `K_01_nk_only_n_or_o(...)` | — | "나이트킵 충돌" |
| K-02 | — | `nightkeepers.ValidateK02()` | "3달 연속 금지" |
| K-04 | — | `nightkeepers.ValidateK04()` | "cooldown 3달 필요" |
| K-05 | — | `nightkeepers.ValidateK05()` | "고정 패턴과 충돌" |
| S-01 | `S_01_balance_off(model, x)` | — | "Off 균형 가중치 적용" |
| S-02 | `S_02_respect_wishes(...)` | — | — |
| S-03 | `S_03_weekend_balance(...)` | — | — |
| S-04 | `S_04_same_shift_streak(...)` | — | — |
| S-05 | `S_05_short_rest(...)` | — | — |
| S-10 | `S_10_level_assignment_cost(...)` | — | — |
| X-03 | — | `swaps.ApproveTx()` (solver.Validate 호출) | "swap 검증 실패" |
| G-04 | `classify.classify_level()` | `nurses.ClassifyLevel()` | — |

---

## 10. Coding Convention

### 10.1 Naming

| Target | Rule | Example |
|---|---|---|
| Go 패키지 | lowercase, 단수 | `schedules`, `nurses` |
| Go 파일 | snake_case | `handler.go`, `level.go` |
| Go 함수 (export) | PascalCase | `ClassifyLevel`, `BuildInput` |
| **Go 룰 함수** | PascalCase + 룰 ID | `ValidateK02`, `ValidateK04` |
| **Python 룰 함수** | snake_case + 룰 ID | `H_01_no_n_to_d`, `K_01_nk_only_n_or_o` |
| Python 모듈 | snake_case | `model.py`, `validator.py` |
| React 컴포넌트 | PascalCase | `SchedulerGrid`, `CellTooltip` |
| Next.js 폴더 | kebab-case | `night-keepers/`, `wishes/` |
| 환경변수 | UPPER_SNAKE | `DB_PASSWORD`, `JWT_SECRET` |

### 10.2 Go Import Order

```go
import (
    // 1) standard
    "context"
    "encoding/json"
    // 2) external
    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    // 3) internal
    "ward-duty-api/internal/auth"
    "ward-duty-api/internal/nurses"
    // 4) pkg
    "ward-duty-api/pkg/apierr"
)
```

### 10.3 Python Style

- `ruff` + `black` 적용
- 타입 힌트 필수 (`from __future__ import annotations` + Pydantic v2)
- 룰 함수는 `(model, x, ctx) -> None` 시그니처 통일

### 10.4 Environment Variables

| Prefix | 용도 | Example |
|---|---|---|
| `DB_` | PostgreSQL | `DB_HOST=shared-postgres`, `DB_NAME=ward_duty` |
| `REDIS_` | Redis | `REDIS_HOST=ward-duty-redis:6379` |
| `JWT_` | 인증 | `JWT_SECRET`, `JWT_ACCESS_TTL=15m` |
| `SOLVER_` | 솔버 | `SOLVER_URL=http://ward-duty-solver:8000`, `SOLVER_INTERNAL_TOKEN` |
| `NEXT_PUBLIC_` | 클라이언트 | `NEXT_PUBLIC_API_URL=/api` (단일 도메인이라 상대 경로) |

---

## 11. Implementation Guide

### 11.1 Top-level File Structure

```
ward-duty/                       # repo root (=  ~/projects/ward-duty)
├── README.md
├── .gitignore
├── docker-compose.yml           # Traefik 라벨, networks(proxy, shared, ward-duty-internal)
├── .env.example
├── docs/                        # (이미 존재)
│   ├── 01-plan/
│   ├── 02-design/
│   └── rules/
├── web/                         # Next.js
├── api/                         # Go
└── solver/                      # Python FastAPI
```

### 11.2 Implementation Order (논리적)

1. infra-foundation (docker-compose, networks, traefik labels, .env.example, README)
2. DB migration `0001_init.sql` (shared-postgres에 DB 생성 + 마이그)
3. solver-core (model.py + 룰 함수 + 단위 테스트 — KPI 검증 1순위)
4. solver-api (FastAPI routes + schemas + Dockerfile)
5. api-core (auth + RBAC 미들웨어 + nurses/levels CRUD)
6. api-domain (wishes, settings, night-keepers, schedules, swaps)
7. web-foundation (Next.js scaffold + auth + lib/api + layout)
8. web-duty-grid (SchedulerGrid + 자동 생성 + 셀 수정 + PDF)
9. web-management (nurses, levels, settings, night-keepers, wishes, swaps)
10. integration (Cloudflare Tunnel 서브도메인 + E2E)

### 11.3 Session Guide

#### Module Map

| Module | Scope Key | Description | Estimated Turns |
|---|---|---|:-:|
| 인프라 + DB 마이그 | `module-1` | docker-compose, traefik, .env, 0001_init.sql 적용 | 20-25 |
| 솔버 코어 (CP-SAT + 테스트) | `module-2` | model.py 룰 함수 16종 + L0 unit + regression 5종 | 50-60 |
| 솔버 API + 통합 | `module-3` | FastAPI routes, schemas, Dockerfile, Go client | 25-30 |
| API 인증 + 명단/등급 | `module-4` | auth(JWT+RBAC), nurses, levels CRUD | 40-50 |
| API 도메인 | `module-5` | wishes, settings, night-keepers(K-02/04/05), schedules(비동기), swaps(상태머신) | 50-60 |
| Web 기반 | `module-6` | Next.js scaffold, auth flow, lib/api, 공통 컴포넌트 | 30-40 |
| Web 듀티 그리드 | `module-7` | SchedulerGrid, 자동 생성 폴링, 셀 수정, PDF | 50-60 |
| Web 관리 화면 | `module-8` | nurses/levels/settings/night-keepers/wishes/swaps 페이지 | 40-50 |
| 통합 + 배포 | `module-9` | Cloudflare Tunnel 등록, E2E 테스트, README 운영 메모 | 20-30 |

#### Recommended Session Plan

| Session | Phase | Scope | Turns |
|---|---|---|:-:|
| Session 1 | Plan + Design (현재) | 전체 | ~60 (사용 중) |
| Session 2 | Do | `--scope module-1,module-2` | 70-85 |
| Session 3 | Do | `--scope module-3,module-4` | 65-80 |
| Session 4 | Do | `--scope module-5` | 50-60 |
| Session 5 | Do | `--scope module-6,module-7` | 80-100 |
| Session 6 | Do | `--scope module-8,module-9` | 60-80 |
| Session 7 | Check + Report | 전체 | 40-50 |

---

## 12. Open Items → Design 시점에서 보강 필요한 것

> 룰 v0.4 §9의 잔여 Open Questions 중 design 진입에 영향 없는 항목만 표시. 모두 design phase 중 솔버 PoC·실제 시드 데이터로 자연스럽게 확정 가능.

- H-15 (DE 연속 / 월 횟수) — 후보 hard, 운영 데이터 누적 후
- W-02 / W-03 (희망 횟수 제한) — UI에 quota indicator 자리만 두고 값은 운영 1개월 후
- W-04 마감일 — `ward_settings.wish_deadline_days_before_month` 기본 5로 시작
- X-04 swap 시한 — `swap_deadline_days_before_date` 기본 1
- R-01~R-03 (월 중 재생성) — v2 검토

---

## Version History

| Version | Date | Changes | Author |
|---|---|---|---|
| 0.1 | 2026-05-24 | Initial design — Option C (Pragmatic Vertical Slices) 채택. Plan v0.3 + Rules v0.4 기반. Go vertical slice + Python pure CP-SAT + Next.js feature-folder. 룰 ID 25개 코드 매핑 표 작성. Session Guide 9개 모듈로 분할. | yang-donggwui |
