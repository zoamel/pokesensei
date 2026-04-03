package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"zoamel/pokesensei/db"
)

func TestRunMigrations_BackfillsGameVersionGroupIDs(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "pokesensei.db")

	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer sqlDB.Close()

	stmts := []string{
		`CREATE TABLE goose_db_version (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version_id INTEGER NOT NULL,
			is_applied INTEGER NOT NULL,
			tstamp TIMESTAMP DEFAULT (datetime('now'))
		);`,
		`INSERT INTO goose_db_version (version_id, is_applied) VALUES
			(0, 1),
			(1, 1),
			(2, 1),
			(3, 1),
			(4, 1),
			(5, 1),
			(6, 1),
			(7, 1),
			(8, 1);`,
		`CREATE TABLE game_versions (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			slug TEXT NOT NULL UNIQUE,
			version_group_id INTEGER
		);`,
		`INSERT INTO game_versions (id, name, slug, version_group_id) VALUES
			(10, 'FireRed', 'firered', NULL),
			(11, 'LeafGreen', 'leafgreen', NULL),
			(15, 'HeartGold', 'heartgold', NULL),
			(16, 'SoulSilver', 'soulsilver', NULL);`,
	}

	for _, stmt := range stmts {
		if _, err := sqlDB.ExecContext(context.Background(), stmt); err != nil {
			t.Fatalf("seed test database: %v", err)
		}
	}

	if err := RunMigrations(dbPath, db.EmbedMigrations); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	rows, err := sqlDB.QueryContext(context.Background(), `
		SELECT id, version_group_id
		FROM game_versions
		ORDER BY id
	`)
	if err != nil {
		t.Fatalf("query game_versions: %v", err)
	}
	defer rows.Close()

	got := map[int64]sql.NullInt64{}
	for rows.Next() {
		var id int64
		var versionGroupID sql.NullInt64
		if err := rows.Scan(&id, &versionGroupID); err != nil {
			t.Fatalf("scan game_version row: %v", err)
		}
		got[id] = versionGroupID
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate game_versions: %v", err)
	}

	want := map[int64]int64{
		10: 7,
		11: 7,
		15: 10,
		16: 10,
	}

	for id, expected := range want {
		actual, ok := got[id]
		if !ok {
			t.Fatalf("missing game version %d", id)
		}
		if !actual.Valid || actual.Int64 != expected {
			t.Fatalf("game version %d version_group_id = %+v, want %d", id, actual, expected)
		}
	}
}
