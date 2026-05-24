# Local Dev — 맥에서 풀스택 띄우기

> Google OAuth 등록 없이 `dev-login` 모드로 바로 UI 확인. ~10분.

---

## 한 번만 — 인프라 컨테이너

```bash
# PostgreSQL (port 5432)
docker run -d --name local-pg \
  -p 5432:5432 \
  -e POSTGRES_PASSWORD=local \
  postgres:16

# Redis (port 6379, no password — local only)
docker run -d --name local-redis -p 6379:6379 redis:7-alpine

# DB + 마이그레이션 적용
docker exec -it local-pg psql -U postgres -c "CREATE DATABASE ward_duty;"
docker exec -i local-pg psql -U postgres -d ward_duty \
  < ~/projects/ward-duty/api/migrations/0001_init.sql

# 확인
docker exec -it local-pg psql -U postgres -d ward_duty -c "\dt"
```

---

## 세 터미널로 실행

### 1) Solver

```bash
cd ~/projects/ward-duty/solver
source .venv/bin/activate
ENV=development uvicorn app.main:app --port 8000 --reload
```

### 2) Go API

`api/.env.local` 작성:
```env
ENV=development
ALLOWED_ORIGINS=http://localhost:3000

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=local
DB_NAME=ward_duty
DB_SSLMODE=disable

REDIS_HOST=localhost:6379
REDIS_PASSWORD=

JWT_SECRET=local-dev-secret-min-32-characters-padding-xxxxxxxx
JWT_ACCESS_TTL=1h
JWT_REFRESH_TTL=168h

SOLVER_URL=http://localhost:8000
SOLVER_INTERNAL_TOKEN=

ADMIN_EMAIL=yang_donggwui@cocone.co.jp
SKIP_MIGRATE=1
```

실행:
```bash
cd ~/projects/ward-duty/api
set -a && source .env.local && set +a
go run ./cmd/server
# {"level":"INFO","msg":"listening","addr":":8080"}
# {"level":"WARN","msg":"dev-login enabled — DO NOT use ENV=development in production"}
```

### 3) Web

```bash
cd ~/projects/ward-duty/web
cp .env.local.example .env.local   # NEXT_PUBLIC_DEV_LOGIN=1
npm run dev
# ▲ Next.js 14.2.15
# - Local: http://localhost:3000
```

---

## 사용 흐름

1. http://localhost:3000 → 자동으로 `/login`
2. 페이지 하단에 점선 박스 **⚙️ DEV LOGIN** 표시됨
3. 이메일 입력 (예: ADMIN_EMAIL과 같은 값) → **Dev Login** 클릭
4. ADMIN_EMAIL이면 자동으로 `nurses` 테이블에 head_nurse 시드 + 로그인
5. `/` 대시보드 → `/nurses`에서 팀원 추가 → `/duty/2026-06` → **자동 생성** 클릭

---

## 자주 마주치는 문제

| 증상 | 해결 |
|---|---|
| `dev-login enabled` 로그 안 보임 | `ENV=development` 환경변수 누락. `env | grep ENV` 확인 |
| `/login`에 DEV LOGIN 박스 안 보임 | `NEXT_PUBLIC_DEV_LOGIN=1` 누락. `.env.local` 확인 후 `npm run dev` 재기동 |
| `CORS error` (browser console) | `ALLOWED_ORIGINS=http://localhost:3000` env 빠짐. Go API 재기동 |
| `psql: connection refused` | `docker ps`로 `local-pg` 실행 확인. `docker start local-pg` |
| 솔버 호출 시 401 "invalid internal token" | dev에선 토큰 검증 우회 — `SOLVER_INTERNAL_TOKEN=` 비워두면 통과 (routes.py C4 가드) |
| 두 번 실행 시 `nurses` UNIQUE 위반 | 정상. 기존 head_nurse가 재로그인됨 |

---

## 데이터 초기화 (테스트 반복용)

```bash
docker exec -it local-pg psql -U postgres -d ward_duty <<'EOF'
TRUNCATE schedule_cells, swap_requests, night_keeper_assignments, schedules, wishes, nurses RESTART IDENTITY CASCADE;
EOF
# experience_levels와 ward_settings는 보존 (seed)
```

---

## 솔버만 빠르게 확인 (UI 없이)

```bash
cd ~/projects/ward-duty/solver
source .venv/bin/activate

# 회귀 24/24
pytest -q

# 단순 풀이
uvicorn app.main:app --port 8000 &
curl -s -X POST http://localhost:8000/generate \
  -H "Content-Type: application/json" \
  -d @tests/fixtures/simple_30_jun.json | jq '.status, .elapsed_ms'
# "ok" 800
```

---

## 다 끝나면 정리

```bash
docker stop local-pg local-redis
docker rm local-pg local-redis    # 데이터 영구 삭제
# (또는 stop만 — 데이터 보존)
```
