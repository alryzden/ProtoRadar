package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	testPostgresImage           = "postgres:16-alpine"
	testPostgresSetupTimeout    = 60 * time.Second
	testPostgresCleanupTimeout  = 15 * time.Second
	testPostgresDatabase        = "protoradar_test"
	testPostgresUser            = "protoradar"
	testPostgresPassword        = "protoradar_test_password"
	testPostgresConnectionParam = "sslmode=disable"
)

var (
	testPostgresURL       string
	testPostgresAdminPool *pgxpool.Pool
	testPostgresStartErr  error
	testSchemaCounter     atomic.Uint64
)

func TestMain(m *testing.M) {
	code := runPostgresRepositoryTests(m)
	os.Exit(code)
}

func runPostgresRepositoryTests(m *testing.M) int {
	cleanup, err := startSharedTestPostgres()
	if err != nil {
		testPostgresStartErr = err
		return m.Run()
	}

	code := m.Run()
	if err := cleanup(); err != nil {
		fmt.Fprintf(os.Stderr, "postgres test container cleanup: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	return code
}

func startSharedTestPostgres() (func() error, error) {
	setupCtx, cancel := context.WithTimeout(context.Background(), testPostgresSetupTimeout)
	defer cancel()

	container, err := tcpostgres.Run(
		setupCtx,
		testPostgresImage,
		tcpostgres.WithDatabase(testPostgresDatabase),
		tcpostgres.WithUsername(testPostgresUser),
		tcpostgres.WithPassword(testPostgresPassword),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres test container: %w", err)
	}

	cleanup := func() error {
		if testPostgresAdminPool != nil {
			testPostgresAdminPool.Close()
		}
		cleanupCtx, cancel := context.WithTimeout(context.Background(), testPostgresCleanupTimeout)
		defer cancel()
		if err := testcontainers.TerminateContainer(container, testcontainers.StopContext(cleanupCtx)); err != nil {
			return fmt.Errorf("terminate postgres test container: %w", err)
		}
		return nil
	}

	databaseURL, err := container.ConnectionString(setupCtx, testPostgresConnectionParam)
	if err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return nil, errors.Join(
				fmt.Errorf("get postgres test container connection string: %w", err),
				fmt.Errorf("cleanup postgres test container: %w", cleanupErr),
			)
		}
		return nil, fmt.Errorf("get postgres test container connection string: %w", err)
	}
	testPostgresURL = databaseURL

	adminPool, err := pgxpool.New(setupCtx, databaseURL)
	if err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return nil, errors.Join(
				fmt.Errorf("connect postgres test container: %w", err),
				fmt.Errorf("cleanup postgres test container: %w", cleanupErr),
			)
		}
		return nil, fmt.Errorf("connect postgres test container: %w", err)
	}
	testPostgresAdminPool = adminPool

	if err := adminPool.Ping(setupCtx); err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return nil, errors.Join(
				fmt.Errorf("ping postgres test container: %w", err),
				fmt.Errorf("cleanup postgres test container: %w", cleanupErr),
			)
		}
		return nil, fmt.Errorf("ping postgres test container: %w", err)
	}

	return cleanup, nil
}

func newTestDB(t *testing.T) *DB {
	t.Helper()
	return New(newMigratedTestPostgresPool(t))
}

func newMigratedTestPostgresPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	pool := newTestPostgresPool(t)
	migrationFS := os.DirFS(filepath.Join("..", "..", "..", "migrations"))
	if _, err := fs.Stat(migrationFS, "000001_registry.up.sql"); err != nil {
		t.Fatalf("migration fs: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), testPostgresSetupTimeout)
	defer cancel()
	if err := RunMigrations(ctx, pool, migrationFS); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	assertTestPostgresTableExists(t, ctx, pool, gooseVersionTable)
	assertTestPostgresTableMissingInSchema(t, ctx, pool, gooseVersionTable, "public")
	return pool
}

func newTestPostgresPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPostgresStartErr != nil {
		t.Skipf("PostgreSQL Testcontainers unavailable: %v", testPostgresStartErr)
	}
	if testPostgresURL == "" || testPostgresAdminPool == nil {
		t.Fatal("postgres test container was not initialized")
	}

	setupCtx, cancel := context.WithTimeout(context.Background(), testPostgresSetupTimeout)
	defer cancel()

	schema := newIsolatedTestSchema(t, testPostgresAdminPool)

	cfg, err := pgxpool.ParseConfig(testPostgresURL)
	if err != nil {
		t.Fatal("parse postgres test container connection string")
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema

	pool, err := pgxpool.NewWithConfig(setupCtx, cfg)
	if err != nil {
		t.Fatal("connect isolated postgres test schema")
	}
	t.Cleanup(pool.Close)
	assertTestPostgresCurrentSchema(t, setupCtx, pool, schema)
	return pool
}

func newIsolatedTestSchema(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()

	setupCtx, cancel := context.WithTimeout(context.Background(), testPostgresSetupTimeout)
	defer cancel()

	schema := fmt.Sprintf("protoradar_test_%06d", testSchemaCounter.Add(1))
	if _, err := pool.Exec(setupCtx, `CREATE SCHEMA `+quoteIdent(schema)); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), testPostgresCleanupTimeout)
		defer cancel()
		if _, err := pool.Exec(cleanupCtx, `DROP SCHEMA IF EXISTS `+quoteIdent(schema)+` CASCADE`); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})
	return schema
}

func assertTestPostgresCurrentSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want string) {
	t.Helper()

	got := currentTestPostgresSchema(t, ctx, pool)
	if got != want {
		t.Fatalf("current schema = %q, want %q", got, want)
	}
}

func currentTestPostgresSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()

	var schema string
	if err := pool.QueryRow(ctx, `SELECT current_schema()`).Scan(&schema); err != nil {
		t.Fatalf("check current schema: %v", err)
	}
	return schema
}

func assertTestPostgresTableExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) {
	t.Helper()

	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = current_schema()
				AND table_name = $1
		)
	`, table).Scan(&exists); err != nil {
		t.Fatalf("check table %s: %v", table, err)
	}
	if !exists {
		t.Fatalf("table %s does not exist", table)
	}
}

func assertTestPostgresTableExistsInSchema(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	table string,
	schema string,
) {
	t.Helper()

	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = $1
				AND table_name = $2
		)
	`, schema, table).Scan(&exists); err != nil {
		t.Fatalf("check table %s.%s: %v", schema, table, err)
	}
	if !exists {
		t.Fatalf("table %s.%s does not exist", schema, table)
	}
}

func assertTestPostgresTableMissingInSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string, schema string) {
	t.Helper()

	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = $1
				AND table_name = $2
		)
	`, schema, table).Scan(&exists); err != nil {
		t.Fatalf("check table %s.%s: %v", schema, table, err)
	}
	if exists {
		t.Fatalf("table %s.%s exists, want missing", schema, table)
	}
}

func assertTestPostgresSchemaMissing(t *testing.T, ctx context.Context, schema string) {
	t.Helper()
	if testPostgresAdminPool == nil {
		t.Fatal("postgres test admin pool was not initialized")
	}

	var exists bool
	if err := testPostgresAdminPool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.schemata
			WHERE schema_name = $1
		)
	`, schema).Scan(&exists); err != nil {
		t.Fatalf("check schema %s: %v", schema, err)
	}
	if exists {
		t.Fatalf("schema %s exists, want dropped", schema)
	}
}

func TestNewTestPostgresPoolReturnsWorkingPool(t *testing.T) {
	ctx := context.Background()
	pool := newTestPostgresPool(t)

	var one int
	if err := pool.QueryRow(ctx, `SELECT 1`).Scan(&one); err != nil {
		t.Fatalf("query postgres test pool: %v", err)
	}
	if one != 1 {
		t.Fatalf("query result = %d, want 1", one)
	}
}

func TestNewMigratedTestPostgresPoolAppliesMigrations(t *testing.T) {
	ctx := context.Background()
	pool := newMigratedTestPostgresPool(t)

	assertTestPostgresTableExists(t, ctx, pool, "modules")
	assertTestPostgresTableExists(t, ctx, pool, gooseVersionTable)
	assertTestPostgresTableMissingInSchema(t, ctx, pool, gooseVersionTable, "public")
}

func TestMigratedTestPostgresPoolsUseIsolatedSchemas(t *testing.T) {
	ctx := context.Background()
	first := newMigratedTestPostgresPool(t)
	second := newMigratedTestPostgresPool(t)

	firstSchema := currentTestPostgresSchema(t, ctx, first)
	secondSchema := currentTestPostgresSchema(t, ctx, second)
	if firstSchema == secondSchema {
		t.Fatalf("test schemas are equal: %s", firstSchema)
	}

	const moduleName = "schema-isolation-module"
	if _, err := first.Exec(ctx, `
		INSERT INTO modules (id, name, description, repository_url, created_at, updated_at)
		VALUES (gen_random_uuid(), $1, '', '', now(), now())
	`, moduleName); err != nil {
		t.Fatalf("insert module in first schema: %v", err)
	}

	var firstCount int
	if err := first.QueryRow(ctx, `SELECT count(*) FROM modules WHERE name = $1`, moduleName).Scan(&firstCount); err != nil {
		t.Fatalf("count module in first schema: %v", err)
	}
	if firstCount != 1 {
		t.Fatalf("first schema module count = %d, want 1", firstCount)
	}

	var secondCount int
	if err := second.QueryRow(ctx, `SELECT count(*) FROM modules WHERE name = $1`, moduleName).Scan(&secondCount); err != nil {
		t.Fatalf("count module in second schema: %v", err)
	}
	if secondCount != 0 {
		t.Fatalf("second schema module count = %d, want 0", secondCount)
	}
}

func TestMigratedTestPostgresPoolCreatesTablesInCurrentSchema(t *testing.T) {
	ctx := context.Background()
	pool := newMigratedTestPostgresPool(t)
	schema := currentTestPostgresSchema(t, ctx, pool)

	for _, table := range []string{
		gooseVersionTable,
		"modules",
		"module_versions",
		"artifacts",
		"api_tokens",
		"outbox_records",
	} {
		assertTestPostgresTableExistsInSchema(t, ctx, pool, table, schema)
		assertTestPostgresTableMissingInSchema(t, ctx, pool, table, "public")
	}
}

func TestTestPostgresPoolCleanupDropsIsolatedSchema(t *testing.T) {
	ctx := context.Background()
	var schema string

	t.Run("create schema", func(t *testing.T) {
		pool := newTestPostgresPool(t)
		schema = currentTestPostgresSchema(t, ctx, pool)
	})

	if schema == "" {
		t.Skip("postgres test pool was skipped before schema creation")
	}
	assertTestPostgresSchemaMissing(t, ctx, schema)
}
