# ward-duty

> 간호사 듀티(근무표) 자동 생성 + 수동 조정 풀스택 웹앱. 한 병동(10~30명) 단위.

## 스택

- **Web**: Next.js 14 App Router (TypeScript)
- **API**: Go + Chi (Vertical Slices)
- **Solver**: Python 3.12 + FastAPI + OR-Tools CP-SAT
- **DB**: PostgreSQL (shared-postgres / DB `ward_duty`)
- **Cache**: Redis (`ward-duty-redis`, 전용)
- **배포**: 홈서버 + Traefik + Cloudflare Tunnel (`ward-duty.dltmxm.link`)

## 문서

- [Plan](docs/01-plan/features/ward-duty-mvp.plan.md) — PDCA Plan v0.3
- [Design](docs/02-design/features/ward-duty-mvp.design.md) — Pragmatic Vertical Slices
- [Rules](docs/rules/scheduling-rules.md) ⚡ **살아있는 문서** — 41개 룰, ID 매핑
- [Setup Notes](docs/operations/setup-notes.md) — 운영 메모

## 디렉토리

```
ward-duty/
├── docker-compose.yml
├── .env.example
├── docs/                  # plan, design, rules, operations
├── web/                   # Next.js (Module 6+)
├── api/                   # Go API (Module 4+)
│   └── migrations/        # 0001_init.sql ...
└── solver/                # Python FastAPI + OR-Tools (Module 2 ✅)
    ├── app/
    └── tests/
```

## 개발 흐름 (Session Guide)

| Session | Scope |
|---|---|
| ✅ 1 | Plan + Design |
| ▶️ 2 | `module-1, module-2` (인프라 + 솔버 코어) |
| 3 | `module-3, module-4` (솔버 API + auth/명단) |
| 4 | `module-5` (API 도메인) |
| 5 | `module-6, module-7` (Web 기반 + 듀티 그리드) |
| 6 | `module-8, module-9` (Web 관리 + 배포) |
| 7 | Check + Report |

## 빠른 실행 (솔버만)

```bash
cd solver
docker build -t ward-duty-solver .
docker run --rm -p 8000:8000 ward-duty-solver

# 단위/회귀 테스트
docker run --rm ward-duty-solver pytest -v
```

## 풀스택 실행 (Module 4 이후)

```bash
cp .env.example .env  # 시크릿 채우기
docker compose up -d
```

자세한 운영 절차는 [docs/operations/setup-notes.md](docs/operations/setup-notes.md) 참고.

## 라이선스

내부 프로젝트.
