package obs

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Telemetry struct {
	outDir string
	log    *log.Logger

	mu     sync.Mutex
	events []Event
}

type Event struct {
	TS     time.Time `json:"ts"`
	TaskID string    `json:"taskId"`
	Kind   string    `json:"kind"`
	Msg    string    `json:"msg"`
	Fields map[string]any `json:"fields,omitempty"`
}

func New(outDir string, logger *log.Logger) *Telemetry {
	return &Telemetry{outDir: outDir, log: logger}
}

func (t *Telemetry) Record(taskID, kind, msg string, fields map[string]any) {
	t.mu.Lock()
	t.events = append(t.events, Event{TS: time.Now(), TaskID: taskID, Kind: kind, Msg: msg, Fields: fields})
	t.mu.Unlock()

	if t.log != nil {
		t.log.Printf("[%s] %s", kind, msg)
	}
}

func (t *Telemetry) RecordArtifact(taskID, kind, path string) {
	t.Record(taskID, "artifact", kind, map[string]any{"path": path})
}

func (t *Telemetry) RecordError(taskID string, err error) {
	if err == nil {
		return
	}
	t.Record(taskID, "error", err.Error(), nil)
}

func (t *Telemetry) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()

	path := filepath.Join(t.outDir, "telemetry.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		if t.log != nil {
			t.log.Printf("write telemetry: %v", err)
		}
		return
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, ev := range t.events {
		_ = enc.Encode(ev)
	}
	if t.log != nil {
		t.log.Printf("wrote %s", path)
	}
}

