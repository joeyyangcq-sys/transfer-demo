package postgres

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/joeyyang/internal-transfers/migrations"
)

// migrationLockID is an arbitrary app-wide key for the advisory lock.
// migrationLockID 是 advisory lock 用的一个应用级固定 key。
const migrationLockID int64 = 4242

// Migrate applies all *.up.sql files in order. It holds a session-level
// advisory lock so only one instance migrates at a time (safe for multiple
// replicas starting together).
// Migrate 按顺序执行所有 *.up.sql。它持有会话级 advisory lock，
// 保证同一时刻只有一个实例在迁移（多副本同时启动也安全）。
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	// Block until we hold the lock; released on unlock or session end.
	// 阻塞直到拿到锁；在 unlock 或会话结束时释放。
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("acquire advisory lock: %w", err)
	}
	// Best-effort unlock; the lock is also released when the session ends.
	// 尽力解锁；即便失败，会话结束时锁也会自动释放。
	defer func() { _, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", migrationLockID) }()

	files, err := upFiles()
	if err != nil {
		return err
	}
	for _, name := range files {
		sql, err := migrations.FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		if _, err := conn.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}

// upFiles returns the *.up.sql migration file names in sorted order.
// upFiles 返回排好序的 *.up.sql 迁移文件名列表。
func upFiles() ([]string, error) {
	entries, err := migrations.FS.ReadDir(".")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}
