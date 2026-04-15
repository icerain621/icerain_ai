package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;

CREATE TABLE IF NOT EXISTS tasks (
  id TEXT PRIMARY KEY,
  url TEXT NOT NULL,
  url_hash TEXT NOT NULL,
  status TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0,
  last_error TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_url_hash ON tasks(url_hash);

CREATE TABLE IF NOT EXISTS checkpoints (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS captures (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id TEXT NOT NULL,
  url TEXT NOT NULL,
  status INTEGER NOT NULL,
  mime TEXT,
  body_path TEXT,
  created_at INTEGER NOT NULL
);
`)
	return err
}

type TaskRow struct {
	ID       string
	URL      string
	Status   string
	Attempts int
	LastErr  string
}

func HashURL(u string) string {
	sum := sha256.Sum256([]byte(u))
	return hex.EncodeToString(sum[:])
}

func (s *Store) Enqueue(ctx context.Context, id, url string) (enqueued bool, err error) {
	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx, `
INSERT OR IGNORE INTO tasks(id, url, url_hash, status, attempts, created_at, updated_at)
VALUES(?, ?, ?, 'queued', 0, ?, ?)
`, id, url, HashURL(url), now, now)
	if err != nil {
		return false, err
	}

	// Detect if it was ignored due to unique url_hash.
	var cnt int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM tasks WHERE id = ?`, id).Scan(&cnt); err != nil {
		return false, err
	}
	return cnt == 1, nil
}

func (s *Store) NextQueued(ctx context.Context) (*TaskRow, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
SELECT id, url, status, attempts, COALESCE(last_error,'')
FROM tasks
WHERE status='queued'
ORDER BY created_at ASC
LIMIT 1
`)
	var t TaskRow
	if err := row.Scan(&t.ID, &t.URL, &t.Status, &t.Attempts, &t.LastErr); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	now := time.Now().Unix()
	_, err = tx.ExecContext(ctx, `
UPDATE tasks SET status='running', attempts=attempts+1, updated_at=? WHERE id=? AND status='queued'
`, now, t.ID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	t.Status = "running"
	t.Attempts++
	return &t, nil
}

func (s *Store) MarkDone(ctx context.Context, id string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE tasks SET status='done', updated_at=? WHERE id=?`, now, id)
	return err
}

func (s *Store) MarkFailed(ctx context.Context, id string, errMsg string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `UPDATE tasks SET status='failed', last_error=?, updated_at=? WHERE id=?`, errMsg, now, id)
	return err
}

func (s *Store) RequeueFailed(ctx context.Context, maxAttempts int) (int64, error) {
	now := time.Now().Unix()
	res, err := s.db.ExecContext(ctx, `
UPDATE tasks
SET status='queued', updated_at=?
WHERE status='failed' AND attempts < ?
`, now, maxAttempts)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) PutCheckpoint(ctx context.Context, key, value string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO checkpoints(key,value,updated_at) VALUES(?,?,?)
ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at
`, key, value, now)
	return err
}

func (s *Store) GetCheckpoint(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM checkpoints WHERE key=?`, key).Scan(&v)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, err
	}
	return v, true, nil
}

func (s *Store) AddCapture(ctx context.Context, taskID, url string, status int64, mime, bodyPath string) error {
	now := time.Now().Unix()
	_, err := s.db.ExecContext(ctx, `
INSERT INTO captures(task_id, url, status, mime, body_path, created_at)
VALUES(?,?,?,?,?,?)
`, taskID, url, status, mime, bodyPath, now)
	return err
}

func (s *Store) Stats(ctx context.Context) (map[string]int64, error) {
	out := map[string]int64{}
	for _, st := range []string{"queued", "running", "done", "failed"} {
		var c int64
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM tasks WHERE status=?`, st).Scan(&c); err != nil {
			return nil, err
		}
		out[st] = c
	}
	return out, nil
}

func (s *Store) EnsureTaskID(url string, i int64) string {
	return fmt.Sprintf("%s-%d", HashURL(url)[:12], i)
}

