package browser

import (
	"context"
	"time"

	"github.com/chromedp/chromedp"
)

type Session struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc

	ctx    context.Context
	cancel context.CancelFunc
}

func NewSession(parent context.Context, opts Options, logf func(string, ...any)) (*Session, error) {
	if err := ValidateOptions(opts); err != nil {
		return nil, err
	}

	allocCtx, allocCancel := NewAllocator(parent, opts)

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(logf))

	// Ensure the browser process is started early; avoids first-action latency.
	startCtx, startCancel := context.WithTimeout(ctx, 30*time.Second)
	defer startCancel()
	if err := chromedp.Run(startCtx); err != nil {
		cancel()
		allocCancel()
		return nil, err
	}

	return &Session{
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
		ctx:         ctx,
		cancel:      cancel,
	}, nil
}

func (s *Session) Context() context.Context { return s.ctx }

func (s *Session) Close() {
	if s.cancel != nil {
		s.cancel()
	}
	if s.allocCancel != nil {
		s.allocCancel()
	}
}

