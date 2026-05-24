# Operations — Setup Notes

> 홈서버(`dltmxm.link`)에서 ward-duty MVP를 처음부터 끝까지 배포·운영하는 단계별 절차.
> 인프라 컨벤션: `~/vault/10-19 Active Projects/50 Infrastructure/51 homeserver/51.01 인프라 레퍼런스.md`

---

## 0. 사전 확인

홈서버에 다음이 이미 갖춰져 있어야 합니다 (`51.01` 기준):
- [ ] Docker · `docker compose` 가용
- [ ] `shared-postgres` 컨테이너 실행 중
- [ ] `shared` / `proxy` 도커 네트워크 존재
- [ ] Traefik(entrypoint `web`) 실행 중
- [ ] Cloudflare Tunnel 토큰 보유 (대시보드 → Zero Trust → Networks → Tunnels)
- [ ] 도메인 `dltmxm.link` Cloudflare DNS 활성

---

## 1. Google OAuth 2.0 Client ID 생성 (외부 등록)

> 사용자가 Cloud Console에서 직접 1회 작업. 약 5분.

1. https://console.cloud.google.com → 프로젝트 선택 또는 생성 (예: `ward-duty-prod`)
2. **APIs & Services → OAuth consent screen**
   - User Type: External (또는 G-Suite면 Internal)
   - App name: `ward-duty`
   - User support email · Developer contact: 본인 이메일
   - Scopes: `openid`, `email`, `profile` 추가
   - Test users (External 모드 + Publishing status="Testing"일 때): 화이트리스트할 팀원 이메일 등록
3. **APIs & Services → Credentials → Create Credentials → OAuth client ID**
   - Application type: **Web application**
   - Name: `ward-duty-web`
   - **Authorized JavaScript origins**: `https://ward-duty.dltmxm.link`
   - **Authorized redirect URIs**: `https://ward-duty.dltmxm.link/api/auth/oauth/google/callback`
   - **Save** → Client ID와 Client Secret 메모

> ⚠️ External 모드는 Publishing status를 "In production"으로 올리지 않으면 7일 후 refresh token 만료. MVP에선 "Testing" + Test users 등록으로 충분 (한 병동 규모).

---

## 2. Cloudflare Tunnel — 서브도메인 등록 (외부 등록)

1. Cloudflare 대시보드 → **Zero Trust → Networks → Tunnels → `<tunnel>` → Public Hostname → Add a public hostname**
2. 설정:
   - **Subdomain**: `ward-duty`
   - **Domain**: `dltmxm.link`
   - **Service Type**: HTTP
   - **URL**: `localhost:80` (Traefik 진입점)
3. Save.
4. (선택) 같은 화면 **Additional application settings → HTTP Settings → No TLS Verify** 체크 (Traefik 내부 평문 통신 시 필요)

확인:
```bash
curl -I https://ward-duty.dltmxm.link/health
# HTTP/2 200
```

---

## 3. 홈서버 SSH — repo 배치

```bash
ssh user@homeserver

mkdir -p ~/docker/ward-duty
cd ~/docker/ward-duty

# (a) git clone
git clone https://github.com/DongGwui/ward-duty.git .

# (b) 또는 로컬에서 rsync
# (Mac에서) rsync -avz --exclude='.git' --exclude='node_modules' --exclude='.venv' \
#   ~/projects/ward-duty/ user@homeserver:~/docker/ward-duty/
```

---

## 4. 시크릿 생성 & `.env` 작성

```bash
cd ~/docker/ward-duty
cp .env.example .env
```

필요한 값 생성 (홈서버에서):
```bash
echo "REDIS_PASSWORD=$(openssl rand -hex 32)"
echo "JWT_SECRET=$(openssl rand -hex 64)"
echo "SOLVER_INTERNAL_TOKEN=$(openssl rand -hex 32)"
echo "OAUTH_STATE_SECRET=$(openssl rand -hex 32)"
```

`.env` 채우기:
```env
PROJECT_NAME=ward-duty

DB_HOST=shared-postgres
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=<shared-postgres 비밀번호>
DB_NAME=ward_duty
DB_SSLMODE=disable

REDIS_HOST=ward-duty-redis:6379
REDIS_PASSWORD=<위에서 생성>

JWT_SECRET=<위에서 생성>
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=168h

GOOGLE_CLIENT_ID=<§1에서 받은 값>.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=<§1에서 받은 값>
OAUTH_REDIRECT_URL=https://ward-duty.dltmxm.link/api/auth/oauth/google/callback
OAUTH_STATE_SECRET=<위에서 생성>
ADMIN_EMAIL=head@example.com               # 첫 head_nurse가 될 이메일 (Google 계정)

SOLVER_URL=http://ward-duty-solver:8000
SOLVER_INTERNAL_TOKEN=<위에서 생성>
SOLVER_TIMEOUT_SECONDS=300

NEXT_PUBLIC_API_URL=/api
LOG_LEVEL=info
```

> 추가로 컨테이너 안에 `ENV=production` 표시가 필요한 경우 docker-compose.yml의 solver/api service environment에 `ENV=production` 추가 (Stage A C4 가드).

---

## 5. PostgreSQL DB & 마이그레이션

```bash
# DB 생성
docker exec -it shared-postgres psql -U postgres -c "CREATE DATABASE ward_duty;"

# 마이그레이션 적용 (goose runner는 컨테이너 부팅 시 자동, 또는 수동:)
docker exec -i shared-postgres psql -U postgres -d ward_duty \
  < ~/docker/ward-duty/api/migrations/0001_init.sql

# 확인
docker exec -it shared-postgres psql -U postgres -d ward_duty -c "\dt"
# experience_levels, nurses, wishes, schedules, schedule_cells,
# swap_requests, night_keeper_assignments, ward_settings 8개 확인
```

---

## 6. 풀스택 기동

```bash
cd ~/docker/ward-duty
docker compose up -d --build

# 상태 확인
docker compose ps
# ward-duty-web        running
# ward-duty-api        running
# ward-duty-solver     running (healthy)
# ward-duty-redis      running (healthy)

# 로그 확인
docker compose logs -f api
docker compose logs -f solver
```

---

## 7. 종단간 점검 (Smoke Test)

`scripts/smoke.sh` 가 자동 검사. 로컬 또는 홈서버에서:
```bash
bash scripts/smoke.sh https://ward-duty.dltmxm.link
```

또는 수동:
```bash
# 1. Web 200
curl -I https://ward-duty.dltmxm.link/
# HTTP/2 200

# 2. API health (인증 불필요)
curl -i https://ward-duty.dltmxm.link/health
# 200 {"status":"ok"}

# 3. 인증 필요 — 401 정상
curl -i https://ward-duty.dltmxm.link/api/auth/me
# 401 UNAUTHORIZED

# 4. Solver는 외부에서 보이면 안 됨
curl -i https://ward-duty.dltmxm.link/solver/health  || echo "ok — 외부 노출 안 됨"
# 404 또는 라우팅 없음 (Traefik 라벨 없음)

# 5. 브라우저 OAuth
# https://ward-duty.dltmxm.link/login → Google 로그인 → ADMIN_EMAIL이면 자동 head_nurse 생성
```

브라우저 흐름:
1. `https://ward-duty.dltmxm.link` 접속 → `/login`으로 자동 리다이렉트
2. **Google로 로그인** 클릭 → Google → consent 화면 → callback
3. 등록된 이메일이면 → `/` 대시보드
4. 첫 head_nurse는 자동 생성됨 (ADMIN_EMAIL)
5. `/nurses`에서 팀원 추가 → 팀원도 Google 로그인 가능

---

## 8. 운영자 자주 쓰는 명령어

```bash
# 컨테이너 상태
docker compose ps

# 솔버만 재기동 (배포 후)
docker compose restart solver

# DB 백업
docker exec shared-postgres pg_dump -U postgres ward_duty > "ward_duty_$(date +%F).sql"

# stale 'generating' schedule 강제 reset (운영자가 UI에서도 가능)
# Web: /duty/[ym]에서 status=failed면 "초기화" 버튼

# 솔버 회귀 테스트 단독 실행 (이미지 빌드된 상태)
docker run --rm ward-duty-solver pytest -v
```

---

## 9. 트러블슈팅

| 증상 | 확인 |
|---|---|
| Traefik 404 | `proxy` 네트워크 연결·라벨 `traefik.enable=true` 확인 |
| OAuth 401 "id_token validate" | Cloud Console의 Authorized redirect URI 일치 확인 |
| OAuth 403 "초대받지 않은 이메일" | head_nurse가 /nurses에서 해당 이메일 등록 필요 |
| 솔버 호출 무한 hang | docker compose logs solver. INFEASIBLE이면 `/duty/[ym]`의 suggestion 확인 후 ward_settings 완화 |
| `schedule_status='generating'` 영구 잔존 | UI에서 "초기화" 또는 `POST /api/schedules/:id/reset` 호출 (head_nurse) |
| Cloudflare 524 timeout (100s) | 솔버 호출은 비동기 dispatch라 정상적으론 발생 X. 발생 시 API의 동기 경로(schedule POST 자체) 점검 |

---

## 10. 관리자 부트스트랩 회복 (ADMIN_EMAIL 분실 시)

H2 fix(Stage B) 적용 전엔 ADMIN_EMAIL이 변경되면 그 이메일로 즉시 head_nurse 시드되므로 주의. DB 직접 조작 회복:
```bash
docker exec -it shared-postgres psql -U postgres -d ward_duty
-- 기존 head 확인
SELECT id, name, email FROM nurses WHERE role = 'head_nurse';
-- 권한 부여
UPDATE nurses SET role = 'head_nurse' WHERE email = '<복구할 이메일>';
```

---

## 11. 잔여 보안 부채 (Stage B 이월 — 운영 1차 후)

`docs/03-analysis/ward-duty-mvp.intermediate-review.md` 의 High 12건 — 1차 배포 직후 sprint에서 처리:

- H2: ADMIN_EMAIL 시드를 head_nurse 0명일 때만으로 제한
- H4: schedules.runSolver를 appCtx + WaitGroup으로 묶어 shutdown race 해소
- H5: swap approve를 단일 TX (cells swap + validate + commit)
- H6: confirmed schedule의 cell PATCH 차단
- H7: 5xx 응답에서 err.Error() → generic + request_id로 마스킹
- H8: 보안 헤더(HSTS·X-Frame-Options·Referrer-Policy 등) 미들웨어
- H9: FRONTEND_REDIRECT_PATH open redirect 가드
- H10: rate limit (IP + nurse_id 2단)
- H12: solver infeasibility 메시지 정밀화 (등급별 미충족 사전 검사)
