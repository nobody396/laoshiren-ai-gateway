package repository

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	embeddedmigrations "github.com/bozhouDev/DragonCode-sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestExtractMigrationUpSQL(t *testing.T) {
	t.Run("unmarked migration is unchanged", func(t *testing.T) {
		content := "CREATE TABLE example (note TEXT DEFAULT '-- +goose Up');"
		got, err := extractMigrationUpSQL(content)
		require.NoError(t, err)
		require.Equal(t, content, got)
	})

	t.Run("comment-only preface is allowed", func(t *testing.T) {
		content := `/* outer comment
   /* nested comment */
*/
-- +goose Up
SELECT 1;`
		got, err := extractMigrationUpSQL(content)
		require.NoError(t, err)
		require.Equal(t, "SELECT 1;", got)
	})

	t.Run("only Up SQL is returned", func(t *testing.T) {
		content := `-- migration description
-- +goose Up
-- +goose StatementBegin
CREATE TABLE kept (id BIGINT);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE kept;
-- +goose StatementEnd`
		got, err := extractMigrationUpSQL(content)
		require.NoError(t, err)
		require.Contains(t, got, "CREATE TABLE kept")
		require.NotContains(t, got, "DROP TABLE")
		require.NotContains(t, got, "+goose")
	})

	t.Run("Up without Down is allowed", func(t *testing.T) {
		got, err := extractMigrationUpSQL("-- +goose Up\nSELECT 1;")
		require.NoError(t, err)
		require.Equal(t, "SELECT 1;", got)
	})

	t.Run("noncanonical whitespace still cannot expose Down SQL", func(t *testing.T) {
		content := "--   +goose   Up\nSELECT 1;\n--  +goose  Down\nDROP TABLE important;"
		got, err := extractMigrationUpSQL(content)
		require.NoError(t, err)
		require.Equal(t, "SELECT 1;", got)
		require.NotContains(t, got, "DROP TABLE")
		require.Error(t, validateMigrationDirectionPolicy("142_new.sql", content))
	})

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "Down before Up", content: "-- +goose Down\nSELECT 1;", want: "misplaced Down"},
		{name: "duplicate Up", content: "-- +goose Up\nSELECT 1;\n-- +goose Up\nSELECT 2;", want: "duplicate or misplaced Up"},
		{name: "duplicate Down", content: "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 2;\n-- +goose Down", want: "duplicate or misplaced Down"},
		{name: "SQL before Up", content: "SELECT 0;\n-- +goose Up\nSELECT 1;", want: "executable SQL before Up"},
		{name: "unclosed statement", content: "-- +goose Up\n-- +goose StatementBegin\nSELECT 1;", want: "unclosed"},
		{name: "empty Up", content: "-- +goose Up\n-- only a comment\n-- +goose Down\nSELECT 1;", want: "contains no executable SQL"},
		{name: "unknown marker", content: "-- +goose Up\nSELECT 1;\n-- +goose Future", want: "unsupported Goose marker"},
		{name: "unterminated preface comment", content: "/* not closed\n-- +goose Up\nSELECT 1;", want: "unterminated block comment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := extractMigrationUpSQL(tt.content)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestValidateMigrationDirectionPolicy(t *testing.T) {
	legacy := "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 2;"
	for name := range legacyMigrationsWithDownSections {
		require.NoError(t, validateMigrationDirectionPolicy(name, legacy), name)
	}

	err := validateMigrationDirectionPolicy("142_new_migration.sql", legacy)
	require.Error(t, err)
	require.Contains(t, err.Error(), "forward-only")
	require.NoError(t, validateMigrationDirectionPolicy("142_new_migration.sql", "SELECT 1;"))
}

func TestApplyMigrationsFS_ExecutesOnlyGooseUpSection(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	prepareMigrationsBootstrapExpectations(mock)
	mock.ExpectQuery("SELECT checksum FROM schema_migrations WHERE filename = \\$1").
		WithArgs("019_migrate_wechat_to_attributes.sql").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectBegin()
	mock.ExpectExec("CREATE TABLE kept").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("INSERT INTO schema_migrations \\(filename, checksum\\) VALUES \\(\\$1, \\$2\\)").
		WithArgs("019_migrate_wechat_to_attributes.sql", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	expectMigrationUnlock(mock)

	fsys := fstest.MapFS{
		"019_migrate_wechat_to_attributes.sql": &fstest.MapFile{Data: []byte(`-- +goose Up
CREATE TABLE kept (id BIGINT);
-- +goose Down
DROP TABLE kept;`)},
	}

	err = applyMigrationsFS(context.Background(), db, fsys)
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEmbeddedMigrations_DoNotAddNewDownSections(t *testing.T) {
	entries, err := fs.ReadDir(embeddedmigrations.FS, ".")
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		content, readErr := fs.ReadFile(embeddedmigrations.FS, entry.Name())
		require.NoError(t, readErr)
		if hasGooseMarker(string(content), "Down") {
			_, ok := legacyMigrationsWithDownSections[entry.Name()]
			require.True(t, ok, "new migration %s contains a forbidden Down section", entry.Name())
		}
	}
}
