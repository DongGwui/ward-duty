---
template: plan-plus
version: 1.0
feature: ward-duty-mvp
date: 2026-05-24
author: yang-donggwui
project: ward-duty
version_project: 0.1.0
---

# ward-duty-mvp Planning Document

> **Summary**: 한 병동(10~30명) 단위로 월별 간호사 듀티(D/E/N/Off)를 OR-Tools CP-SAT로 자동 생성하고, 수간호사가 수동으로 다듬는 셀프호스트 풀스택 웹 앱.
>
> **Project**: ward-duty
> **Version**: 0.1.0
> **Author**: yang-donggwui
> **Date**: 2026-05-24
> **Status**: Draft
> **Method**: Plan Plus (Brainstorming-Enhanced PDCA)

---

## Executive Summary

| Perspective | Content |
|-------------|---------|
| **Problem** | 수간호사가 월 수 시간 들여 듀티표를 손으로 짜고, 팀원 희망일은 카톡/종이로 흩어져 누락·갈등이 잦다. |
| **Solution** | OR-Tools CP-SAT 솔버가 hard/soft constraint를 풀어 "그대로 쓸 수 있는" 초안을 생성하고, 수간호사는 위반 표시를 보며 미세 조정만 한다. |
| **Function/UX Effect** | 자동 생성 ≤1분, 초안 80%+ 가 수정 없이 사용 가능. 팀원은 다음 달 희망일을 캘린더에 직접 입력해 누락이 사라진다. |
| **Core Value** | 수간호사의 작성 부담을 분 단위로 줄이고, 팀원 희망 반영을 시스템화하여 공정성과 만족도를 동시에 확보한다. |

---

## 1. User Intent Discovery

### 1.1 Core Problem

수간호사가 월 1회 듀티표를 작성할 때 ① 다수 제약조건(N후 휴식, 시프트별 최소인원, 개인 희망/불가일)을 머릿속으로 풀어야 하고, ② 팀원의 희망일이 카톡·종이쪽지·구두로 흩어져 누락·오기재가 발생한다. 두 문제를 한 시스템에서 해결한다.

### 1.2 Target Users

| User Type | Usage Context | Key Need |
|-----------|---------------|----------|
| 수간호사 (head_nurse) | 월말 다음 달 듀티 작성 | 자동 초안 → 최소 수정으로 확정 |
| 팀원 간호사 (nurse) | 월중 다음 달 희망일 입력 / 확정 듀티 조회 | 모바일에서 빠르게 희망일 등록 / 본인 일정 확인 |

### 1.3 Success Criteria

- [ ] **KPI ①**: 자동 생성된 초안의 80% 이상 셀이 수정 없이 그대로 확정됨
- [ ] 30명 × 31일 기준 자동 생성 응답 시간 60초 이하
- [ ] 확정된 듀티표에 hard constraint 위반 0건
- [ ] 팀원 희망일 입력 흐름이 모바일 브라우저에서 3-탭 이내로 완료

### 1.4 Constraints

| Constraint | Details | Impact |
|------------|---------|--------|
| 비용 | 월 0원 자체호스팅 (도메인 비용 제외) | High |
| 인프라 | 홈서버 `dltmxm.link` (미니PC N150, 16GB) — `51.01 인프라 레퍼런스` 컨벤션 준수 | High |
| 스택 일관성 | 기존 blog-api/games-api가 Go이므로 API는 Go 유지 | Medium |
| 외부 노출 | Cloudflare Tunnel 100초 응답 타임아웃 → 솔버 호출 비동기 필수 | High |
| 리소스 | N150 4코어, 동시 빌드/고동시성 회피 | Medium |

---

## 2. Alternatives Explored

### 2.1 Approach A: OR-Tools CP-SAT (Python 마이크로서비스) — **Selected**

| Aspect | Details |
|--------|---------|
| **Summary** | bkend.ai 대신 셀프호스트 PostgreSQL + Go API + Python FastAPI 솔버 분리 |
| **Pros** | OR-Tools 의료 스케줄링 표준 / KPI 80% 달성 가능성 최상 / 솔버 OOM이 API에 영향 없음 |
| **Cons** | 컨테이너 3종(web/api/solver) 운영. 단 홈서버는 docker compose 한 줄로 끝나 부담 작음 |
| **Effort** | High (솔버 모델링) + Medium (Go API + Next.js web) |
| **Best For** | 자동 생성 품질이 1순위 KPI인 본 프로젝트 |

### 2.2 Approach B: 순수 JS 휴리스틱 (백트래킹 + 로컬 서치)

| Aspect | Details |
|--------|---------|
| **Summary** | Next.js API route에서 그리디 + 백트래킹 + 2-opt swap |
| **Pros** | 스택 단일화 / 의존성 zero |
| **Cons** | 30명·다중 soft constraint에서 품질 보장 어려움 → KPI 80% 미달 위험 / 규칙 추가 시 알고리즘 직접 손봐야 함 |
| **Effort** | Medium |
| **Best For** | 인원 한 자리수, 규칙 2-3개의 단순 환경 |

### 2.3 Approach C: LLM 기반 생성 (Claude/GPT)

| Aspect | Details |
|--------|---------|
| **Summary** | 제약을 프롬프트로 주고 LLM이 표 생성, 후처리 검증 + 재시도 |
| **Pros** | 모델링 코드 최소 / 새 제약을 프롬프트 한 줄로 |
| **Cons** | Hard constraint 100% 만족 보장 불가 / API 비용 (월 0원 제약 위반) / 930셀 토큰 한계 |
| **Effort** | Low (초기) ~ High (검증 루프) |
| **Best For** | 규칙이 모호하고 정성적 판단이 중요한 영역 — 본 프로젝트엔 부적합 |

### 2.4 Decision Rationale

**Selected**: Approach A
**Reason**: KPI "초안 80%+ 그대로 사용"은 **결정론적 제약 만족**이 보장돼야 달성 가능. JS 휴리스틱은 품질 보장 X, LLM은 hard constraint 위반 + 비용 제약 위반. 컨테이너 운영 부담은 홈서버 컨벤션(`docker compose up -d`)이 흡수.

---

## 3. YAGNI Review

### 3.1 Included (v1 Must-Have)

**Always-in (필수)**
- [ ] 월별 듀티표 자동 생성 (D/E/N/Off, OR-Tools CP-SAT)
- [ ] 수간호사 수동 수정 + 실시간 규칙 위반 시각화
- [ ] 팀원 희망일/불가일 입력 화면
- [ ] 이메일 + 비밀번호 로그인 (자체 구현)
- [ ] 권한 분리 (head_nurse / nurse)
- [ ] 간호사 명단 CRUD

**Opt-in (사용자 선택)**
- [ ] 시프트별 최소인원 설정 UI (D≥3, E≥2, N≥2 등 수간호사 조정)
- [ ] 팀원간 근무 교환(swap) 워크플로우 (pending → b_accepted → approved)
- [ ] 클라이언트사이드 PDF/PNG 내보내기 (html2canvas + jsPDF)

### 3.2 Deferred (v2+ Maybe)

| Feature | Reason for Deferral | Revisit When |
|---------|---------------------|--------------|
| 알림 (카톡/메일/푸시) | 외부 서비스 연동 부담 + 월 0원 제약 / MVP 검증 우선 | v1 운영 1개월 후 |
| 과거 N개월 패턴 학습 | 데이터 축적 필요 (최소 3~6개월 운영 데이터) | v1 6개월 후 |
| 멀티테넌트 (병원·병동 다중) | 한 병동 검증 후 확장 결정 | v1 안정화 후 |
| SSO/소셜 로그인 | 한 병동 단위라 이메일로 충분 | 멀티테넌트 검토 시 함께 |
| 서버사이드 PDF 렌더링 (Go gofpdf/chromedp) | 클라이언트사이드로 동일 UX 달성 가능 | 서버 측 일괄 발송이 필요할 때 |

### 3.3 Removed (Won't Do)

| Feature | Reason for Removal |
|---------|-------------------|
| `users` 테이블 분리 | 한 병동 1인=1계정. `nurses`에 email/password_hash 통합이 단순 |
| `sessions` DB 테이블 | 홈서버 컨벤션이 Redis 전용. 세션·refresh 토큰 모두 Redis로 통일 |
| `ward_id` 컬럼 | 단일 병동 전제. 멀티테넌트 v2에서 마이그레이션으로 추가 |
| 서버사이드 듀티 생성 동기 응답 | Cloudflare Tunnel 100초 타임아웃 위험. 비동기로 결정 |

---

## 4. Scope

### 4.1 In Scope

- [ ] Next.js 14 App Router 웹 (로그인, 듀티 그리드, 희망일 캘린더, 명단·설정·swap)
- [ ] Go API (auth, nurses, wishes, schedules, swaps, settings, solver-client)
- [ ] Python FastAPI 솔버 (POST /generate, POST /validate) — OR-Tools CP-SAT
- [ ] shared-postgres 안에 신규 DB `ward_duty` + goose 마이그레이션
- [ ] 전용 ward-duty-redis (세션·refresh 토큰)
- [ ] docker-compose.yml + Traefik 라우팅 (`ward-duty.dltmxm.link`, `/api` 경로)
- [ ] Cloudflare Tunnel 서브도메인 등록

### 4.2 Out of Scope

- 알림 시스템 (v2)
- 과거 패턴 학습 (v2)
- 멀티테넌트 (v2)
- 서버사이드 PDF (v2)
- MinIO 사용 (PDF 즉시 응답이라 영속화 불필요)

---

## 4.3 Rules Document (별도 살아있는 문서)

스케줄링 규칙은 운영하며 계속 진화하므로 **별도 살아있는 문서**로 분리한다.

- 📄 `docs/rules/scheduling-rules.md`
- Plan은 카테고리·UI 영향만 다루고, **구체적인 hard/soft 분류·가중치·임계값·룰 ID는 룰 문서에서 관리**.
- 룰 ID 컨벤션: `H-NN` (hard) / `S-NN` (soft) / `W-NN` (wish) / `X-NN` (swap) / `G-NN` (general) / `R-NN` (regeneration)
- 솔버 모델·API·UI·테스트는 모두 룰 ID로 역참조한다 (Appendix B 매핑표).
- 룰 문서 v0.1은 가설(`pending`) 상태로 초기화돼 있으며, 현장 청취 후 `confirmed`로 승격된다.

---

## 5. Requirements

### 5.1 Functional Requirements

| ID | Requirement | Priority | Status |
|----|-------------|----------|--------|
| FR-01 | 수간호사가 `year_month`를 지정해 듀티 자동 생성 요청, 60초 이내 결과 | High | Pending |
| FR-02 | 생성된 듀티 셀의 hard constraint 위반은 0건 (N→D 금지, 최소인원 미달 금지) | High | Pending |
| FR-03 | 수간호사가 셀을 드래그/클릭으로 수정 시 실시간으로 규칙 위반 표시 | High | Pending |
| FR-04 | 팀원이 자신의 다음 달 희망일(off/n/d/e/unavailable)을 입력·수정 | High | Pending |
| FR-05 | 수간호사가 듀티를 `confirmed` 상태로 확정 (이후 수정은 swap만) | High | Pending |
| FR-06 | 팀원 A가 본인 셀과 팀원 B 셀의 교환 요청 → B 수락 → 수간호사 승인 | Medium | Pending |
| FR-07 | 수간호사가 D/E/N 시프트별 최소인원을 UI에서 조정 | Medium | Pending |
| FR-08 | 간호사 추가·수정·비활성화 (head_nurse 전용) | Medium | Pending |
| FR-09 | 듀티표 화면을 PDF/PNG로 내보내기 (클라이언트사이드) | Low | Pending |
| FR-10 | Google OAuth 로그인 + 팀원별 이메일 화이트리스트 (사전 등록된 이메일만 통과) + JWT(15m) + Refresh(7d, Redis). 첫 head_nurse는 `.env`의 `ADMIN_EMAIL`로 시드 | High | Pending |
| FR-11 | RBAC: head_nurse는 모든 액션, nurse는 본인 wishes·조회·swap만 | High | Pending |
| FR-12 | 연차 등급 — head_nurse가 명단 페이지에서 각 간호사에게 직접 부여 (룰 G-04 v0.5). hire_date는 참고용으로만 보존, 자동 분류 미사용 | High | Pending |
| FR-13 | 고정 시프트 패턴 관리 — head_nurse가 nurses 상세에서 `fixed_shift_pattern` 설정 (룰 H-11) | High | Pending |
| FR-14 | 나이트킵 월별 지정 관리 — head_nurse가 `night_keeper_assignments` 등록 + K-02/K-04 자동 검증 | High | Pending |
| FR-15 | 자동 생성 시 이전 달 마지막 주(`previous_month_lookback_days`) cells를 솔버 입력에 자동 포함 (룰 H-10) | High | Pending |
| FR-16 | 연차 등급 시스템 — head_nurse가 `experience_levels` 자유 정의 (code/이름/기간/등급별 min_d/min_e/min_n/가중치). G-04·H-12·S-10 (단일 임계값 모델 폐기) | High | Pending |
| FR-17 | `DE` 더블 시프트 — 솔버 자동 생성 금지(H-13), head_nurse 수동 수정에서만 배정. shift enum 확장 | Medium | Pending |

### 5.2 Non-Functional Requirements

| Category | Criteria | Measurement Method |
|----------|----------|-------------------|
| Performance | 자동 생성 30명×31일 P95 ≤ 60초 | solver 로깅 + Grafana 대시보드 |
| Performance | API 응답 P95 ≤ 200ms (솔버 호출 제외) | Traefik 액세스 로그 + Grafana |
| 메모리 | api 컨테이너 ≤ 150MB, solver 유휴 시 ≤ 250MB | `docker stats` |
| Security | bcrypt cost 12+, HTTPS only(쿠키 Secure+HttpOnly+SameSite=Lax) | 코드 리뷰 |
| Security | 솔버 외부 노출 금지 (Traefik 라벨 없음, `ward-duty-internal` 네트워크만) | docker-compose 리뷰 |
| 가용성 | 홈서버 재기동 후 모든 컨테이너 자동 시작 (`restart: unless-stopped`) | docker compose 설정 |

---

## 6. Success Criteria

### 6.1 Definition of Done

- [ ] FR-01 ~ FR-11 모두 구현 및 수동 테스트 통과
- [ ] 솔버 회귀 테스트(대표 케이스 30명/31일 5종) 통과
- [ ] Go API 단위 테스트 핵심 핸들러 커버
- [ ] `ward-duty.dltmxm.link` 외부 접속 정상
- [ ] README + 운영 메모(.env, 마이그레이션, 백업) 작성
- [ ] `51 homeserver MOC.md`에 ward-duty 항목 추가

### 6.2 Quality Criteria

- [ ] hard constraint 위반 0건 (회귀 테스트)
- [ ] 솔버 P95 ≤ 60s
- [ ] Go API lint(`golangci-lint`) 에러 0
- [ ] Next.js 빌드 에러 0
- [ ] 모든 컨테이너 healthcheck `healthy`

---

## 7. Risks and Mitigation

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| 솔버가 60초 안에 풀지 못함 (제약 과다·infeasible) | High | Medium | CP-SAT `max_time_in_seconds=55` 설정 후 best-effort 반환 + 위반 표시. infeasible 시 명확한 사유 표시 |
| Cloudflare Tunnel 100초 타임아웃 | High | Low | 솔버 호출 비동기화로 해소 (api는 즉시 202 응답) |
| KPI 80% 미달 (soft constraint 가중치 부적절) | High | Medium | 실제 듀티 3개월치를 회귀 테스트 셋으로 만들어 가중치 튜닝 |
| pg_bigm 등 기존 PostgreSQL 확장이 신규 DB에 영향 | Low | Low | 신규 DB `ward_duty`는 독립 schema, 확장 사용 안 함 |
| Redis 비밀번호·DB 비밀번호 등 secret 노출 | High | Low | `.env`는 git ignore, `~/docker/ward-duty/.env`만 보유. 백업 별도 |
| N150 CPU 동시 빌드 시 다운 | Medium | Low | CI는 GitHub Actions 클라우드 빌드 후 이미지만 pull. 로컬 빌드는 새벽 시간 |
| 수간호사 1인 의존 (사용자가 본인 또는 가족) | Medium | High | 운영 회고를 retros 폴더에 분기별 1회 |

---

## 8. Architecture Considerations

### 8.1 Project Level Selection

| Level | Characteristics | Recommended For | Selected |
|-------|-----------------|-----------------|:--------:|
| **Starter** | Simple structure | Static sites, portfolios | |
| **Dynamic** | Feature-based modules, fullstack | Web apps with backend | ✅ |
| **Enterprise** | Strict layer separation, microservices | High-traffic, complex systems | |

> bkit Dynamic 레벨 — 단 BaaS(bkend.ai) 대신 셀프호스트 스택. `51.01 인프라 레퍼런스` 컨벤션 우선.

### 8.2 Key Decisions

| Decision | Options | Selected | Rationale |
|----------|---------|----------|-----------|
| 자동 생성 알고리즘 | CP-SAT / JS 휴리스틱 / LLM | **CP-SAT** | KPI 80% 달성 가능성 |
| 호스팅 | bkend.ai / Vercel / 홈서버 | **홈서버** | 월 0원 제약 |
| API 언어 | Go / Python / Node | **Go** | 홈서버 컨벤션, 메모리 50~100MB |
| 솔버 언어 | Python / Node OR-Tools 포팅 | **Python** | OR-Tools 공식 우선 지원 |
| DB | shared-postgres / 전용 PG / SQLite | **shared-postgres** | 공유 인프라 활용, 백업 통합 |
| 세션 | DB / Redis / JWT only | **Redis (전용)** | 홈서버 컨벤션 |
| 마이그레이션 | goose / golang-migrate / atlas | **goose** | 단순함, CLI 사용 편의 |
| API 라우터 | Chi / Echo / Gin | **Chi** | 표준 net/http 호환, 경량 |
| 솔버 호출 | 동기 / 비동기(폴링) / SSE | **비동기 폴링** | Cloudflare 타임아웃 회피, MVP 단순함 (SSE는 v2) |
| 인증 토큰 | JWT only / Refresh+Access | **Access(15m) + Refresh(7d, Redis)** | 보안 + 사용성 균형 |
| PDF 렌더링 | 서버(gofpdf/chromedp) / 클라이언트(jsPDF) | **클라이언트** | 서버 부하 0, 모듈 1개 절약 |
| 라우팅 | 서브도메인 분리 / 경로 분리 | **경로 분리 `/api`** | 홈서버 games 패턴 동일, 쿠키 단일 도메인 |
| 시니어 판정 | 단일 임계값 / 다단계 등급 사용자 정의 | **다단계 등급 (`experience_levels` 테이블)** | head_nurse가 등급·필수 인원·가중치 자유 설정 (룰 G-04 재정의) |
| 고정 시프트 표현 | enum / JSON 패턴 / 별도 테이블 | **`nurses.fixed_shift_pattern` enum** | MVP는 enum 5종으로 충분, 복잡 패턴은 v2 |
| 나이트킵 표현 | 컬럼 플래그 / 별도 테이블 | **별도 `night_keeper_assignments` 테이블** | 월 단위 이력·검증·통계 모두 필요 (룰 K-NN) |
| 월 경계 처리 | 월 독립 / 이전 달 마지막 주 입력 | **이전 달 마지막 N일 cells를 솔버에 상수로 전달** | 첫 주 H-01/H-02/H-03/H-06 보장 (룰 H-10) |
| 더블 시프트 표현 | 별도 row / shift enum 확장 / 별도 컬럼 | **`shift` enum에 `DE` 추가** | 단일 row 유지로 단순, 솔버 자동 생성 도메인에서 제외 (룰 G-05·H-13) |
| 더블 시프트 자동화 | 솔버 자동 / soft 회피 / hard 차단 | **hard 차단 (솔버 자동 불가)** | "정말 부득이한 경우" 운영 의도 반영, 수동 배정 전용 (룰 H-13) |

### 8.3 Component Overview

```
ward-duty/
├── docker-compose.yml          ← Traefik 라벨 + networks(proxy, shared, ward-duty-internal)
├── .env.example
│
├── web/                        ward-duty-web (Next.js 14 App Router)
│   ├── app/
│   │   ├── (auth)/login
│   │   ├── duty/[yyyymm]       월별 듀티 그리드 (드래그·위반 표시)
│   │   ├── wishes              희망일 캘린더
│   │   ├── nurses              간호사 명단 (head_nurse)
│   │   ├── settings            시프트 최소인원 (head_nurse)
│   │   └── swaps               근무 교환
│   ├── components/             SchedulerGrid, WishCalendar, RuleViolationBadge ...
│   ├── lib/api.ts              JWT 자동 첨부, refresh 자동
│   └── Dockerfile
│
├── api/                        ward-duty-api (Go + Chi)
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── auth                bcrypt + JWT 발급/검증 + RBAC 미들웨어
│   │   ├── nurses              CRUD
│   │   ├── wishes              팀원 본인 / head_nurse 전체 조회
│   │   ├── schedules           생성 dispatch / 조회 / 셀 PATCH / 확정
│   │   ├── swaps               상태 머신
│   │   ├── settings            ward_settings single-row
│   │   ├── solver              ward-duty-solver HTTP 클라이언트 (internal-only)
│   │   └── db                  pgx + goose
│   ├── migrations/             0001_init.sql, 0002_schedule_status.sql, ...
│   └── Dockerfile
│
└── solver/                     ward-duty-solver (Python 3.12 + FastAPI + OR-Tools)
    ├── app/
    │   ├── main.py
    │   ├── routes.py           POST /generate, POST /validate
    │   ├── model.py            CP-SAT (hard + weighted soft)
    │   └── schemas.py          Pydantic
    ├── tests/                  대표 케이스 회귀
    └── Dockerfile
```

### 8.4 Data Flow

**A. 자동 생성 (비동기)**

```
Browser ──POST /api/schedules──▶ api ─INSERT schedule(status=generating)
                                  │
                                  ├─ load nurses + 등급 분류 (G-04, experience_levels)
                                  ├─ load wishes (target month)
                                  ├─ load previous_month_last_week cells (H-10)
                                  ├─ load fixed_shift_pattern per nurse (H-11)
                                  ├─ load night_keeper_assignments for ym (K-01)
                                  ├─ load ward_settings + experience_levels (H-12, S-10)
                                  ├─ solver 도메인에서 DE 제외 (H-13)
                                  │
                                  └─ goroutine ──POST solver:/generate──▶ solver
                                       (timeout 5m)
Browser ◀──── 202 { schedule_id, status: 'generating' } ───┘

Browser ── poll GET /api/schedules/:id ──▶ api ──▶ DB

solver (수~수십 초) ──cells──▶ api ─UPSERT cells + UPDATE status=generated

Browser (poll) ──▶ status==='generated' → GET /cells → render grid
```

**B. 수동 수정 + 검증**

```
드래그 ── PATCH /api/schedules/:id/cells/:cellId { shift } ──▶ api
   ├─ UPDATE cell (source=manual, modified_by)
   ├─ POST solver:/validate { all_cells_after, settings }
   └─ ◀── { violations: [...] }
Browser ── 셀에 빨간 테두리 + 툴팁 / hard 위반 시 확정 차단
```

**C. Swap 워크플로우**

```
A ── POST /api/swaps {schedule, requester_date, target_nurse, target_date}
                                                ↓ status=pending
B ── PATCH /api/swaps/:id {accept}              ↓ status=b_accepted
head_nurse ── PATCH /api/swaps/:id {approve}
   ├─ TX: schedule_cells 2건 shift 교환 + swap status=approved
   └─ solver /validate 한 번 더 → 위반 시 거부 + rollback
```

### 8.5 Data Model (요약)

```
nurses              (id, name, role[head_nurse|nurse], email UNIQUE, password_hash,
                     hire_date,                              -- 등급 자동분류 기준 (G-04)
                     experience_level_override TEXT NULL,    -- experience_levels.code 참조 (G-04)
                     fixed_shift_pattern TEXT NULL,          -- 'D_ONLY'|'E_ONLY'|'N_ONLY'|'WEEKDAY_D'|'WEEKDAY_E'|NULL (H-11)
                     active, created_at)

experience_levels   -- v0.3 신규 (G-04 다단계 등급, H-12·S-10)
                    (id, code UNIQUE, display_name,
                     min_months, max_months,                 -- hire 후 경과 개월로 자동 분류 (NULL=무제한)
                     min_d, min_e, min_n,                    -- 등급별 시프트당 필수 인원 (H-12)
                     weight_coverage,                        -- min_* 부족 시 페널티
                     weight_d_assignment, weight_e_assignment, weight_n_assignment,  -- 등급별 시프트 배정 cost (S-10)
                     sort_order, updated_at)

wishes              (id, nurse_id FK, date DATE, type[off|n|d|e|unavailable],
                     reason, created_at)
                     UNIQUE(nurse_id, date)

schedules           (id, year_month CHAR(7) UNIQUE,
                     status[draft|generating|generated|confirmed|failed],
                     generated_at, confirmed_at, generation_log JSONB)

schedule_cells      (id, schedule_id FK, nurse_id FK, date DATE,
                     shift[D|E|N|O|DE],                      -- v0.3에서 DE 추가 (G-05)
                     source[auto|manual], modified_by_nurse_id, updated_at)
                     UNIQUE(schedule_id, nurse_id, date)

swap_requests       (id, schedule_id FK, requester_nurse_id FK, target_nurse_id FK,
                     requester_date DATE, target_date DATE,
                     status[pending|b_accepted|approved|rejected_by_b|rejected_by_head|cancelled],
                     created_at, updated_at)

night_keeper_assignments  -- v0.2 신규 (K-NN)
                    (id, nurse_id FK, year_month CHAR(7),
                     assigned_by_nurse_id FK, reason TEXT, created_at)
                     UNIQUE(nurse_id, year_month)
                     INDEX (year_month)

ward_settings       (id=1,
                     min_d, min_e, min_n,                              -- H-04 (전체)
                     max_consecutive_n,                                -- H-02
                     min_rest_after_n = 1,                             -- H-06 (confirmed v0.3)
                     max_consecutive_workdays = 5,                     -- H-03 (confirmed v0.3)
                     balance_off_tolerance,
                     previous_month_lookback_days = 7,                 -- H-10
                     night_keeper_max_consecutive_months = 2,          -- K-02
                     night_keeper_cooldown_months = 3,                 -- K-04 (v0.3 변경: 1→3)
                     wish_deadline_days_before_month,
                     swap_deadline_days_before_date,
                     weight_balance_off, weight_respect_wishes,
                     weight_weekend_balance, weight_same_shift_streak,
                     weight_short_rest_pattern,
                     updated_at)  -- single-row
                     -- v0.3에서 seniority_threshold_months 제거 (G-04 다단계로 일반화)
```

> 모든 룰의 ID·임계값·기본값은 `docs/rules/scheduling-rules.md` v0.2 §7~§8 참조.

---

## 9. Convention Prerequisites

### 9.1 Applicable Conventions

- [x] 홈서버 인프라 컨벤션 (`51.01 인프라 레퍼런스`) 확인 — shared-postgres / Traefik / Cloudflare Tunnel 활용
- [x] 디렉토리 네이밍 kebab-case 확인 — `ward-duty` 채택
- [x] 도메인 패턴 확인 — `<프로젝트>.dltmxm.link`
- [ ] Go 프로젝트 구조 컨벤션 — 기존 `blog-api` 구조 참고하여 design phase에서 결정
- [ ] Next.js 디렉토리 구조 — design phase에서 확정
- [ ] 마이그레이션 네이밍 — `0001_init.sql` (goose 기본)

---

## 10. Next Steps

1. [ ] **`/pdca design ward-duty-mvp`** — Plan을 design 문서로 구체화 (특히 CP-SAT 제약 모델, API 스펙, 컴포넌트 트리)
2. [ ] design 검토 후 **`/pdca do ward-duty-mvp`** — 구현 시작
3. [ ] **운영 준비**
   - shared-postgres에 DB `ward_duty` 생성
   - Cloudflare Tunnel에 `ward-duty.dltmxm.link` 서브도메인 추가
   - `~/docker/ward-duty/` 디렉토리 + `.env` 생성
4. [ ] **`51 homeserver MOC.md`** 운영 중 서비스 표에 `ward-duty.dltmxm.link` 추가

---

## Appendix A: Brainstorming Log

| Phase | Question | Answer | Decision |
|-------|----------|--------|----------|
| Project Setup | 프로젝트 시작 방식 | Dynamic 풀스택 + 한 병동(10~30명) | Dynamic 레벨 채택 |
| Intent (시나리오) | 주 사용자와 시나리오 | 수간호사 작성 + 팀원 희망일 입력 + 조회 | 권한 2종 (head_nurse / nurse) |
| Intent (KPI) | 성공 지표 1순위 | 그대로 쓸 수 있는 초안 비율 (80%+) | 알고리즘 품질 최우선 |
| Alternatives | 자동 생성 알고리즘 | CP-SAT vs 휴리스틱 vs LLM | **CP-SAT (Python)** |
| YAGNI (기본) | 기본 포함 6종 | 자동생성/수동수정/희망일/이메일로그인/권한분리/명단CRUD | 모두 포함 |
| YAGNI (선택) | 추가 v1 항목 | 시프트별 최소인원 UI / swap / PDF | 3개 모두 포함 |
| YAGNI (선택) | 알림 | 외부 연동·비용 부담 | v2 이후 |
| Architecture (호스팅) | 비용 제약 | 월 0원 자체호스팅 | bkend.ai 제외, 홈서버 채택 |
| Architecture (스택) | 최선 구조 | Go API + Python 솔버 3-tier | 분리 채택 (장애 격리·메모리·일관성) |
| Components | 모듈 검토 | users/sessions/pdf 과도 + ward_id 미사용 + 솔버 동기 호출 위험 | 5개 조정 (테이블 9→6, PDF 클라이언트사이드, 솔버 비동기) |
| Data Flow | 데이터 흐름 확정 | 자동생성 비동기 폴링 / 수정시 solver validate / swap 3단계 | 그대로 진행 |

---

## Appendix B: Infrastructure Mapping (참조: `51.01 인프라 레퍼런스`)

| 인프라 항목 | 본 프로젝트 사용 |
|---|---|
| 하드웨어 | 미니 PC N150, 16GB / 512GB |
| OS | Ubuntu Server LTS |
| 도메인 | `ward-duty.dltmxm.link` (신규 서브도메인 등록 필요) |
| 외부 접근 | Cloudflare Tunnel |
| 리버스 프록시 | Traefik (`web` entrypoint) |
| 공유 PostgreSQL | `shared-postgres`, DB명 `ward_duty` 신규 생성 |
| 공유 MinIO | **사용 안 함** (PDF는 클라이언트사이드 즉시 생성) |
| 전용 Redis | `ward-duty-redis` (세션·refresh 토큰) |
| 네트워크 | `proxy` + `shared` + `ward-duty-internal` (solver는 internal만) |
| 디렉토리 | `~/docker/ward-duty/` |
| Repo | `~/projects/ward-duty/` |

---

## Version History

| Version | Date | Changes | Author |
|---------|------|---------|--------|
| 0.1 | 2026-05-24 | Initial draft (Plan Plus) | yang-donggwui |
| 0.2 | 2026-05-24 | 1차 청취 반영: FR-12~15 추가, `nurses`에 hire_date·is_senior_override·fixed_shift_pattern 컬럼 추가, `night_keeper_assignments` 테이블 신설, `ward_settings`에 5개 키 추가, 솔버 입력 흐름에 이전 달·고정패턴·나이트킵·시니어 컨텍스트 명시. 구체 룰은 `docs/rules/scheduling-rules.md` v0.2 위임 | yang-donggwui |
| 0.3 | 2026-05-24 | 2차 청취 반영: FR-16(연차 등급 시스템)·FR-17(DE 시프트) 추가, `experience_levels` 테이블 신설, `nurses.is_senior_override` → `experience_level_override` 일반화, `schedule_cells.shift` enum에 `DE` 추가, `ward_settings`에서 seniority_threshold_months 제거·night_keeper_cooldown_months 1→3, 솔버 입력에 등급·DE 제외 명시. 구체 룰은 `docs/rules/scheduling-rules.md` v0.3 위임 | yang-donggwui |
