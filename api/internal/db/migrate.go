// Package db — goose 마이그레이션 runner.
// Design Ref: §11.2 — migration은 goose로 관리.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// MigrateUp — 컨테이너 부팅 시 호출 (옵션).
//
// migrations 디렉토리는 빌드 시 컨테이너의 /migrations에 복사됨 (Dockerfile).
func MigrateUp(ctx context.Context, deps *Deps) error {
	if os.Getenv("SKIP_MIGRATE") == "1" {
		return nil
	}
	db := stdlib.OpenDBFromPool(deps.PG)
	defer db.Close()
	goose.SetBaseFS(nil)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	dir := envOr("MIGRATIONS_DIR", "/migrations")
	if err := goose.UpContext(ctx, db, dir); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// 사용 안 함 — 컴파일 가드: stdlib import가 안 쓰이면 빌드 에러 방지.
var _ = sql.ErrNoRows
