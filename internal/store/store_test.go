package store

import (
	"context"
	"os"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Runs against a throwaway database, e.g.:
//
//	docker run -d --rm --name llb-test -e POSTGRES_PASSWORD=test -p 55432:5432 postgres:17-alpine
//	TEST_DATABASE_URL=postgres://postgres:test@localhost:55432/postgres?sslmode=disable go test ./internal/store
func TestStore(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	init, err := os.ReadFile("../../migrations/00001_init.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, pool, fstest.MapFS{"00001_init.sql": {Data: init}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Exec(ctx, `DROP TABLE IF EXISTS word_groups, groups, words, settings, goose_db_version CASCADE; DROP FUNCTION IF EXISTS add_word`)
	})
	s := New(pool)

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.AddWord(ctx, "en", "apple", "яблоко", "", []string{"Food"}))
	must(s.AddWord(ctx, "en", "Apple", "яблоко, яблоня", "fruit", []string{"basics"})) // upsert, case-insensitive
	must(s.AddWord(ctx, "en", "server", "сервер", "", []string{"arch", "food"}))
	must(s.AddWord(ctx, "ja", "猫", "кошка", "ねこ", nil))

	langs, err := s.LanguageCounts(ctx)
	must(err)
	if len(langs) != 2 || langs[0].Name != "en" || langs[0].Total != 2 || langs[0].Due != 2 {
		t.Fatalf("LanguageCounts = %+v", langs)
	}

	food, err := s.GroupID(ctx, "FOOD")
	must(err)
	ids, err := s.ReviewIDs(ctx, "en", food, true)
	must(err)
	if len(ids) != 2 {
		t.Fatalf("ReviewIDs(food) = %v, want 2 words", ids)
	}
	w, err := s.Word(ctx, ids[0])
	must(err)
	if w.Word != "apple" || w.Translation != "яблоко, яблоня" || w.Note != "fruit" || w.Status != "new" {
		t.Fatalf("Word = %+v", w)
	}

	must(s.Rate(ctx, w.ID, "good", 0, time.Now().Add(24*time.Hour)))
	due, err := s.ReviewIDs(ctx, "en", 0, true)
	must(err)
	all, err := s.ReviewIDs(ctx, "en", 0, false)
	must(err)
	if len(due) != 1 || len(all) != 2 || all[1] != w.ID {
		t.Fatalf("after rating: due = %v, all = %v", due, all)
	}

	groups, err := s.GroupCounts(ctx, "en")
	must(err)
	if len(groups) != 3 { // arch, basics, food
		t.Fatalf("GroupCounts = %+v", groups)
	}

	must(s.SetSetting(ctx, "lang", "ja"))
	if v, _ := s.Setting(ctx, "lang", "en"); v != "ja" {
		t.Fatalf("Setting = %q", v)
	}
	if ok, err := s.DeleteWord(ctx, "en", "APPLE"); !ok || err != nil {
		t.Fatalf("DeleteWord = %v, %v", ok, err)
	}
}
