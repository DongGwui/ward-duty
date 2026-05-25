---
doc: scheduling-rules
project: ward-duty
version: 0.3
status: draft
last_updated: 2026-05-24
maintained_by: yang-donggwui
related:
  - ../01-plan/features/ward-duty-mvp.plan.md
  - ../01-plan/research/interview-cheatsheet.md
---

# Scheduling Rules — 간호사 듀티 스케줄링 규칙

> **살아있는 문서** — 청취·운영 회고·인시던트로 계속 진화.
> 모든 룰에는 ID가 부여되며 코드(solver 모델·테스트·UI 위반 메시지)에서 역참조한다.

---

## 0. 룰 ID 컨벤션

| 접두 | 영역 | 시행 방식 |
|---|---|---|
| `G-NN` | **General** (시프트 정의, 등급 정의) | 입력 형태·enum 등에 영향 |
| `H-NN` | **Hard constraint** | 위반 시 솔버가 해를 반환하지 않음. 확정 차단. |
| `S-NN` | **Soft constraint** | 솔버 목적함수의 가중치 항. 위반 가능하나 최소화. |
| `W-NN` | **Wish (희망일)** 운영 룰 | UI/API 제약. 사용자 행동을 가이드. |
| `X-NN` | **Swap (교환)** 운영 룰 | swap 상태머신·검증 로직에 반영. |
| `K-NN` | **Night-Keeper (나이트킵)** 룰 | 월 단위 지정·이력 관리. |
| `R-NN` | **Re-generation** (월 중 재생성) | 휴직·신규 입사 대응 |

### 룰 표 컬럼 정의

- **Status**: `pending` · `confirmed` · `deprecated`
- **Source**: `inferred` · `interview` · `paper` · `incident`
- **Added in**: 룰 문서 버전 (v0.1, v0.2, v0.3, ...)

---

## 1. 시프트 정의 & 연차 등급 정의 (G-NN)

| ID | Status | Source | Added | 룰 | 비고 |
|---|---|---|---|---|---|
| G-01 | **confirmed** | interview | v0.1→v0.3 | **시프트 enum: `D, E, N, O, DE`** — 시간대 확정 (§7) | v0.3에서 DE 추가 |
| G-02 | confirmed | interview | v0.1 | 1인 1일 1시프트 (overlap 불가) | hard 함의 |
| G-03 | pending | inferred | v0.1 | 추가 코드(`Edu`, `V`, `S`, `H`) 운영 여부 | 청취 필요 |
| **G-04** | **confirmed** | **interview** | **v0.2→v0.5** | **연차 등급 시스템 — head_nurse 직접 부여**. `experience_levels` 테이블에서 등급을 정의(code/display_name/min_d/min_e/min_n/weights). 각 간호사의 등급은 명단 페이지에서 `experience_level_override`로 **직접 지정**. v0.5에서 hire_date 자동 분류는 제거 — 운영 유연성·임상 직책 반영 어려움. `hire_date`와 `min_months`/`max_months`는 참고 정보로만 보존. 미지정 nurse는 sort_order가 가장 낮은 등급으로 fallback. | 신규 테이블 §8 참조 |
| **G-05** | **confirmed** | **interview** | **v0.3→v0.4** | **`DE` 더블 시프트** — D+E 합쳐 07:00~22:00 (15시간). 부득이한 인력 부족 시에만 사용. 솔버 자동 배정 X (H-13 참조). **DE 배정자는 H-04 / H-12의 D 및 E 카운트에 동시 포함**. | 수동 배정 전용 |
| **G-06** | **confirmed** | **interview** | **v0.4** | **공휴일 별도 처리 없음** — 병동 간호사 근무 형태상 공휴일은 평일과 동일 취급. WEEKDAY_*/WEEKDAY_E 패턴은 토·일만 Off로 간주. `treat_holiday_as_weekend` 같은 설정 키 도입하지 않음. | 솔버·UI 단순화 |

> **청취 필요(잔여)**:
> - G-03: 추가 코드(`Edu`/`V`/`S`/`H`) 운영 여부

---

## 2. Hard Constraints (절대 위반 금지)

| ID | Status | Source | Added | 룰 | 솔버 표현 (예정) |
|---|---|---|---|---|---|
| H-01 | pending | inferred | v0.1 | **N → D 금지** | `x[i,t,N]=1 ⇒ x[i,t+1,D]=0` |
| H-02 | pending | inferred | v0.1 | **N 연속 최대 N_max일** | sliding window |
| **H-03** | **confirmed** | **interview** | v0.1→v0.3 | **연속 근무 최대 5일** (`max_consecutive_workdays=5`) | sliding window |
| H-04 | pending | inferred | v0.1 | **시프트별 일일 최소 인원(전체)** D/E/N | `Σ_i x[i,t,s] ≥ ward_settings.min_s` |
| H-05 | pending | inferred | v0.1 | **`unavailable` 희망일에 어떤 시프트도 배정 불가** | `x[i,t,*]=0` for blocked |
| **H-06** | **confirmed** | **interview** | v0.1→v0.3 | **N 시프트 직후 Off 최소 1일** (`min_rest_after_n=1`). N을 연속으로 한 경우도 마지막 N 다음날 Off 1일 이상 보장. | `x[i,t,N]=1 ⇒ x[i,t+1,O]=1` (sliding) |
| H-10 | confirmed | interview | v0.2 | **월 경계 연속성** — 이전 달 마지막 7일 cells를 솔버 입력에 상수로 포함 (H-01/H-02/H-03/H-06 검증이 월 경계를 넘음) | `previous_month_last_week_cells[]` |
| H-11 | confirmed | interview | v0.2 | **고정 시프트 배정** — `nurses.fixed_shift_pattern` 설정 시 패턴대로만 | 결정변수 사전 고정 |
| **H-12** | **confirmed** | **interview** | v0.2→v0.3 | **등급별 시프트당 최소 인원** — 각 날짜 t, 시프트 s∈{D,E,N}, 각 등급 L에 대해 배정 인원 ≥ `experience_levels[L].min_s`. (단일 임계값 모델 폐기, 다단계 등급으로 일반화) | `Σ_{i∈L} x[i,t,s] ≥ L.min_s` for all L, s, t |
| **H-13** | **confirmed** | **interview** | **v0.3** | **`DE` 시프트는 솔버 자동 생성 불가** — 수동 수정에서만 head_nurse가 배정. 자동 생성 결과에 `DE`가 포함되면 안 됨. | 결정변수 도메인에서 DE 제외 (자동 생성 모드) |
| **H-14** | **confirmed** | **interview** | **v0.4** | **DE 직후 D 금지** — DE(15h 근무) 다음날 D 시프트 배정 불가. E/N/O는 가능 (H-01과 같은 패턴, Off 강제 아님). | `x[i,t,DE]=1 ⇒ x[i,t+1,D]=0` |

### Hard 후보 (검토 중)

| ID | Status | 가설 룰 |
|---|---|---|
| H-07? | pending | E → D (저녁 → 다음날 아침) 금지? |
| H-08? | pending | 신규(입사 3개월 이내) 단독 N 금지? |
| H-09? | pending | 같은 날 더블 시프트 함의 패턴 금지? (DE 도입으로 일부 흡수됨) |
| ~~H-14?~~ | ~~pending~~ | **v0.4에서 H-14로 승격: DE 직후 D 금지 (Off 강제는 X)** |
| H-15? | pending | `DE` 연속 금지 / 월 최대 횟수? |

---

## 3. Soft Constraints (가중치 기반 최소화)

| ID | Status | Source | Added | 룰 | 초기 가중치 |
|---|---|---|---|---|---|
| S-01 | pending | inferred | v0.1 | **월 Off 수 균형** — ±`balance_off_tolerance`일 이내 | 10 |
| S-02 | pending | inferred | v0.1 | **희망일 반영** — `wishes.type ∈ {off,d,e,n}` 최대한 충족 | 8 |
| S-03 | pending | inferred | v0.1 | **주말(토·일) 균형** | 5 |
| S-04 | pending | inferred | v0.1 | **같은 시프트 연속 회피** (D/E) | 3 |
| S-05 | pending | inferred | v0.1 | **N→Off→D 짧은 휴식 패턴 회피** | 4 |
| S-09 | deprecated | interview | v0.1→v0.2 | ~~시프트별 시니어 분산~~ → H-12 (등급별) 로 일반화 | — |
| **S-10** | **confirmed** | **interview** | **v0.3** | **등급별 시프트 배정 가중치** — `experience_levels[L].weight_coverage`. 등급 L이 시프트 s에 배정될 때의 선호도/페널티. head_nurse가 자유롭게 설정 가능. | 가변 |

### Soft 후보

| ID | Status | 가설 룰 |
|---|---|---|
| S-06? | pending | 페어링 — 멘토-멘티 같은 시프트 |
| S-07? | pending | 안티페어링 — 특정 두 명 분리 |
| S-08? | pending | 공휴일 균형 (S-03과 별개) |

---

## 4. 희망일 룰 (W-NN)

| ID | Status | Source | Added | 룰 |
|---|---|---|---|---|
| W-01 | pending | inferred | v0.1 | `wishes.type` enum: `off, d, e, n, unavailable` |
| W-02 | pending | inferred | v0.1 | 월 `unavailable` 횟수 제한 (값 미정) |
| W-03 | pending | inferred | v0.1 | 월 `off/d/e/n` 선호 횟수 제한 (값 미정) |
| W-04 | pending | inferred | v0.1 | 희망일 입력 마감일 (전월 D-day) |
| W-05 | pending | inferred | v0.1 | 마감 후 `unavailable` 추가는 head_nurse 승인 |
| W-06 | pending | inferred | v0.1 | 희망 충돌 시 우선순위 결정 방식 |

---

## 5. 교환(swap) 룰 (X-NN)

| ID | Status | Source | Added | 룰 |
|---|---|---|---|---|
| X-01 | pending | inferred | v0.1 | 상태머신 `pending → b_accepted → approved` |
| X-02 | confirmed | interview | v0.1 | 같은 `schedule_id` 내에서만 swap |
| X-03 | confirmed | interview | v0.1→v0.4 | swap 결과 hard 위반 시 승인 거부 (자동 검증). **검증 대상 hard 룰: H-01, H-02, H-03, H-06, H-10, H-11, H-12, H-14, K-01** (head_nurse 승인 시점에 solver `/validate` 전체 통과해야 함) |
| X-04 | pending | inferred | v0.1 | swap 가능 시한 (값 미정) |
| X-05 | pending | inferred | v0.1 | head_nurse 강제 변경 경로 운영 여부 |

---

## 6. Night-Keeper 룰 (K-NN)

| ID | Status | Source | Added | 룰 | 시행 위치 |
|---|---|---|---|---|---|
| K-01 | confirmed | interview | v0.2 | 나이트킵 지정 시 N/O만 배정 | 솔버 hard 제약 |
| K-02 | confirmed | interview | v0.2 | 연속 3달 이상 금지 | API/UI validation |
| **K-03** | **confirmed** | **interview** | v0.2→v0.3 | **표준 패턴: 연속 2달**. 부득이한 경우 추가/이동 허용(보통 고연차 대상). UI에서 head_nurse가 권장 패턴을 어기는 지정 시 확인 다이얼로그 표시(차단은 X — soft) | UI 안내 + 통계 |
| **K-04** | **confirmed** | **interview** | v0.2→v0.3 | **2달 연속 후 cooldown ≥ 3달** (`night_keeper_cooldown_months=3`). 즉 (M, M+1) 연속 지정 후 M+2/M+3/M+4 동일인 지정 불가, M+5부터 가능. | API/UI validation |
| **K-05** | **confirmed** | **interview** | **v0.4** | **`fixed_shift_pattern` ∧ `night_keeper_assignment` 동시 지정 불가** — D_ONLY 등 고정 패턴이 비-NULL인 간호사를 같은 달의 night_keeper로 지정 시 API/UI에서 차단. (모순으로 인한 솔버 infeasible 사전 방지) | API/UI validation |

### Night-Keeper 데이터 모델

```sql
CREATE TABLE night_keeper_assignments (
  id SERIAL PRIMARY KEY,
  nurse_id INT NOT NULL REFERENCES nurses(id),
  year_month CHAR(7) NOT NULL,                  -- '2026-06'
  assigned_by_nurse_id INT REFERENCES nurses(id),
  reason TEXT,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  UNIQUE(nurse_id, year_month)
);
CREATE INDEX idx_nk_year_month ON night_keeper_assignments(year_month);
```

### K-02 / K-04 검증 의사코드 (cooldown=3 적용)

```go
// 신규 지정 (nurse_id, target_ym) 검증
prev1 := target_ym.MinusMonths(1)
prev2 := target_ym.MinusMonths(2)
prev3 := target_ym.MinusMonths(3)
prev4 := target_ym.MinusMonths(4)
next1 := target_ym.PlusMonths(1)
next2 := target_ym.PlusMonths(2)

has := func(ym string) bool { /* SELECT EXISTS */ }

// K-02: 3달 연속 금지 (자기 자신 포함)
if has(prev2) && has(prev1) { return ErrK02 }
if has(prev1) && has(next1) { return ErrK02 }
if has(next1) && has(next2) { return ErrK02 }

// K-04: (M-2, M-1) 연속이면 M, M+1, M+2 모두 cooldown
//  → 자기 자신 위치 기준으로 prev1/prev2/prev3/prev4 중 두 칸 연속 검사
if (has(prev2) && has(prev3)) ||
   (has(prev3) && has(prev4)) {
    // 직전 cooldown 윈도우 안이면 차단
    return ErrK04
}
```

> **청취 필요(추가)**:
> - K-03 부득이한 추가가 "1년에 몇 회까지" 같은 운영 상한이 있는지

---

## 7. 인원 변동 / 월 중 재생성 (R-NN)

| ID | Status | 룰 |
|---|---|---|
| R-01 | pending | 휴직·장기병가 시 남은 일자만 재생성, 지나간 날 freeze |
| R-02 | pending | 신규 입사 월 중간 발생 시 입사 이후 일자만 추가 배정 |
| R-03 | pending | 재생성 시 확정된 swap은 `locked=true` 보존 |

---

## 8. 데이터 모델 (v0.3)

### 8.1 시프트 시간대 (G-01 확정값)

| 코드 | 시간 | 설명 |
|---|---|---|
| `D` | 07:00 ~ 15:00 | Day (8h) |
| `E` | 14:00 ~ 22:00 | Evening (8h, D와 1h 인계) |
| `N` | 21:00 ~ 익일 08:00 | Night (11h, E와 1h 인계) |
| `O` | 24h 휴무 | Off |
| `DE` | 07:00 ~ 22:00 | Double (15h, 인력 부족 시 부득이) |

### 8.2 `experience_levels` 테이블 (G-04 재정의 핵심)

```sql
CREATE TABLE experience_levels (
  id SERIAL PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,              -- 'L1', 'L2', 'L3' 등 head_nurse 자유 정의
  display_name TEXT NOT NULL,             -- '신규', '저연차', '중간연차', '고연차' 등
  min_months INT NOT NULL DEFAULT 0,      -- 이 등급의 hire 후 경과 개월 하한
  max_months INT,                         -- 상한 (NULL = 무제한)
  -- 시프트별 필수 인원 (H-12)
  min_d INT NOT NULL DEFAULT 0,
  min_e INT NOT NULL DEFAULT 0,
  min_n INT NOT NULL DEFAULT 0,
  -- 가중치 (S-10)
  weight_coverage INT NOT NULL DEFAULT 1, -- min_* 부족 시 soft 페널티 강도 (hard는 그대로)
  weight_d_assignment INT NOT NULL DEFAULT 0,  -- 이 등급을 D에 배정 시 cost
  weight_e_assignment INT NOT NULL DEFAULT 0,
  weight_n_assignment INT NOT NULL DEFAULT 0,
  sort_order INT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ DEFAULT NOW()
);
```

**등급 결정 로직 (v0.5 단순화)**:
```
nurse의 등급 =
  IF nurse.experience_level_override IS NOT NULL
    THEN nurse.experience_level_override
  ELSE
    levels[0].code   -- sort_order 가장 낮은 등급 (안전 fallback)
```

> v0.4까지의 hire_date 자동 분류는 제거. min_months/max_months 컬럼은 호환 유지(참고용).

**head_nurse UI 동작**:
- 등급 추가/삭제/이름 변경
- 각 등급의 `min_months`, `max_months` 조정
- 각 등급의 `min_d/min_e/min_n` 시프트당 필수 인원 설정
- 각 등급의 `weight_*` 가중치 분배

### 8.3 `nurses` 변경

```diff
nurses
  id, name, role, email, password_hash, active, created_at,
  hire_date,
- is_senior_override BOOLEAN NULL,
+ experience_level_override TEXT REFERENCES experience_levels(code),
  fixed_shift_pattern TEXT NULL  -- 'D_ONLY'|'E_ONLY'|'N_ONLY'|'WEEKDAY_D'|'WEEKDAY_E'|NULL
```

### 8.4 `schedule_cells.shift` enum 확장

```diff
- shift IN ('D', 'E', 'N', 'O')
+ shift IN ('D', 'E', 'N', 'O', 'DE')
```

### 8.5 `ward_settings` 기본값 (v0.3)

| 키 | 기본값 | 출처 |
|---|---|---|
| `min_d` | 3 | inferred |
| `min_e` | 2 | inferred |
| `min_n` | 2 | inferred |
| `max_consecutive_n` | 3 | inferred (H-02) |
| `min_rest_after_n` | **1** | **interview (H-06 confirmed)** |
| `max_consecutive_workdays` | **5** | **interview (H-03 confirmed)** |
| `balance_off_tolerance` | 1 | inferred |
| `previous_month_lookback_days` | **7** | **interview (H-10)** |
| `night_keeper_max_consecutive_months` | 2 | interview (K-02) |
| `night_keeper_cooldown_months` | **3** | **interview (K-04 변경: 1→3)** |
| `wish_unavailable_quota_monthly` | (미정) | TBD by W-02 |
| `wish_preference_quota_monthly` | (미정) | TBD by W-03 |
| `wish_deadline_days_before_month` | 5 | inferred |
| `swap_deadline_days_before_date` | 1 | inferred |
| `weight_balance_off` | 10 | inferred (S-01) |
| `weight_respect_wishes` | 8 | inferred (S-02) |
| `weight_weekend_balance` | 5 | inferred (S-03) |
| `weight_same_shift_streak` | 3 | inferred (S-04) |
| `weight_short_rest_pattern` | 4 | inferred (S-05) |

> v0.3에서 `seniority_threshold_months` 제거 (G-04 단일 임계값 모델 폐기).

### 8.6 고정 시프트 패턴 enum

`nurses.fixed_shift_pattern`:

| 값 | 의미 |
|---|---|
| `D_ONLY` | 매일 D 또는 Off |
| `E_ONLY` | 매일 E 또는 Off |
| `N_ONLY` | 매일 N 또는 Off (영구 나이트, K-NN과 별개) |
| `WEEKDAY_D` | 평일만 D, 주말 Off |
| `WEEKDAY_E` | 평일만 E, 주말 Off |
| `NULL` | 일반 로테이션 |

> 청취 필요: 실제 운영 중인 패턴이 위 enum과 일치하는지

---

## 9. Open Questions (잔여)

### v0.1에서 이월
- [ ] G-03: 추가 코드(`Edu`/`V`/`S`/`H`) 운영 여부
- [ ] H-01 / H-02: N→D 금지·N 연속 한도 명시 임계값
- [ ] H-04: 시프트별 전체 최소인원 D/E/N 실제 값
- [ ] H-07 / H-08 / H-09: hard 후보 채택 여부
- [ ] S-01 / S-02 / S-04: soft 가중치 상대 중요도
- [ ] S-06 / S-07 / S-08: soft 후보 채택 여부
- [ ] W-02 / W-03 / W-04 / W-06: 희망일 운영 정책
- [ ] X-04 / X-05: swap 시한·강제 변경
- [ ] R-01~R-03: 월 중 재생성 운영 방식

### v0.3 신규 (일부 v0.4에서 해소)
- [x] ~~H-14: DE 후 휴식~~ → v0.4 confirmed (D만 금지)
- [ ] **H-15**: DE 연속 금지 / 월 최대 횟수
- [ ] K-03: 부득이 추가 시 1년 상한
- [ ] G-04 seed: 운영 시 어떤 등급 체계로 시작할지
- [ ] fixed_shift_pattern enum: 운영 중인 실제 패턴 종류 확인

---

## 10. Infeasibility Policy (v0.4 잠정안)

> 솔버가 모든 hard constraint를 만족하는 해를 찾지 못할 때의 동작 정책.
> **v0.4 잠정안 — 운영 1개월 후 회고에서 자동 완화 정책 도입 여부 재검토.**

### 10.1 기본 원칙

- **자동 완화하지 않는다.** 솔버는 hard 제약을 임의로 강등하지 않고 `infeasible`을 그대로 반환.
- head_nurse가 입력(등급별 min, 시프트 최소인원, 나이트킵 지정, 희망일 등)을 수정한 뒤 재생성하도록 한다.
- 자동 완화는 "조용한 룰 위반"을 만들 위험이 커서 의료 도메인 특성상 부적합.

### 10.2 실패 시 동작

1. `schedules.status = 'failed'`
2. `schedules.generation_log` (JSONB)에 다음 기록:
   - `solver_status`: `infeasible` / `timeout` / `error`
   - `violated_rule_ids`: 추정되는 위반 룰 ID 배열 (CP-SAT의 unsat core 기반)
   - `suggestion`: 운영자에게 보여줄 권장 조치 (예: "min_n을 1로 낮추거나 N-only 고정 패턴 인력을 1명 추가하세요")
3. UI에는 위반 룰 ID와 자연어 메시지를 함께 표시.

### 10.3 운영자 해소 경로 (권장 순서)

1. `experience_levels`의 등급별 min_* 완화
2. `ward_settings.min_d/e/n` 완화
3. 나이트킵 지정 검토
4. `unavailable` 희망일이 과도하지 않은지 확인
5. `H-15`(DE 연속 제한) 등 후보 hard가 너무 빡빡하지 않은지 확인

### 10.4 v2 검토 사항

- 단계적 자동 완화 모드 (head_nurse가 "자동 완화 허용" 토글)
- 완화된 룰을 명시 표시한 best-effort 해 반환

---

## Appendix A: Source Log

| Date | Source | 영향받은 룰 | 메모 |
|---|---|---|---|
| 2026-05-24 | inferred (Plan Plus brainstorming) | v0.1 전체 | 초기 가설 |
| 2026-05-24 | interview (1차) | H-10, H-11, H-12, K-01~K-04, G-04, S-09→deprecated | 청취 4건 |
| 2026-05-24 | interview (2차) | G-01·G-04·G-05·H-03·H-06·H-12·H-13·K-03·K-04·S-10 | 청취 7건: ① 등급 사용자 정의 ② 나이트킵 부득이 추가(고연차) ③ cooldown 3달 ④ 시프트 시간대 확정 ⑤ 연속근무 5일·N후 Off 1일 ⑥ N후 Off ≥ 1일 ⑦ DE 더블 시프트 |
| 2026-05-24 | rule-audit + interview (3차, 점검 후속) | G-05·G-06·H-14·K-05·X-03 + §10 신설 | 점검 결과 8건 중 5건 v0.4 반영: ① DE 카운트 D·E 양쪽 명시 ② 공휴일 별도 처리 없음 ③ DE 직후 D만 금지(Off 강제 아님) ④ fixed_shift_pattern과 night_keeper 동시 지정 차단 ⑤ swap 검증 범위 명시. §10 Infeasibility Policy 잠정안 추가(자동 완화 X). |
| 2026-05-24 | interview (4차, 운영 피드백) | G-04 단순화 | hire_date 자동 분류 제거 — 직책·역량을 입사일로 매핑하기 어렵다는 운영 의견. head_nurse가 명단에서 직접 등급 부여. min_months/max_months는 참고용으로 호환 보존. |

---

## Appendix B: 룰 → 코드 역참조 (구현 시 채움)

| Rule ID | 솔버 모델 | API 검증 (Go) | UI | 테스트 |
|---|---|---|---|---|
| H-01 ~ H-09 | TBD | TBD | TBD | TBD |
| H-10 | `add_previous_month_cells_as_const` | `solver.BuildInput` | — | `test_month_boundary` |
| H-11 | `fix_pattern_assignments` | `solver.BuildInput` | "고정 패턴 위반" | `test_fixed_pattern` |
| H-12 | `level_per_shift_min` | — | "이 시프트에 등급 L 부족" | `test_level_coverage` |
| H-13 | `exclude_de_from_domain` | — | "DE는 수동 배정만" | `test_no_auto_de` |
| K-01 | `night_keeper_only_n_or_o` | — | "나이트킵 충돌" | `test_nk_assignment` |
| K-02 | — | `validateNightKeeper` | "연속 3달 금지" | `test_nk_3consec` |
| K-04 | — | `validateNightKeeper` | "cooldown 3달 필요" | `test_nk_cooldown` |
| G-04 | `classify_level` | `nurseLevel(nurse)` | — | `test_level_classification` |
| G-05 | — | — | "DE 시간 07-22" | — |
| S-10 | `level_assignment_cost` | — | — | `test_level_weighting` |

---

## Version History

| Version | Date | Changes |
|---|---|---|
| 0.1 | 2026-05-24 | 초기 가설 룰 정의 — 모든 룰 `pending` |
| 0.2 | 2026-05-24 | 1차 청취 반영: H-10/H-11/H-12, G-04, K-NN 4종, fixed_shift_pattern, S-09 deprecated |
| **0.3** | **2026-05-24** | **2차 청취 반영: G-01 시간대 확정·G-04 다단계 등급 시스템·G-05/H-13 DE 시프트 도입·H-03(=5)·H-06(≥1) confirmed·K-04 cooldown 1→3·K-03 부득이 추가 허용·S-10 등급별 가중치. `experience_levels` 테이블 신설, `seniority_threshold_months` 제거. shift enum 확장.** |
| **0.4** | **2026-05-24** | **룰 점검 + 보강: H-14(DE 직후 D 금지) confirmed, G-06(공휴일 별도 처리 없음) 신규, G-05에 DE 카운트 명시, K-05(fixed_pattern + night_keeper 동시 차단) 신규, X-03 검증 범위(9개 hard ID) 명시, §10 Infeasibility Policy 잠정안 추가(자동 완화 X·fail+사유 기록). 직접적 모순 없음 확인 — design phase 진입 가능.** |
| **0.5** | **2026-05-24** | **G-04 단순화: hire_date 자동 분류 제거, head_nurse가 명단 페이지에서 직접 등급 부여. `experience_level_override` 가 유일한 등급 결정자. min_months/max_months/hire_date는 참고용으로 호환 보존. 미지정 nurse는 sort_order 가장 낮은 등급 fallback. 사용자 운영 피드백 반영: 임상 직책·역량 등급을 자동 매핑하기 어려움. ClassifyLevel 함수 단순화, /nurses·/levels UI 라벨 수정.** |
| **0.6** | **2026-05-25** | **등급 시드 라벨·구간 갱신 (사용자 운영 피드백). 신입→신규(0~4개월), 주니어→저연차(4~24), 중급→중간연차(25~48), 시니어→고연차(49+). v0.5에 따라 hire_date 자동 분류는 여전히 미사용 — 새 구간 숫자는 매니저가 등급 부여할 때 참고하는 가이드 텍스트로만 의미. migration 시드(0001_init.sql) + 라이브 DB(PATCH /api/levels) 양쪽 적용.** |
