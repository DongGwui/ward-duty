# Operations — Setup Notes

> 홈서버(`dltmxm.link`)에서 ward-duty를 처음 구동할 때 필요한 절차.
> 참고: `~/vault/10-19 Active Projects/50 Infrastructure/51 homeserver/51.01 인프라 레퍼런스.md`

---

## 1. 사전 준비 (홈서버 SSH 후)

```bash
# 1-1. 디렉토리
mkdir -p ~/docker/ward-duty
cd ~/docker/ward-duty

# 1-2. repo 클론 (or rsync from local)
# 옵션 A — git
git clone <repo-url> .
# 옵션 B — 로컬에서 동기화
# (Mac에서) rsync -avz --exclude='.git' --exclude='node_modules' \
#   ~/projects/ward-duty/ user@homeserver:~/docker/ward-duty/

# 1-3. .env 생성
cp .env.example .env

# 1-4. 시크릿 채우기
nano .env
#   DB_PASSWORD=<shared-postgres 비밀번호>
#   REDIS_PASSWORD=$(openssl rand -hex 32)
#   JWT_SECRET=$(openssl rand -hex 64)
#   SOLVER_INTERNAL_TOKEN=$(openssl rand -hex 32)
```

---

## 2. PostgreSQL — 신규 DB 생성

```bash
docker exec -it shared-postgres psql -U postgres -c "CREATE DATABASE ward_duty;"

# 마이그레이션 1차 적용 (Module 1 시점, goose 도입 전)
docker exec -i shared-postgres psql -U postgres -d ward_duty \
  < ~/docker/ward-duty/api/migrations/0001_init.sql

# 확인
docker exec -it shared-postgres psql -U postgres -d ward_duty -c "\dt"
# experience_levels, nurses, wishes, schedules, schedule_cells,
# swap_requests, night_keeper_assignments, ward_settings 8개 확인
```

---

## 3. Cloudflare Tunnel — 서브도메인 등록

Cloudflare 대시보드 → Zero Trust → Networks → Tunnels → `<tunnel>` → Public Hostnames:

| Subdomain | Service |
|---|---|
| `ward-duty` | `http://localhost:80` (Traefik 진입점) |

> Traefik이 Host 헤더 + PathPrefix(`/api`)로 web/api 분기 (`docker-compose.yml` labels).

---

## 4. Docker 네트워크 확인

```bash
docker network ls | grep -E 'proxy|shared'
# 둘 다 있어야 함. 없으면:
docker network create proxy
docker network create shared
```

---

## 5. 솔버만 먼저 실행 (Module 2 검증용)

```bash
cd solver
docker build -t ward-duty-solver:dev .
docker run --rm -p 8000:8000 -e SOLVER_INTERNAL_TOKEN=dev ward-duty-solver:dev

# 다른 터미널
curl -H "X-Internal-Token: dev" http://localhost:8000/health
# {"status":"ok","version":"0.1.0"}
```

---

## 6. 풀스택 기동 (Module 4 이후)

```bash
cd ~/docker/ward-duty
docker compose up -d
docker compose ps
docker compose logs -f api
```

배포 후 확인:
```bash
curl -H "Host: ward-duty.dltmxm.link" http://localhost/         # web 200
curl -H "Host: ward-duty.dltmxm.link" http://localhost/api/auth/me   # 401 (인증 없음, 정상)
```

---

## 7. 백업 / 복원

```bash
# PostgreSQL 백업
docker exec shared-postgres pg_dump -U postgres ward_duty > ward_duty_$(date +%F).sql

# 복원
cat ward_duty_2026-06-01.sql | docker exec -i shared-postgres psql -U postgres -d ward_duty
```

---

## 8. 트러블슈팅

| 증상 | 확인 |
|---|---|
| Traefik 404 | `proxy` 네트워크 연결·라벨 `traefik.enable=true` 확인 |
| api → solver 연결 실패 | `ward-duty-internal` 네트워크에 둘 다 연결됐는지 |
| 솔버 외부 노출 우려 | `traefik.enable` 라벨 없는지 재확인 (Design §7) |
| 솔버 timeout | `SOLVER_TIMEOUT_SECONDS` 늘리기 + `max_time_seconds` (req body) 조정 |
| infeasible 자주 발생 | `ward_settings.min_d/e/n` 또는 `experience_levels.min_*` 완화 (§10 Infeasibility Policy) |
