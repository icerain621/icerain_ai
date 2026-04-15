package runner

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	sqlstore "github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/store/sqlite"
)

type SQLiteQueue struct {
	Store *sqlstore.Store
}

func (q SQLiteQueue) Next(ctx context.Context) (Task, error) {
	row, err := q.Store.NextQueued(ctx)
	if err != nil {
		return Task{}, err
	}
	return Task{
		ID:        row.ID,
		TargetURL: row.URL,
		OutDir:    "", // caller should set per-task outdir
	}, nil
}

type SQLiteSink struct {
	Store *sqlstore.Store
}

func (s SQLiteSink) Put(ctx context.Context, res *Result) error {
	if res == nil {
		return nil
	}
	return s.Store.MarkDone(ctx, res.TaskID)
}

func (s SQLiteSink) PutError(ctx context.Context, taskID string, err error) error {
	if err == nil {
		return nil
	}
	return s.Store.MarkFailed(ctx, taskID, err.Error())
}

// SeedFromFile reads urls line by line, trims spaces, skips empty/comment lines.
func SeedFromFile(ctx context.Context, st *sqlstore.Store, path string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	var enq int64
	sc := bufio.NewScanner(f)
	var i int64
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i++
		id := st.EnsureTaskID(line, i)
		ok, err := st.Enqueue(ctx, id, line)
		if err != nil {
			return enq, err
		}
		if ok {
			enq++
		}
	}
	if err := sc.Err(); err != nil {
		return enq, err
	}
	return enq, nil
}

// WaitUntilEmpty blocks until queue is empty or ctx canceled.
func WaitUntilEmpty(ctx context.Context, st *sqlstore.Store, poll time.Duration) error {
	if poll <= 0 {
		poll = 2 * time.Second
	}
	for {
		stats, err := st.Stats(ctx)
		if err != nil {
			return err
		}
		if stats["queued"] == 0 && stats["running"] == 0 {
			return nil
		}
		select {
		case <-time.After(poll):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func IsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}

