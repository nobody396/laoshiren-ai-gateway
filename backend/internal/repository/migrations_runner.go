package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/migrations"
)

// schemaMigrationsTableDDL 定义迁移记录表的 DDL。
// 该表用于跟踪已应用的迁移文件及其校验和。
// - filename: 迁移文件名，作为主键唯一标识每个迁移
// - checksum: 文件内容的 SHA256 哈希值，用于检测迁移文件是否被篡改
// - applied_at: 迁移应用时间戳
const schemaMigrationsTableDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
	filename   TEXT PRIMARY KEY,
	checksum   TEXT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

const atlasSchemaRevisionsTableDDL = `
CREATE TABLE IF NOT EXISTS atlas_schema_revisions (
	version TEXT PRIMARY KEY,
	description TEXT NOT NULL,
	type INTEGER NOT NULL,
	applied INTEGER NOT NULL DEFAULT 0,
	total INTEGER NOT NULL DEFAULT 0,
	executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	execution_time BIGINT NOT NULL DEFAULT 0,
	error TEXT NULL,
	error_stmt TEXT NULL,
	hash TEXT NOT NULL DEFAULT '',
	partial_hashes TEXT[] NULL,
	operator_version TEXT NULL
);
`

// migrationsAdvisoryLockID 是用于序列化迁移操作的 PostgreSQL Advisory Lock ID。
// 在多实例部署场景下，该锁确保同一时间只有一个实例执行迁移。
// 任何稳定的 int64 值都可以，只要不与同一数据库中的其他锁冲突即可。
const migrationsAdvisoryLockID int64 = 694208311321144027
const migrationsLockRetryInterval = 500 * time.Millisecond
const migrationsUnlockTimeout = 5 * time.Second
const nonTransactionalMigrationSuffix = "_notx.sql"

var legacyMigrationsWithDownSections = map[string]struct{}{
	"019_migrate_wechat_to_attributes.sql": {},
	"024_add_gemini_tier_id.sql":           {},
	"037_ops_alert_silences.sql":           {},
}

type migrationExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type migrationChecksumCompatibilityRule struct {
	fileChecksum       string
	acceptedDBChecksum map[string]struct{}
}

// migrationChecksumCompatibilityRules 仅用于兼容历史上误修改过的迁移文件 checksum。
// 规则必须同时匹配「迁移名 + 当前文件 checksum + 历史库 checksum」才会放行，避免放宽全局校验。
var migrationChecksumCompatibilityRules = map[string]migrationChecksumCompatibilityRule{
	"054_drop_legacy_cache_columns.sql": {
		fileChecksum: "82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d",
		acceptedDBChecksum: map[string]struct{}{
			"182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4": {},
		},
	},
	"061_add_usage_log_request_type.sql": {
		fileChecksum: "66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c",
		acceptedDBChecksum: map[string]struct{}{
			"08a248652cbab7cfde147fc6ef8cda464f2477674e20b718312faa252e0481c0": {},
			"222b4a09c797c22e5922b6b172327c824f5463aaa8760e4f621bc5c22e2be0f3": {},
		},
	},
	"111_add_commission_consumption_idempotency.sql": {
		fileChecksum: "c4f13930e5049baeec9b80757c9dcb8420d3e85a16a99aeba6fa55aecf7574e2",
		acceptedDBChecksum: map[string]struct{}{
			"bbbd704d99e4700da6fed36f0eccf05afbc71319cbda1cfc4e8ccbd7aa1f5dda": {},
		},
	},
}

// ApplyMigrations 将嵌入的 SQL 迁移文件应用到指定的数据库。
//
// 该函数可以在每次应用启动时安全调用：
// - 已应用的迁移会被自动跳过（通过校验 filename 判断）
// - 如果迁移文件内容被修改（checksum 不匹配），会返回错误
// - 使用 PostgreSQL Advisory Lock 确保多实例并发安全
//
// 参数：
//   - ctx: 上下文，用于超时控制和取消
//   - db: 数据库连接
//
// 返回：
//   - error: 迁移过程中的任何错误
func ApplyMigrations(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("nil sql db")
	}
	return applyMigrationsFS(ctx, db, migrations.FS)
}

// applyMigrationsFS 是迁移执行的核心实现。
// 它从指定的文件系统读取 SQL 迁移文件并按顺序应用。
//
// 迁移执行流程：
//  1. 获取 PostgreSQL Advisory Lock，防止多实例并发迁移
//  2. 确保 schema_migrations 表存在
//  3. 按文件名排序读取所有 .sql 文件
//  4. 对于每个迁移文件：
//     - 计算文件内容的 SHA256 校验和
//     - 检查该迁移是否已应用（通过 filename 查询）
//     - 如果已应用，验证校验和是否匹配
//     - 如果未应用，在事务中执行迁移并记录
//  5. 释放 Advisory Lock
//
// 参数：
//   - ctx: 上下文
//   - db: 数据库连接
//   - fsys: 包含迁移文件的文件系统（通常是 embed.FS）
func applyMigrationsFS(ctx context.Context, db *sql.DB, fsys fs.FS) (resultErr error) {
	if db == nil {
		return errors.New("nil sql db")
	}

	// PostgreSQL session advisory locks are connection-scoped. Pin one physical
	// connection for lock acquisition, every migration statement, and unlock.
	// Using *sql.DB for those operations can acquire and release on different
	// pooled sessions, leaving the migration sequence effectively unlocked.
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("reserve migrations connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	// 获取分布式锁，确保多实例部署时只有一个实例执行迁移。
	// 这是 PostgreSQL 特有的 Advisory Lock 机制。
	if err := pgAdvisoryLock(ctx, conn); err != nil {
		return err
	}
	defer func() {
		// 无论迁移是否成功，都在获取锁的同一 session 上释放它。
		// 使用独立且有界的 context，避免原 ctx 取消后跳过 unlock，
		// 也避免数据库异常时 shutdown 永久阻塞。
		unlockCtx, cancel := context.WithTimeout(context.Background(), migrationsUnlockTimeout)
		defer cancel()
		if err := pgAdvisoryUnlock(unlockCtx, conn); err != nil {
			// Returning a session that still owns the advisory lock to the pool
			// would deadlock future migrations. driver.ErrBadConn forces
			// database/sql to discard the physical connection instead.
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			resultErr = errors.Join(resultErr, err)
		}
	}()

	// 创建迁移记录表（如果不存在）。
	// 该表记录所有已应用的迁移及其校验和。
	if _, err := conn.ExecContext(ctx, schemaMigrationsTableDDL); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	// 自动对齐 Atlas 基线（如果检测到 legacy schema_migrations 且缺失 atlas_schema_revisions）。
	if err := ensureAtlasBaselineAligned(ctx, conn, fsys); err != nil {
		return err
	}

	// 获取所有 .sql 迁移文件并按文件名排序。
	// 命名规范：使用零填充数字前缀（如 001_init.sql, 002_add_users.sql）。
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(files) // 确保按文件名顺序执行迁移

	for _, name := range files {
		// 读取迁移文件内容
		contentBytes, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		content := strings.TrimSpace(string(contentBytes))
		if content == "" {
			continue // 跳过空文件
		}

		if err := validateMigrationDirectionPolicy(name, content); err != nil {
			return fmt.Errorf("validate migration %s direction: %w", name, err)
		}
		executableContent, err := extractMigrationUpSQL(content)
		if err != nil {
			return fmt.Errorf("parse migration %s direction: %w", name, err)
		}

		// 计算文件内容的 SHA256 校验和，用于检测文件是否被修改。
		// 这是一种防篡改机制：如果有人修改了已应用的迁移文件，系统会拒绝启动。
		sum := sha256.Sum256([]byte(content))
		checksum := hex.EncodeToString(sum[:])

		// 检查该迁移是否已经应用
		var existing string
		rowErr := conn.QueryRowContext(ctx, "SELECT checksum FROM schema_migrations WHERE filename = $1", name).Scan(&existing)
		if rowErr == nil {
			// 迁移已应用，验证校验和是否匹配
			if existing != checksum {
				// 兼容特定历史误改场景（仅白名单规则），其余仍保持严格不可变约束。
				if isMigrationChecksumCompatible(name, existing, checksum) {
					continue
				}
				// 校验和不匹配意味着迁移文件在应用后被修改，这是危险的。
				// 正确的做法是创建新的迁移文件来进行变更。
				return fmt.Errorf(
					"migration %s checksum mismatch (db=%s file=%s)\n"+
						"This means the migration file was modified after being applied to the database.\n"+
						"Solutions:\n"+
						"  1. Revert to original: git log --oneline -- migrations/%s && git checkout <commit> -- migrations/%s\n"+
						"  2. For new changes, create a new migration file instead of modifying existing ones\n"+
						"Note: Modifying applied migrations breaks the immutability principle and can cause inconsistencies across environments",
					name, existing, checksum, name, name,
				)
			}
			continue // 迁移已应用且校验和匹配，跳过
		}
		if !errors.Is(rowErr, sql.ErrNoRows) {
			return fmt.Errorf("check migration %s: %w", name, rowErr)
		}

		nonTx, err := validateMigrationExecutionMode(name, executableContent)
		if err != nil {
			return fmt.Errorf("validate migration %s: %w", name, err)
		}

		if nonTx {
			// *_notx.sql：用于 CREATE/DROP INDEX CONCURRENTLY 场景，必须非事务执行。
			// 逐条语句执行，避免将多条 CONCURRENTLY 语句放入同一个隐式事务块。
			statements := splitSQLStatements(executableContent)
			for i, stmt := range statements {
				trimmed := strings.TrimSpace(stmt)
				if trimmed == "" {
					continue
				}
				if stripSQLLineComment(trimmed) == "" {
					continue
				}
				if _, err := conn.ExecContext(ctx, trimmed); err != nil {
					return fmt.Errorf("apply migration %s (non-tx statement %d): %w", name, i+1, err)
				}
			}
			if _, err := conn.ExecContext(ctx, "INSERT INTO schema_migrations (filename, checksum) VALUES ($1, $2)", name, checksum); err != nil {
				return fmt.Errorf("record migration %s (non-tx): %w", name, err)
			}
			continue
		}

		// 默认迁移在事务中执行，确保原子性：要么完全成功，要么完全回滚。
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}

		// 执行迁移 SQL
		if _, err := tx.ExecContext(ctx, executableContent); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		// 记录迁移已完成，保存文件名和校验和
		if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (filename, checksum) VALUES ($1, $2)", name, checksum); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		// 提交事务
		if err := tx.Commit(); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}

	return nil
}

func ensureAtlasBaselineAligned(ctx context.Context, db migrationExecutor, fsys fs.FS) error {
	hasLegacy, err := tableExists(ctx, db, "schema_migrations")
	if err != nil {
		return fmt.Errorf("check schema_migrations: %w", err)
	}
	if !hasLegacy {
		return nil
	}

	hasAtlas, err := tableExists(ctx, db, "atlas_schema_revisions")
	if err != nil {
		return fmt.Errorf("check atlas_schema_revisions: %w", err)
	}
	if !hasAtlas {
		if _, err := db.ExecContext(ctx, atlasSchemaRevisionsTableDDL); err != nil {
			return fmt.Errorf("create atlas_schema_revisions: %w", err)
		}
	}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM atlas_schema_revisions").Scan(&count); err != nil {
		return fmt.Errorf("count atlas_schema_revisions: %w", err)
	}
	if count > 0 {
		return nil
	}

	version, description, hash, err := latestMigrationBaseline(fsys)
	if err != nil {
		return fmt.Errorf("atlas baseline version: %w", err)
	}

	if _, err := db.ExecContext(ctx, `
		INSERT INTO atlas_schema_revisions (version, description, type, applied, total, executed_at, execution_time, hash)
		VALUES ($1, $2, $3, 0, 0, NOW(), 0, $4)
	`, version, description, 1, hash); err != nil {
		return fmt.Errorf("insert atlas baseline: %w", err)
	}
	return nil
}

func tableExists(ctx context.Context, db migrationExecutor, tableName string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, tableName).Scan(&exists)
	return exists, err
}

func latestMigrationBaseline(fsys fs.FS) (string, string, string, error) {
	files, err := fs.Glob(fsys, "*.sql")
	if err != nil {
		return "", "", "", err
	}
	if len(files) == 0 {
		return "baseline", "baseline", "", nil
	}
	sort.Strings(files)
	name := files[len(files)-1]
	contentBytes, err := fs.ReadFile(fsys, name)
	if err != nil {
		return "", "", "", err
	}
	content := strings.TrimSpace(string(contentBytes))
	sum := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(sum[:])
	version := strings.TrimSuffix(name, ".sql")
	return version, version, hash, nil
}

func isMigrationChecksumCompatible(name, dbChecksum, fileChecksum string) bool {
	rule, ok := migrationChecksumCompatibilityRules[name]
	if !ok {
		return false
	}
	if rule.fileChecksum != fileChecksum {
		return false
	}
	_, ok = rule.acceptedDBChecksum[dbChecksum]
	return ok
}

type migrationDirection int

const (
	migrationDirectionNone migrationDirection = iota
	migrationDirectionUp
	migrationDirectionDown
)

// validateMigrationDirectionPolicy prevents new rollback sections from being
// added to the embedded migration stream. Three historical files are allowed
// because their checksums are immutable; extractMigrationUpSQL makes them safe
// by executing only their Up sections.
func validateMigrationDirectionPolicy(name, content string) error {
	if !hasGooseMarker(content, "Down") {
		return nil
	}
	if _, ok := legacyMigrationsWithDownSections[name]; !ok {
		return errors.New("down sections are forbidden in new migrations; create a forward-only compensating migration")
	}
	return nil
}

// extractMigrationUpSQL returns the executable Up portion of a migration.
// Files without Goose markers retain the repository's historical behavior.
// Marker-bearing files are validated strictly so malformed direction blocks
// fail before any SQL is executed or recorded.
func extractMigrationUpSQL(content string) (string, error) {
	if !hasAnyGooseDirective(content) {
		return content, nil
	}

	lines := strings.Split(content, "\n")
	direction := migrationDirectionNone
	seenUp := false
	seenDown := false
	statementOpen := false
	upLines := make([]string, 0, len(lines))
	preface := make([]string, 0)

	for lineNo, line := range lines {
		marker, isDirective := parseGooseDirective(line)
		if !isDirective {
			switch direction {
			case migrationDirectionNone:
				preface = append(preface, line)
			case migrationDirectionUp:
				upLines = append(upLines, line)
			case migrationDirectionDown:
				// Down SQL is intentionally validated structurally but never executed.
			}
			continue
		}

		switch marker {
		case "Up":
			if seenUp || direction != migrationDirectionNone || statementOpen {
				return "", fmt.Errorf("line %d: duplicate or misplaced Up marker", lineNo+1)
			}
			executable, err := containsExecutableSQL(strings.Join(preface, "\n"))
			if err != nil {
				return "", fmt.Errorf("line %d: invalid content before Up marker: %w", lineNo+1, err)
			}
			if executable {
				return "", fmt.Errorf("line %d: executable SQL before Up marker", lineNo+1)
			}
			seenUp = true
			direction = migrationDirectionUp
		case "Down":
			if !seenUp || seenDown || direction != migrationDirectionUp || statementOpen {
				return "", fmt.Errorf("line %d: duplicate or misplaced Down marker", lineNo+1)
			}
			seenDown = true
			direction = migrationDirectionDown
		case "StatementBegin":
			if direction == migrationDirectionNone || statementOpen {
				return "", fmt.Errorf("line %d: misplaced StatementBegin marker", lineNo+1)
			}
			statementOpen = true
		case "StatementEnd":
			if direction == migrationDirectionNone || !statementOpen {
				return "", fmt.Errorf("line %d: misplaced StatementEnd marker", lineNo+1)
			}
			statementOpen = false
		default:
			return "", fmt.Errorf("line %d: unsupported Goose marker %q", lineNo+1, marker)
		}
	}

	if !seenUp {
		return "", errors.New("goose markers present without an Up section")
	}
	if statementOpen {
		return "", errors.New("unclosed Goose StatementBegin block")
	}

	upSQL := strings.TrimSpace(strings.Join(upLines, "\n"))
	executable, err := containsExecutableSQL(upSQL)
	if err != nil {
		return "", fmt.Errorf("invalid Goose Up section: %w", err)
	}
	if !executable {
		return "", errors.New("goose Up section contains no executable SQL")
	}
	return upSQL, nil
}

func hasAnyGooseDirective(content string) bool {
	for _, line := range strings.Split(content, "\n") {
		if _, ok := parseGooseDirective(line); ok {
			return true
		}
	}
	return false
}

// containsExecutableSQL distinguishes comment-only regions from executable
// SQL. It is deliberately conservative: outside line or nested block comments,
// any non-whitespace byte counts as executable content.
func containsExecutableSQL(content string) (bool, error) {
	blockDepth := 0
	for i := 0; i < len(content); {
		if blockDepth > 0 {
			switch {
			case i+1 < len(content) && content[i:i+2] == "/*":
				blockDepth++
				i += 2
			case i+1 < len(content) && content[i:i+2] == "*/":
				blockDepth--
				i += 2
			default:
				i++
			}
			continue
		}

		switch {
		case i+1 < len(content) && content[i:i+2] == "--":
			i += 2
			for i < len(content) && content[i] != '\n' {
				i++
			}
		case i+1 < len(content) && content[i:i+2] == "/*":
			blockDepth = 1
			i += 2
		case content[i] == ' ' || content[i] == '\t' || content[i] == '\r' || content[i] == '\n':
			i++
		default:
			return true, nil
		}
	}
	if blockDepth != 0 {
		return false, errors.New("unterminated block comment")
	}
	return false, nil
}

func hasGooseMarker(content, marker string) bool {
	for _, line := range strings.Split(content, "\n") {
		parsed, ok := parseGooseDirective(line)
		if ok && parsed == marker {
			return true
		}
	}
	return false
}

// parseGooseDirective recognizes the directive family even when whitespace is
// non-canonical. Unknown or misspelled directives then fail closed instead of
// letting rollback SQL be mistaken for an ordinary unmarked migration.
func parseGooseDirective(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "--") {
		return "", false
	}
	comment := strings.TrimSpace(strings.TrimPrefix(trimmed, "--"))
	fields := strings.Fields(comment)
	if len(fields) == 0 || !strings.EqualFold(fields[0], "+goose") {
		return "", false
	}
	if len(fields) == 1 {
		return "", true
	}
	return strings.Join(fields[1:], " "), true
}

func validateMigrationExecutionMode(name, content string) (bool, error) {
	normalizedName := strings.ToLower(strings.TrimSpace(name))
	upperContent := strings.ToUpper(content)
	nonTx := strings.HasSuffix(normalizedName, nonTransactionalMigrationSuffix)

	if !nonTx {
		if strings.Contains(upperContent, "CONCURRENTLY") {
			return false, errors.New("CONCURRENTLY statements must be placed in *_notx.sql migrations")
		}
		return false, nil
	}

	if strings.Contains(upperContent, "BEGIN") || strings.Contains(upperContent, "COMMIT") || strings.Contains(upperContent, "ROLLBACK") {
		return false, errors.New("*_notx.sql must not contain transaction control statements (BEGIN/COMMIT/ROLLBACK)")
	}

	statements := splitSQLStatements(content)
	for _, stmt := range statements {
		normalizedStmt := strings.ToUpper(stripSQLLineComment(strings.TrimSpace(stmt)))
		if normalizedStmt == "" {
			continue
		}

		if strings.Contains(normalizedStmt, "CONCURRENTLY") {
			isCreateIndex := strings.Contains(normalizedStmt, "CREATE") && strings.Contains(normalizedStmt, "INDEX")
			isDropIndex := strings.Contains(normalizedStmt, "DROP") && strings.Contains(normalizedStmt, "INDEX")
			if !isCreateIndex && !isDropIndex {
				return false, errors.New("*_notx.sql currently only supports CREATE/DROP INDEX CONCURRENTLY statements")
			}
			if isCreateIndex && !strings.Contains(normalizedStmt, "IF NOT EXISTS") {
				return false, errors.New("CREATE INDEX CONCURRENTLY in *_notx.sql must include IF NOT EXISTS for idempotency")
			}
			if isDropIndex && !strings.Contains(normalizedStmt, "IF EXISTS") {
				return false, errors.New("DROP INDEX CONCURRENTLY in *_notx.sql must include IF EXISTS for idempotency")
			}
			continue
		}

		return false, errors.New("*_notx.sql must not mix non-CONCURRENTLY SQL statements")
	}

	return true, nil
}

func splitSQLStatements(content string) []string {
	parts := strings.Split(content, ";")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func stripSQLLineComment(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if idx := strings.Index(line, "--"); idx >= 0 {
			lines[i] = line[:idx]
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// pgAdvisoryLock 获取 PostgreSQL Advisory Lock。
// Advisory Lock 是一种轻量级的锁机制，不与任何特定的数据库对象关联。
// 它非常适合用于应用层面的分布式锁场景，如迁移序列化。
func pgAdvisoryLock(ctx context.Context, db migrationExecutor) error {
	ticker := time.NewTicker(migrationsLockRetryInterval)
	defer ticker.Stop()

	for {
		var locked bool
		if err := db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationsAdvisoryLockID).Scan(&locked); err != nil {
			return fmt.Errorf("acquire migrations lock: %w", err)
		}
		if locked {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("acquire migrations lock: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

// pgAdvisoryUnlock 释放 PostgreSQL Advisory Lock。
// 必须在获取锁后确保释放，否则会阻塞其他实例的迁移操作。
func pgAdvisoryUnlock(ctx context.Context, db migrationExecutor) error {
	var unlocked bool
	err := db.QueryRowContext(ctx, "SELECT pg_advisory_unlock($1)", migrationsAdvisoryLockID).Scan(&unlocked)
	if err != nil {
		return fmt.Errorf("release migrations lock: %w", err)
	}
	if !unlocked {
		return errors.New("release migrations lock: current PostgreSQL session does not own the lock")
	}
	return nil
}
