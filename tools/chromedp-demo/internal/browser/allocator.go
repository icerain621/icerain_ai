package browser

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

type Options struct {
	Headless    bool
	Proxy       string
	UserAgent   string
	UserDataDir string
	WindowWidth int
	WindowHeight int
}

func NewAllocator(parent context.Context, opts Options) (context.Context, context.CancelFunc) {
	allocOpts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)

	allocOpts = append(allocOpts,
		chromedp.Flag("headless", opts.Headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
	)

	if opts.WindowWidth > 0 && opts.WindowHeight > 0 {
		allocOpts = append(allocOpts, chromedp.WindowSize(opts.WindowWidth, opts.WindowHeight))
	}
	if opts.Proxy != "" {
		allocOpts = append(allocOpts, chromedp.ProxyServer(opts.Proxy))
	}
	if opts.UserAgent != "" {
		allocOpts = append(allocOpts, chromedp.UserAgent(opts.UserAgent))
	}
	if opts.UserDataDir != "" {
		allocOpts = append(allocOpts, chromedp.UserDataDir(opts.UserDataDir))
	}

	// A small default timeout to avoid hanging allocator startup; callers should wrap with their own.
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, allocOpts...)

	// Tie both cancels: allocCancel first, then timeout cancel.
	return allocCtx, func() {
		allocCancel()
		cancel()
	}
}

func ValidateOptions(opts Options) error {
	if opts.WindowWidth < 0 || opts.WindowHeight < 0 {
		return fmt.Errorf("invalid window size")
	}
	return nil
}

