-- Design Ref: §3.1 DDL
-- shared-postgres / DB: ward_duty
-- 적용: docker exec -i shared-postgres psql -U postgres -d ward_duty < 0001_init.sql
-- Migration runner는 Module 4(api-core)에서 goose 도입 예정.

-- goose Up
-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- ENUMS
-- ============================================================

CREATE TYPE nurse_role        AS ENUM ('head_nurse', 'nurse');
CREATE TYPE fixed_pattern     AS ENUM ('D_ONLY', 'E_ONLY', 'N_ONLY', 'WEEKDAY_D', 'WEEKDAY_E');
CREATE TYPE wish_type         AS ENUM ('off', 'd', 'e', 'n', 'unavailable');
CREATE TYPE shift_code        AS ENUM ('D', 'E', 'N', 'O', 'DE');                -- G-01, G-05
CREATE TYPE schedule_status   AS ENUM ('draft', 'generating', 'generated', 'confirmed', 'failed');
CREATE TYPE cell_source       AS ENUM ('auto', 'manual');
CREATE TYPE swap_status       AS ENUM (
  'pending', 'b_accepted', 'approved',
  'rejected_by_b', 'rejected_by_head', 'cancelled'
);

-- ============================================================
-- experience_levels — G-04 다단계 등급 (head_nurse 자유 정의)
-- H-12 (등급별 시프트당 최소 인원) + S-10 (등급별 배정 가중치)
-- ============================================================

CREATE TABLE experience_levels (
  id                    SERIAL PRIMARY KEY,
  code                  TEXT NOT NULL UNIQUE,
  display_name          TEXT NOT NULL,
  min_months            INT NOT NULL DEFAULT 0,
  max_months            INT,                                  -- NULL = 무제한
  min_d                 INT NOT NULL DEFAULT 0,
  min_e                 INT NOT NULL DEFAULT 0,
  min_n                 INT NOT NULL DEFAULT 0,
  weight_coverage       INT NOT NULL DEFAULT 1,
  weight_d_assignment   INT NOT NULL DEFAULT 0,
  weight_e_assignment   INT NOT NULL DEFAULT 0,
  weight_n_assignment   INT NOT NULL DEFAULT 0,
  sort_order            INT NOT NULL DEFAULT 0,
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 초기 시드 (head_nurse가 /levels 화면에서 자유 변경)
INSERT INTO experience_levels (code, display_name, min_months, max_months, min_d, min_e, min_n, sort_order) VALUES
  ('L1', '신입',    0,   12,   0, 0, 0, 1),
  ('L2', '주니어', 12,   36,   0, 0, 0, 2),
  ('L3', '중급',   36,   84,   1, 1, 1, 3),
  ('L4', '시니어', 84, NULL,   0, 0, 0, 4);

-- ============================================================
-- nurses
-- ============================================================

CREATE TABLE nurses (
  id                          SERIAL PRIMARY KEY,
  name                        TEXT NOT NULL,
  role                        nurse_role NOT NULL DEFAULT 'nurse',
  email                       TEXT UNIQUE,                     -- NULL 허용: 계정 없이 명단만 추가 가능 (Stage 1)
  google_sub                  TEXT UNIQUE,                     -- Google subject ID, 첫 로그인 시 upsert
  hire_date                   DATE,                            -- 참고용 (G-04 v0.5: 자동 분류 미사용)
  experience_level_override   TEXT REFERENCES experience_levels(code),
  fixed_shift_pattern         fixed_pattern,                   -- H-11
  active                      BOOLEAN NOT NULL DEFAULT TRUE,
  last_login_at               TIMESTAMPTZ,
  created_at                  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_nurses_active ON nurses(active);
COMMENT ON COLUMN nurses.email                     IS 'FR-10 (Stage 1): NULL 허용. 계정 연결된 nurse만 값 보유';
COMMENT ON COLUMN nurses.google_sub                IS 'FR-10: 구글 sub. 첫 로그인 시 upsert. 이메일 변경에도 안정적';
COMMENT ON COLUMN nurses.experience_level_override IS 'G-04: NULL이면 hire_date 기반 자동 분류';
COMMENT ON COLUMN nurses.fixed_shift_pattern       IS 'H-11: NULL이면 일반 로테이션. K-05로 night_keeper와 동시 X';

-- ============================================================
-- wishes — W-01
-- ============================================================

CREATE TABLE wishes (
  id            SERIAL PRIMARY KEY,
  nurse_id      INT NOT NULL REFERENCES nurses(id) ON DELETE CASCADE,
  date          DATE NOT NULL,
  type          wish_type NOT NULL,
  reason        TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(nurse_id, date)
);
CREATE INDEX idx_wishes_date ON wishes(date);

-- ============================================================
-- schedules
-- ============================================================

CREATE TABLE schedules (
  id              SERIAL PRIMARY KEY,
  year_month      CHAR(7) NOT NULL UNIQUE,                    -- '2026-06'
  status          schedule_status NOT NULL DEFAULT 'draft',
  generated_at    TIMESTAMPTZ,
  confirmed_at    TIMESTAMPTZ,
  generation_log  JSONB                                        -- §10 Infeasibility Policy
);
CREATE INDEX idx_schedules_status ON schedules(status);

-- ============================================================
-- schedule_cells — G-05 (DE 포함 enum)
-- ============================================================

CREATE TABLE schedule_cells (
  id                    SERIAL PRIMARY KEY,
  schedule_id           INT NOT NULL REFERENCES schedules(id) ON DELETE CASCADE,
  nurse_id              INT NOT NULL REFERENCES nurses(id),
  date                  DATE NOT NULL,
  shift                 shift_code NOT NULL,
  source                cell_source NOT NULL DEFAULT 'auto',
  modified_by_nurse_id  INT REFERENCES nurses(id),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(schedule_id, nurse_id, date)
);
CREATE INDEX idx_cells_schedule_date ON schedule_cells(schedule_id, date);
CREATE INDEX idx_cells_nurse_date    ON schedule_cells(nurse_id, date);
COMMENT ON COLUMN schedule_cells.shift IS 'G-01/G-05: D/E/N/O/DE — DE는 H-13으로 자동 생성 불가';

-- ============================================================
-- swap_requests
-- ============================================================

CREATE TABLE swap_requests (
  id                    SERIAL PRIMARY KEY,
  schedule_id           INT NOT NULL REFERENCES schedules(id),
  requester_nurse_id    INT NOT NULL REFERENCES nurses(id),
  target_nurse_id       INT NOT NULL REFERENCES nurses(id),
  requester_date        DATE NOT NULL,
  target_date           DATE NOT NULL,
  status                swap_status NOT NULL DEFAULT 'pending',
  reason                TEXT,
  rejected_reason       TEXT,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (requester_nurse_id <> target_nurse_id)
);
CREATE INDEX idx_swaps_status   ON swap_requests(status);
CREATE INDEX idx_swaps_schedule ON swap_requests(schedule_id);

-- ============================================================
-- night_keeper_assignments — K-NN
-- ============================================================

CREATE TABLE night_keeper_assignments (
  id                    SERIAL PRIMARY KEY,
  nurse_id              INT NOT NULL REFERENCES nurses(id) ON DELETE CASCADE,
  year_month            CHAR(7) NOT NULL,
  assigned_by_nurse_id  INT REFERENCES nurses(id),
  reason                TEXT,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(nurse_id, year_month)
);
CREATE INDEX idx_nk_year_month ON night_keeper_assignments(year_month);
COMMENT ON TABLE night_keeper_assignments IS 'K-01~K-05: K-02(3달 연속 금지), K-04(cooldown 3달), K-05(fixed_pattern 충돌 금지)는 API 측 검증';

-- ============================================================
-- ward_settings — single-row 패턴 (id=1만 허용)
-- ============================================================

CREATE TABLE ward_settings (
  id                                   INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  min_d                                INT NOT NULL DEFAULT 3,
  min_e                                INT NOT NULL DEFAULT 2,
  min_n                                INT NOT NULL DEFAULT 2,
  max_consecutive_n                    INT NOT NULL DEFAULT 3,
  min_rest_after_n                     INT NOT NULL DEFAULT 1,    -- H-06 confirmed
  max_consecutive_workdays             INT NOT NULL DEFAULT 5,    -- H-03 confirmed
  balance_off_tolerance                INT NOT NULL DEFAULT 1,
  previous_month_lookback_days         INT NOT NULL DEFAULT 7,    -- H-10
  night_keeper_max_consecutive_months  INT NOT NULL DEFAULT 2,    -- K-02
  night_keeper_cooldown_months         INT NOT NULL DEFAULT 3,    -- K-04
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

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS swap_requests;
DROP TABLE IF EXISTS schedule_cells;
DROP TABLE IF EXISTS schedules;
DROP TABLE IF EXISTS wishes;
DROP TABLE IF EXISTS night_keeper_assignments;
DROP TABLE IF EXISTS nurses;
DROP TABLE IF EXISTS experience_levels;
DROP TABLE IF EXISTS ward_settings;

DROP TYPE IF EXISTS swap_status;
DROP TYPE IF EXISTS cell_source;
DROP TYPE IF EXISTS schedule_status;
DROP TYPE IF EXISTS shift_code;
DROP TYPE IF EXISTS wish_type;
DROP TYPE IF EXISTS fixed_pattern;
DROP TYPE IF EXISTS nurse_role;

-- +goose StatementEnd
