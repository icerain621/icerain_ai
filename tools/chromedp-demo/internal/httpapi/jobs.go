package httpapi

import (
	"context"
	"sync"
	"time"
)

type JobStatus string

const (
	JobQueued  JobStatus = "queued"
	JobRunning JobStatus = "running"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
)

type Job struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Status    JobStatus `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	StartedAt *time.Time `json:"startedAt,omitempty"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`

	OutDir string `json:"outDir,omitempty"`

	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type JobManager struct {
	mu   sync.Mutex
	jobs map[string]*Job
}

func NewJobManager() *JobManager {
	return &JobManager{jobs: map[string]*Job{}}
}

func (m *JobManager) Create(id, kind, outDir string) *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	j := &Job{
		ID:        id,
		Kind:      kind,
		Status:    JobQueued,
		CreatedAt: time.Now(),
		OutDir:    outDir,
	}
	m.jobs[id] = j
	return j
}

func (m *JobManager) Get(id string) (*Job, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[id]
	return j, ok
}

func (m *JobManager) Start(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[id]; ok {
		now := time.Now()
		j.Status = JobRunning
		j.StartedAt = &now
	}
}

func (m *JobManager) Done(id string, result any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[id]; ok {
		now := time.Now()
		j.Status = JobDone
		j.EndedAt = &now
		j.Result = result
	}
}

func (m *JobManager) Fail(id string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if j, ok := m.jobs[id]; ok {
		now := time.Now()
		j.Status = JobFailed
		j.EndedAt = &now
		if err != nil {
			j.Error = err.Error()
		}
	}
}

// RunAsync runs fn in a goroutine and records status transitions.
func (m *JobManager) RunAsync(ctx context.Context, id string, fn func(context.Context) (any, error)) {
	go func() {
		m.Start(id)
		res, err := fn(ctx)
		if err != nil {
			m.Fail(id, err)
			return
		}
		m.Done(id, res)
	}()
}

