package postgres

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRunMigrationsUsesGooseWithEmbeddedSplitMigrationFiles(t *testing.T) {
	ctx := context.Background()
	pool := newTestPostgresPool(t)
	migrations := fstest.MapFS{
		"000001_create_widgets.up.sql": {
			Data: []byte(`-- +goose Up
-- +goose StatementBegin
CREATE TABLE migration_widgets (
	id BIGINT PRIMARY KEY,
	name TEXT NOT NULL
);
-- +goose StatementEnd
`),
		},
		"000001_create_widgets.down.sql": {
			Data: []byte(`-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS migration_widgets;
-- +goose StatementEnd
`),
		},
		"000002_add_widget_description.up.sql": {
			Data: []byte(`-- +goose Up
-- +goose StatementBegin
ALTER TABLE migration_widgets
	ADD COLUMN description TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd
`),
		},
		"000002_add_widget_description.down.sql": {
			Data: []byte(`-- +goose Down
-- +goose StatementBegin
ALTER TABLE migration_widgets
	DROP COLUMN IF EXISTS description;
-- +goose StatementEnd
`),
		},
	}

	if err := RunMigrations(ctx, pool, migrations); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if err := RunMigrations(ctx, pool, migrations); err != nil {
		t.Fatalf("rerun migrations: %v", err)
	}

	assertMigrationColumnExists(t, ctx, pool, "migration_widgets", "description")
	assertMigrationTableExists(t, ctx, pool, gooseVersionTable)
	assertMigrationTableMissing(t, ctx, pool, "schema_migrations")
	assertGooseVersionExists(t, ctx, pool, 1)
	assertGooseVersionExists(t, ctx, pool, 2)
}

func TestRunMigrationsDoesNotMarkFailedMigrationApplied(t *testing.T) {
	ctx := context.Background()
	pool := newTestPostgresPool(t)
	migrations := fstest.MapFS{
		"000001_create_widgets.up.sql": {
			Data: []byte(`-- +goose Up
CREATE TABLE migration_widgets (
	id BIGINT PRIMARY KEY
);
`),
		},
		"000002_fail_after_ddl.up.sql": {
			Data: []byte(`-- +goose Up
-- +goose StatementBegin
CREATE TABLE migration_failed_marker (
	id BIGINT PRIMARY KEY
);
SELECT * FROM missing_table_for_migration_failure;
-- +goose StatementEnd
`),
		},
	}

	err := RunMigrations(ctx, pool, migrations)
	if err == nil {
		t.Fatal("run migrations error = nil, want failure")
	}
	if !strings.Contains(err.Error(), "run postgres migrations") {
		t.Fatalf("error = %v, want migration context", err)
	}

	assertGooseVersionExists(t, ctx, pool, 1)
	assertGooseVersionMissing(t, ctx, pool, 2)
	assertMigrationTableMissing(t, ctx, pool, "migration_failed_marker")
}

func TestRunMigrationsDoesNotCreateLegacySchemaMigrations(t *testing.T) {
	ctx := context.Background()
	pool := newTestPostgresPool(t)
	migrations := fstest.MapFS{
		"000001_create_widgets.up.sql": {
			Data: []byte(`-- +goose Up
CREATE TABLE migration_widgets (
	id BIGINT PRIMARY KEY
);
`),
		},
		"000002_add_widget_name.up.sql": {
			Data: []byte(`-- +goose Up
ALTER TABLE migration_widgets
	ADD COLUMN name TEXT NOT NULL DEFAULT '';
`),
		},
	}

	if err := RunMigrations(ctx, pool, migrations); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	assertMigrationColumnExists(t, ctx, pool, "migration_widgets", "name")
	assertMigrationTableExists(t, ctx, pool, gooseVersionTable)
	assertMigrationTableMissing(t, ctx, pool, "schema_migrations")
	assertGooseVersionExists(t, ctx, pool, 1)
	assertGooseVersionExists(t, ctx, pool, 2)
}

func TestGooseMigrationFSCombinesUpAndDownFiles(t *testing.T) {
	migrations := fstest.MapFS{
		"000001_create_widgets.up.sql": {
			Data: []byte("-- +goose Up\nCREATE TABLE migration_widgets (id BIGINT PRIMARY KEY);\n"),
		},
		"000001_create_widgets.down.sql": {
			Data: []byte("-- +goose Down\nDROP TABLE IF EXISTS migration_widgets;\n"),
		},
	}

	gooseFS, err := newGooseMigrationFS(migrations)
	if err != nil {
		t.Fatalf("new goose fs: %v", err)
	}
	names, err := fs.Glob(gooseFS, "*.sql")
	if err != nil {
		t.Fatalf("glob goose fs: %v", err)
	}
	if len(names) != 1 || names[0] != "000001_create_widgets.sql" {
		t.Fatalf("goose names = %#v", names)
	}
	body, err := fs.ReadFile(gooseFS, names[0])
	if err != nil {
		t.Fatalf("read goose file: %v", err)
	}
	if got := string(body); !strings.Contains(got, "-- +goose Up") || !strings.Contains(got, "-- +goose Down") {
		t.Fatalf("combined migration does not include up and down annotations:\n%s", got)
	}
}

func assertMigrationColumnExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string, column string) {
	t.Helper()
	var exists bool
	if err := pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema = current_schema()
				AND table_name = $1
				AND column_name = $2
		)
	`, table, column).Scan(&exists); err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	if !exists {
		t.Fatalf("column %s.%s does not exist", table, column)
	}
}

func assertMigrationTableMissing(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) {
	t.Helper()
	assertMigrationTablePresence(t, ctx, pool, table, false)
}

func assertMigrationTableExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) {
	t.Helper()
	assertMigrationTablePresence(t, ctx, pool, table, true)
}

func assertMigrationTablePresence(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string, want bool) {
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
	if exists != want {
		t.Fatalf("table %s exists = %v, want %v", table, exists, want)
	}
}

func assertGooseVersionExists(t *testing.T, ctx context.Context, pool *pgxpool.Pool, version int64) {
	t.Helper()
	assertVersionPresence(t, ctx, pool, gooseVersionTable, "version_id", version, true)
}

func assertGooseVersionMissing(t *testing.T, ctx context.Context, pool *pgxpool.Pool, version int64) {
	t.Helper()
	assertVersionPresence(t, ctx, pool, gooseVersionTable, "version_id", version, false)
}

func assertVersionPresence(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	table string,
	column string,
	version int64,
	want bool,
) {
	t.Helper()
	var exists bool
	query := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE %s = $1)`, table, column)
	if err := pool.QueryRow(ctx, query, version).Scan(&exists); err != nil {
		t.Fatalf("check migration version %s.%s=%d: %v", table, column, version, err)
	}
	if exists != want {
		t.Fatalf("migration version %s.%s=%d exists = %v, want %v", table, column, version, exists, want)
	}
}
