package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"zoamel/pokesensei/db"
)

func TestRunMigrations_011_InsertsXYGameData(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "pokesensei.db")

	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open sqlite db: %v", err)
	}
	defer sqlDB.Close()

	if err := RunMigrations(dbPath, db.EmbedMigrations); err != nil {
		t.Fatalf("run migrations: %v", err)
	}

	ctx := context.Background()

	var vgName, vgSlug, vgEra string
	var vgGeneration, vgMaxDex, vgMaxBadges int
	err = sqlDB.QueryRowContext(ctx, `
		SELECT name, slug, generation, max_pokedex, type_chart_era, max_badges
		FROM version_groups WHERE id = 15
	`).Scan(&vgName, &vgSlug, &vgGeneration, &vgMaxDex, &vgEra, &vgMaxBadges)
	if err != nil {
		t.Fatalf("query xy version group: %v", err)
	}
	if vgSlug != "xy" || vgName != "X / Y" || vgGeneration != 6 ||
		vgMaxDex != 721 || vgEra != "post_fairy" || vgMaxBadges != 8 {
		t.Fatalf("xy version group fields wrong: slug=%q name=%q gen=%d maxDex=%d era=%q badges=%d",
			vgSlug, vgName, vgGeneration, vgMaxDex, vgEra, vgMaxBadges)
	}

	gameVersions := map[int64]struct {
		slug string
		name string
	}{
		23: {"x", "X"},
		24: {"y", "Y"},
	}
	for id, want := range gameVersions {
		var slug, name string
		var vgID sql.NullInt64
		err := sqlDB.QueryRowContext(ctx, `
			SELECT slug, name, version_group_id FROM game_versions WHERE id = ?
		`, id).Scan(&slug, &name, &vgID)
		if err != nil {
			t.Fatalf("query game_version %d: %v", id, err)
		}
		if slug != want.slug || name != want.name ||
			!vgID.Valid || vgID.Int64 != 15 {
			t.Fatalf("game_version %d wrong: slug=%q name=%q vgID=%+v",
				id, slug, name, vgID)
		}
	}

	expectedStarters := map[int64][]int64{
		23: {1, 4, 7, 650, 653, 656},
		24: {1, 4, 7, 650, 653, 656},
	}
	for gvID, wantIDs := range expectedStarters {
		rows, err := sqlDB.QueryContext(ctx, `
			SELECT pokemon_id FROM starter_groups
			WHERE game_version_id = ? ORDER BY pokemon_id
		`, gvID)
		if err != nil {
			t.Fatalf("query starters for %d: %v", gvID, err)
		}
		var got []int64
		for rows.Next() {
			var pid int64
			if err := rows.Scan(&pid); err != nil {
				rows.Close()
				t.Fatalf("scan starter row: %v", err)
			}
			got = append(got, pid)
		}
		rows.Close()
		if len(got) != len(wantIDs) {
			t.Fatalf("game_version %d starters: got %d rows, want %d", gvID, len(got), len(wantIDs))
		}
		for i, want := range wantIDs {
			if got[i] != want {
				t.Fatalf("game_version %d starter[%d] = %d, want %d", gvID, i, got[i], want)
			}
		}
	}
}
