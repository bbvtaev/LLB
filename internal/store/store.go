// Package store is the Postgres layer: words, groups and settings.
package store

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

var ErrNotFound = errors.New("not found")

type Word struct {
	ID          int64
	Language    string
	Word        string
	Translation string
	Note        string
	Status      string
	Streak      int
	DueAt       time.Time
}

// Count is a review bucket (a language or a group) with due and total word counts.
type Count struct {
	ID    int64
	Name  string
	Due   int
	Total int
}

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Migrate applies all pending migrations from fsys (SQL files at its root).
func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	p, err := goose.NewProvider(goose.DialectPostgres, db, fsys)
	if err != nil {
		return err
	}
	_, err = p.Up(ctx)
	return err
}

func (s *Store) AddWord(ctx context.Context, lang, word, translation, note string, groups []string) error {
	_, err := s.pool.Exec(ctx, `SELECT add_word($1, $2, $3, $4, $5)`, lang, word, translation, note, groups)
	return err
}

func (s *Store) DeleteWord(ctx context.Context, lang, word string) (bool, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM words WHERE language = $1 AND lower(word) = lower($2)`, lang, word)
	return tag.RowsAffected() > 0, err
}

const wordColumns = `id, language, word, translation, COALESCE(note, ''), status, streak, due_at`

func scanWord(row pgx.Row) (Word, error) {
	var w Word
	err := row.Scan(&w.ID, &w.Language, &w.Word, &w.Translation, &w.Note, &w.Status, &w.Streak, &w.DueAt)
	return w, err
}

func (s *Store) Word(ctx context.Context, id int64) (Word, error) {
	w, err := scanWord(s.pool.QueryRow(ctx, `SELECT `+wordColumns+` FROM words WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return w, ErrNotFound
	}
	return w, err
}

// groupFilter matches all words when $2 is 0, otherwise words in group $2.
const groupFilter = `($2::bigint = 0 OR EXISTS (SELECT 1 FROM word_groups wg WHERE wg.word_id = w.id AND wg.group_id = $2))`

// ReviewIDs returns the words to review in language lang, most overdue first.
// groupID 0 means all groups; dueOnly limits the result to words whose timer has run out.
func (s *Store) ReviewIDs(ctx context.Context, lang string, groupID int64, dueOnly bool) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT w.id FROM words w
		WHERE w.language = $1 AND `+groupFilter+` AND (NOT $3::bool OR w.due_at <= now())
		ORDER BY w.due_at, random()`, lang, groupID, dueOnly)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int64])
}

func (s *Store) Rate(ctx context.Context, id int64, status string, streak int, dueAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE words SET status = $2, streak = $3, due_at = $4, last_reviewed_at = now()
		WHERE id = $1`, id, status, streak, dueAt)
	return err
}

func (s *Store) ListWords(ctx context.Context, lang string, groupID int64, limit int) ([]Word, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+wordColumns+` FROM words w
		WHERE w.language = $1 AND `+groupFilter+`
		ORDER BY w.created_at DESC LIMIT $3`, lang, groupID, limit)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Word, error) { return scanWord(row) })
}

func (s *Store) GroupID(ctx context.Context, name string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT id FROM groups WHERE name = lower($1)`, name).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

func collectCounts(rows pgx.Rows, err error) ([]Count, error) {
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (Count, error) {
		var c Count
		err := row.Scan(&c.ID, &c.Name, &c.Due, &c.Total)
		return c, err
	})
}

// LanguageCounts returns every language that has words. Count.ID is always 0.
func (s *Store) LanguageCounts(ctx context.Context) ([]Count, error) {
	return collectCounts(s.pool.Query(ctx, `
		SELECT 0::bigint, language, count(*) FILTER (WHERE due_at <= now()), count(*)
		FROM words GROUP BY language ORDER BY language`))
}

// GroupCounts returns the groups that have words in language lang, oldest group first.
func (s *Store) GroupCounts(ctx context.Context, lang string) ([]Count, error) {
	return collectCounts(s.pool.Query(ctx, `
		SELECT g.id, g.name, count(*) FILTER (WHERE w.due_at <= now()), count(*)
		FROM groups g
		JOIN word_groups wg ON wg.group_id = g.id
		JOIN words w ON w.id = wg.word_id
		WHERE w.language = $1
		GROUP BY g.id, g.name ORDER BY g.id`, lang))
}

// StatusCounts returns how many words of language lang are in each status.
func (s *Store) StatusCounts(ctx context.Context, lang string) (map[string]int, error) {
	rows, err := s.pool.Query(ctx, `SELECT status, count(*) FROM words WHERE language = $1 GROUP BY status`, lang)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := map[string]int{}
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		counts[status] = n
	}
	return counts, rows.Err()
}

func (s *Store) Setting(ctx context.Context, key, fallback string) (string, error) {
	var v string
	err := s.pool.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return fallback, nil
	}
	if err != nil {
		return "", fmt.Errorf("setting %s: %w", key, err)
	}
	return v, nil
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO settings (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value)
	return err
}
