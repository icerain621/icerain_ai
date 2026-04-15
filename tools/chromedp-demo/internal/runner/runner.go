package runner

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/browser"
	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/guard"
	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/login"
	netobs "github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/network"
	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/obs"
)

type CaptureConfig struct {
	EnableObserver bool
	URLRegex       any // *regexp.Regexp, kept as any to avoid exposing regexp in JSON result struct
	MaxBodyBytes   int
	BlockImages    bool
}

type Task struct {
	ID        string
	TargetURL string
	OutDir    string
	Capture   CaptureConfig
	TriggerEval string
	TriggerWait time.Duration
}

type Result struct {
	TaskID   string                    `json:"taskId"`
	Target   string                    `json:"target"`
	Title    string                    `json:"title"`
	URL      string                    `json:"url"`
	Text     string                    `json:"text"`
	HTMLPath string                    `json:"htmlPath"`
	ShotPath string                    `json:"shotPath"`
	Network  []netobs.CapturedResponse `json:"network,omitempty"`
}

type Options struct {
	BrowserOptions  browser.Options
	Policy          guard.Policy
	Telemetry       *obs.Telemetry
	NetworkObserver *netobs.Observer
}

type Runner struct {
	opts Options

	interceptor *netobs.Interceptor
	formLogin   *FormLoginConfig
}

func New(opts Options) *Runner { return &Runner{opts: opts} }

// SetInterceptHeader enables Fetch interception and adds a request header for all URLs.
// This is a demo hook; production usage should bind this to site policy + allowlist.
func (r *Runner) SetInterceptHeader(k, v string) {
	r.interceptor = netobs.NewInterceptor([]netobs.InterceptRule{
		{
			URLRegex: regexp.MustCompile(".*"),
			Action:   netobs.InterceptAction{AddHeaders: map[string]string{k: v}},
		},
	})
}

type FormLoginConfig struct {
	LoginURL          string
	Username          string
	Password          string
	UsernameSelector  string
	PasswordSelector  string
	SubmitSelector    string
	AfterSelector     string
}

func (r *Runner) SetFormLogin(cfg FormLoginConfig) {
	r.formLogin = &cfg
}

func (r *Runner) Run(ctx context.Context, task Task) (*Result, error) {
	if r.opts.Telemetry != nil {
		r.opts.Telemetry.Record(task.ID, "start", "task started", map[string]any{"url": task.TargetURL})
	}
	if err := r.opts.Policy.CheckURL(task.TargetURL); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(task.OutDir, 0o755); err != nil {
		return nil, err
	}

	sess, err := browser.NewSession(ctx, r.opts.BrowserOptions, func(format string, a ...any) {
		if r.opts.Telemetry != nil {
			r.opts.Telemetry.Record(task.ID, "chromedp", fmt.Sprintf(format, a...), nil)
		}
	})
	if err != nil {
		return nil, err
	}
	defer sess.Close()

	pageCtx := sess.Context()

	// Network setup.
	if err := chromedp.Run(pageCtx, network.Enable()); err != nil {
		return nil, err
	}

	if r.interceptor != nil {
		if err := chromedp.Run(pageCtx, fetch.Enable()); err != nil {
			return nil, err
		}
	}

	if task.Capture.BlockImages {
		_ = chromedp.Run(pageCtx, network.SetBlockedURLS([]string{
			"*.png", "*.jpg", "*.jpeg", "*.gif", "*.webp", "*.svg", "*.ico",
		}))
	}

	var metaStore *netobs.MetaStore
	if task.Capture.EnableObserver && r.opts.NetworkObserver != nil {
		metaStore = netobs.NewMetaStore()
		r.attachNetworkListeners(pageCtx, task, metaStore)
	}

	if r.interceptor != nil {
		r.attachFetchInterceptor(pageCtx)
	}

	if r.formLogin != nil && r.formLogin.LoginURL != "" {
		if err := r.doFormLogin(pageCtx); err != nil {
			return nil, err
		}
	}

	// Basic crawl.
	htmlSel := "html"
	if err := chromedp.Run(pageCtx,
		chromedp.Navigate(task.TargetURL),
		chromedp.WaitReady(htmlSel, chromedp.ByQuery),
	); err != nil {
		return nil, err
	}

	if task.TriggerEval != "" {
		if err := chromedp.Run(pageCtx, chromedp.Evaluate(task.TriggerEval, nil)); err != nil {
			return nil, err
		}
		wait := task.TriggerWait
		if wait <= 0 {
			wait = 800 * time.Millisecond
		}
		select {
		case <-time.After(wait):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	shotPath := filepath.Join(task.OutDir, "page.png")
	var shot []byte
	if err := chromedp.Run(pageCtx, chromedp.FullScreenshot(&shot, 90)); err != nil {
		return nil, err
	}
	if err := os.WriteFile(shotPath, shot, 0o644); err != nil {
		return nil, err
	}
	if r.opts.Telemetry != nil {
		r.opts.Telemetry.RecordArtifact(task.ID, "screenshot", shotPath)
	}

	snap, err := browser.BasicSnapshot(pageCtx, htmlSel, r.opts.Policy.Budget.MaxHTMLBytes)
	if err != nil {
		return nil, err
	}

	htmlPath := filepath.Join(task.OutDir, "page.html")
	if err := os.WriteFile(htmlPath, []byte(snap.HTML), 0o644); err != nil {
		return nil, err
	}
	if r.opts.Telemetry != nil {
		r.opts.Telemetry.RecordArtifact(task.ID, "html", htmlPath)
	}

	// Allow network events to drain briefly after load.
	select {
	case <-time.After(800 * time.Millisecond):
	case <-ctx.Done():
	}

	res := &Result{
		TaskID:   task.ID,
		Target:   task.TargetURL,
		Title:    snap.Title,
		URL:      snap.URL,
		Text:     truncate(snap.Text, 20_000),
		HTMLPath: htmlPath,
		ShotPath: shotPath,
	}

	if r.opts.NetworkObserver != nil {
		res.Network = r.opts.NetworkObserver.Snapshot()
	}

	if r.opts.Telemetry != nil {
		r.opts.Telemetry.Record(task.ID, "done", "task finished", map[string]any{
			"title": res.Title,
		})
	}
	return res, nil
}

func (r *Runner) attachNetworkListeners(ctx context.Context, task Task, metaStore *netobs.MetaStore) {
	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			if e == nil || e.Response == nil {
				return
			}
			url := e.Response.URL
			if r.opts.NetworkObserver == nil || !r.opts.NetworkObserver.ShouldCapture(url) {
				return
			}
			metaStore.Put(netobs.ResponseMeta{
				RequestID: e.RequestID,
				URL:       url,
				Status:    int64(e.Response.Status),
				MIMEType:  e.Response.MimeType,
			})
		case *network.EventLoadingFinished:
			if r.opts.NetworkObserver == nil {
				return
			}
			meta, ok := metaStore.Get(e.RequestID)
			if !ok {
				return
			}
			body, base64Encoded, err := netobs.CaptureResponseBody(ctx, e.RequestID)
			if err != nil {
				return
			}

			max := task.Capture.MaxBodyBytes
			if max <= 0 {
				max = 1_000_000
			}
			truncated := false
			if len(body) > max {
				body = body[:max]
				truncated = true
			}

			c := netobs.CapturedResponse{
				URL:        meta.URL,
				Status:     meta.Status,
				MIMEType:   meta.MIMEType,
				CapturedAt: time.Now(),
				Truncated:  truncated,
			}
			if base64Encoded {
				c.BodyBase64 = base64.StdEncoding.EncodeToString(body)
			} else {
				c.BodyText = string(body)
			}
			r.opts.NetworkObserver.AddCaptured(c)
		}
	})
}

func (r *Runner) attachFetchInterceptor(ctx context.Context) {
	chromedp.ListenTarget(ctx, func(ev any) {
		e, ok := ev.(*fetch.EventRequestPaused)
		if !ok || r.interceptor == nil {
			return
		}
		_ = r.interceptor.Handle(ctx, e)
	})
}

func (r *Runner) doFormLogin(ctx context.Context) error {
	cfg := r.formLogin
	if cfg == nil {
		return nil
	}
	if cfg.Username == "" || cfg.Password == "" {
		return fmt.Errorf("login credentials missing (use -login-user/-login-pass or env CRAWL_LOGIN_USER/CRAWL_LOGIN_PASS)")
	}
	if err := chromedp.Run(ctx,
		chromedp.Navigate(cfg.LoginURL),
		chromedp.WaitReady("html", chromedp.ByQuery),
	); err != nil {
		return err
	}
	return login.FormLogin(ctx, login.FormSpec{
		UsernameSelector: cfg.UsernameSelector,
		PasswordSelector: cfg.PasswordSelector,
		SubmitSelector:   cfg.SubmitSelector,
		AfterSelector:    cfg.AfterSelector,
	}, login.Credentials{Username: cfg.Username, Password: cfg.Password})
}

func truncate(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n]
}

