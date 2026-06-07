package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
)

const gooseVersionTable = goose.DefaultTablename

func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrations fs.FS) (err error) {
	if pool == nil {
		return errors.New("postgres migrations: pool is nil")
	}
	if migrations == nil {
		return errors.New("postgres migrations: migrations filesystem is nil")
	}

	gooseFS, err := newGooseMigrationFS(migrations)
	if err != nil {
		return fmt.Errorf("prepare postgres migrations: %w", err)
	}
	if len(gooseFS.files) == 0 {
		return nil
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close postgres migration sql handle: %w", closeErr))
		}
	}()

	store, err := database.NewStore(database.DialectPostgres, gooseVersionTable)
	if err != nil {
		return fmt.Errorf("create postgres migration store: %w", err)
	}
	provider, err := goose.NewProvider("", sqlDB, gooseFS, goose.WithStore(store), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return fmt.Errorf("create postgres migration provider: %w", err)
	}
	if _, err := provider.GetDBVersion(ctx); err != nil {
		return fmt.Errorf("initialize postgres migration state: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("run postgres migrations: %w", err)
	}
	return nil
}

type gooseMigrationFS struct {
	files map[string]string
}

func newGooseMigrationFS(migrations fs.FS) (gooseMigrationFS, error) {
	upFiles, err := fs.Glob(migrations, "*.up.sql")
	if err != nil {
		return gooseMigrationFS{}, err
	}
	sort.Strings(upFiles)

	files := make(map[string]string, len(upFiles))
	for _, upName := range upFiles {
		upBody, err := fs.ReadFile(migrations, upName)
		if err != nil {
			return gooseMigrationFS{}, fmt.Errorf("read up migration %s: %w", upName, err)
		}

		combinedName := strings.TrimSuffix(path.Base(upName), ".up.sql") + ".sql"
		combined := string(upBody)
		downName := strings.TrimSuffix(upName, ".up.sql") + ".down.sql"
		if downBody, err := fs.ReadFile(migrations, downName); err == nil {
			combined += "\n\n" + string(downBody)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return gooseMigrationFS{}, fmt.Errorf("read down migration %s: %w", downName, err)
		}
		files[combinedName] = combined
	}
	return gooseMigrationFS{files: files}, nil
}

func (m gooseMigrationFS) Open(name string) (fs.File, error) {
	cleanName := strings.TrimPrefix(path.Clean(name), "./")
	if cleanName == "." {
		return &migrationDir{names: sortedMigrationNames(m.files)}, nil
	}
	content, ok := m.files[cleanName]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return &migrationFile{
		name:   cleanName,
		reader: strings.NewReader(content),
		size:   int64(len(content)),
	}, nil
}

func sortedMigrationNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type migrationDir struct {
	names []string
	read  bool
}

func (d migrationDir) Stat() (fs.FileInfo, error) {
	return migrationFileInfo{name: ".", dir: true}, nil
}

func (d migrationDir) Read([]byte) (int, error) {
	return 0, errors.New("cannot read migration directory")
}

func (d migrationDir) Close() error {
	return nil
}

func (d *migrationDir) ReadDir(count int) ([]fs.DirEntry, error) {
	if d.read {
		return nil, io.EOF
	}
	d.read = true

	entries := make([]fs.DirEntry, 0, len(d.names))
	for _, name := range d.names {
		entries = append(entries, migrationDirEntry{name: name})
	}
	return entries, nil
}

type migrationFile struct {
	name   string
	reader *strings.Reader
	size   int64
}

func (f *migrationFile) Stat() (fs.FileInfo, error) {
	return migrationFileInfo{name: f.name, size: f.size}, nil
}

func (f *migrationFile) Read(p []byte) (int, error) {
	return f.reader.Read(p)
}

func (f *migrationFile) Close() error {
	return nil
}

type migrationDirEntry struct {
	name string
}

func (e migrationDirEntry) Name() string {
	return e.name
}

func (e migrationDirEntry) IsDir() bool {
	return false
}

func (e migrationDirEntry) Type() fs.FileMode {
	return 0
}

func (e migrationDirEntry) Info() (fs.FileInfo, error) {
	return migrationFileInfo{name: e.name}, nil
}

type migrationFileInfo struct {
	name string
	size int64
	dir  bool
}

func (i migrationFileInfo) Name() string {
	return i.name
}

func (i migrationFileInfo) Size() int64 {
	return i.size
}

func (i migrationFileInfo) Mode() fs.FileMode {
	if i.dir {
		return fs.ModeDir | 0o755
	}
	return 0o644
}

func (i migrationFileInfo) ModTime() time.Time {
	return time.Time{}
}

func (i migrationFileInfo) IsDir() bool {
	return i.dir
}

func (i migrationFileInfo) Sys() any {
	return nil
}
