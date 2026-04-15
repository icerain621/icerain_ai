package obs

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RunLogger writes human-readable logs to stdout and structured JSONL logs to file.
type RunLogger struct {
	Logger *log.Logger
	Close  func() error
}

type jsonlWriter struct {
	mu  sync.Mutex
	out io.Writer
}

func (w *jsonlWriter) Write(p []byte) (int, error) {
	// log.Logger may call Write with partial lines; treat each call as one message.
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}
	rec := map[string]any{
		"ts":  time.Now().Format(time.RFC3339Nano),
		"msg": msg,
	}
	b, _ := json.Marshal(rec)

	w.mu.Lock()
	defer w.mu.Unlock()
	_, err := w.out.Write(append(b, '\n'))
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

// NewRunLogger creates a logger that logs to stdout and also writes JSONL to outDir/run.log.jsonl.
func NewRunLogger(outDir string) (*RunLogger, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(outDir, "run.log.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}

	jw := &jsonlWriter{out: f}
	mw := io.MultiWriter(os.Stdout, jw)
	l := log.New(mw, "", log.LstdFlags|log.Lmicroseconds)

	return &RunLogger{
		Logger: l,
		Close:  f.Close,
	}, nil
}

