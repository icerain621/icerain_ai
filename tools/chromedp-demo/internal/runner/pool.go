package runner

import (
	"context"
	"sync"
)

// TaskSource allows plugging in queue systems (Redis/SQS/Kafka/etc.).
type TaskSource interface {
	Next(ctx context.Context) (Task, error)
}

// TaskSink allows persisting results (DB/Object storage/etc.).
type TaskSink interface {
	Put(ctx context.Context, res *Result) error
	PutError(ctx context.Context, taskID string, err error) error
}

type Pool struct {
	Runner *Runner
	Workers int
}

func (p Pool) Run(ctx context.Context, src TaskSource, sink TaskSink) error {
	if p.Workers <= 0 {
		p.Workers = 1
	}

	var wg sync.WaitGroup
	errCh := make(chan error, p.Workers)

	worker := func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			task, err := src.Next(ctx)
			if err != nil {
				errCh <- err
				return
			}
			res, runErr := p.Runner.Run(ctx, task)
			if runErr != nil {
				if sink != nil {
					_ = sink.PutError(ctx, task.ID, runErr)
				}
				continue
			}
			if sink != nil {
				_ = sink.Put(ctx, res)
			}
		}
	}

	for i := 0; i < p.Workers; i++ {
		wg.Add(1)
		go worker()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		return err
	case <-done:
		return nil
	}
}

