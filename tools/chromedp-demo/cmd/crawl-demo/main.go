package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/browser"
	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/guard"
	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/network"
	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/obs"
	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/runner"
	sqlstore "github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/store/sqlite"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	if len(os.Args) < 2 || isHelpToken(os.Args[1]) {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "help":
		printHelp(os.Args[2:])
		return
	case "v0":
		if err := runBasic(os.Args[2:], logger); err != nil {
			logger.Printf("v0 failed: %v", err)
			os.Exit(1)
		}
	case "v1-queue":
		if err := runQueue(os.Args[2:], logger); err != nil {
			logger.Printf("v1-queue failed: %v", err)
			os.Exit(1)
		}
	case "v1-capture":
		if err := runCapture(os.Args[2:], logger); err != nil {
			logger.Printf("v1-capture failed: %v", err)
			os.Exit(1)
		}
	case "v2":
		if err := runV2(os.Args[2:], logger); err != nil {
			logger.Printf("v2 failed: %v", err)
			os.Exit(1)
		}
	case "v2-queue":
		if err := runV2Queue(os.Args[2:], logger); err != nil {
			logger.Printf("v2-queue failed: %v", err)
			os.Exit(1)
		}
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage:")
	fmt.Fprintln(os.Stderr, "  crawl-demo help [command]")
	fmt.Fprintln(os.Stderr, "  crawl-demo v0 [flags]")
	fmt.Fprintln(os.Stderr, "  crawl-demo v1-queue [flags]")
	fmt.Fprintln(os.Stderr, "  crawl-demo v1-capture [flags]")
	fmt.Fprintln(os.Stderr, "  crawl-demo v2 -workflow <file.json> [flags]")
	fmt.Fprintln(os.Stderr, "  crawl-demo v2-queue -workflow <file.json> [flags]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Try:")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/crawl-demo v0 -url https://example.com -out artifacts")
	fmt.Fprintln(os.Stderr, "  go run ./cmd/crawl-demo help")
}

func printHelp(args []string) {
	cmd := ""
	if len(args) > 0 {
		cmd = strings.TrimSpace(args[0])
	}
	if cmd == "" || cmd == "all" {
		fmt.Fprintln(os.Stdout, helpAll)
		return
	}
	switch cmd {
	case "v0":
		fmt.Fprintln(os.Stdout, helpV0)
	case "v1-queue":
		fmt.Fprintln(os.Stdout, helpV1Queue)
	case "v1-capture":
		fmt.Fprintln(os.Stdout, helpV1Capture)
	case "v2":
		fmt.Fprintln(os.Stdout, helpV2)
	case "v2-queue":
		fmt.Fprintln(os.Stdout, helpV2Queue)
	default:
		fmt.Fprintf(os.Stdout, "Unknown command: %s\n\n", cmd)
		fmt.Fprintln(os.Stdout, helpAll)
	}
}

func isHelpToken(s string) bool {
	switch strings.TrimSpace(strings.ToLower(s)) {
	case "-h", "--help", "help":
		return true
	default:
		return false
	}
}

const helpAll = `crawl-demo help

This CLI integrates all crawler capabilities in this module:

  1) Basic page crawl (navigate / wait / screenshot / html+text dump)
     - Subcommand: v0

  2) v1 task queue + dedup + resume (SQLite-backed queue/state)
     - Subcommand: v1-queue

  3) v1 capture XHR/JSON responses (by URL regex), optionally login first and trigger XHR
     - Subcommand: v1-capture

  4) v2 workflow mode (compose login / capture / trigger using a JSON workflow)
     - Subcommand: v2

  5) v2 queue + workflow (SQLite-backed queue/state; each task runs the same workflow)
     - Subcommand: v2-queue

Local mock login site (for testing login + XHR capture):
  go run ./cmd/mocksite -addr 127.0.0.1:18080

Recommended workflows:

  A) Crawl a single page:
     go run ./cmd/crawl-demo v0 -url https://example.com -out artifacts

  B) Test login flow on mocksite:
     go run ./cmd/crawl-demo v0 ^
       -allow-domains 127.0.0.1 ^
       -url http://127.0.0.1:18080/app ^
       -out artifacts_login ^
       -login-url http://127.0.0.1:18080/login ^
       -login-user-sel "#login_field" ^
       -login-pass-sel "#password" ^
       -login-submit-sel "#sign_in" ^
       -login-after-sel "#app_home" ^
       -login-user demo ^
       -login-pass demo

  C) Login then capture JSON XHR (mocksite /api/profile):
     go run ./cmd/crawl-demo v1-capture ^
       -allow-domains 127.0.0.1 ^
       -url http://127.0.0.1:18080/app ^
       -out artifacts_capture_login ^
       -capture-url "/api/profile" ^
       -login-url http://127.0.0.1:18080/login ^
       -login-user-sel "#login_field" ^
       -login-pass-sel "#password" ^
       -login-submit-sel "#sign_in" ^
       -login-after-sel "#app_home" ^
       -login-user demo ^
       -login-pass demo ^
       -trigger-eval "fetch('/api/profile').then(r=>r.json()).then(_=>0)"

  D) Queue-based crawling with resume:
     go run ./cmd/crawl-demo v1-queue -seed-file seed.txt -db queue.db -out artifacts_queue -workers 4

Tips:
  - Always set -allow-domains for production.
  - Use -user-data-dir to reuse a logged-in Chrome profile when sites are complex.
  - Capture writes bodies to the output dir and records metadata in result.json (no DB write for captures).

Help per command:
  crawl-demo help v0
  crawl-demo help v1-queue
  crawl-demo help v1-capture
  crawl-demo help v2
  crawl-demo help v2-queue
`

const helpV0 = `crawl-demo v0

Basic crawl of a single URL: navigate -> wait -> screenshot -> dump HTML/Text -> result.json

Example:
  go run ./cmd/crawl-demo v0 -url https://example.com -out artifacts

Login (form) options:
  -login-url, -login-user-sel, -login-pass-sel, -login-submit-sel, -login-after-sel
  -login-user/-login-pass (or env CRAWL_LOGIN_USER/CRAWL_LOGIN_PASS)
`

const helpV1Queue = `crawl-demo v1-queue

SQLite-backed task queue with dedup + resume:

  -seed-file: newline-delimited URL list (optional; can be re-run, deduped)
  -db: queue database path (default queue.db)
  -out: output root directory (each task writes to out/<taskId>/)
  -workers: number of workers
  -max-attempts: retry cap before staying failed

Example:
  go run ./cmd/crawl-demo v1-queue -seed-file seed.txt -db queue.db -out artifacts_queue -workers 4
`

const helpV1Capture = `crawl-demo v1-capture

Capture XHR/JSON response bodies by URL regex:

  -capture-url: REQUIRED regex; matching responses will be saved to out/capture_*.json (or .b64)
  -capture-max-bytes: per-response cap

Optional login (form):
  -login-url, selectors, -login-user/-login-pass (or env vars)

Optional trigger (useful for SPA/API):
  -trigger-eval: JS to run after navigation (e.g. fetch('/api/profile')...)
  -trigger-wait: wait time after trigger to let XHR finish
`

const helpV2 = `crawl-demo v2

Workflow mode: load a JSON workflow file and run it for a single task.

Example (mocksite login + capture /api/profile):
  go run ./cmd/crawl-demo v2 ^
    -workflow workflows/mocksite_login_capture.json ^
    -allow-domains 127.0.0.1 ^
    -url http://127.0.0.1:18080/app ^
    -out artifacts_v2

Workflow JSON supports step kinds:
  - login_form
  - capture
  - trigger_eval
  - navigate (optional; if omitted, uses -url as target)

Secrets:
  username/password can be literal or "env:VAR".
`

const helpV2Queue = `crawl-demo v2-queue

Queue + workflow mode: SQLite-backed queue with dedup + resume, each task runs the same workflow.

Example (seed urls -> run workflow per url):
  go run ./cmd/crawl-demo v2-queue ^
    -workflow workflows/mocksite_login_capture.json ^
    -seed-file seed.txt ^
    -db queue_v2.db ^
    -out artifacts_v2_queue ^
    -workers 4 ^
    -allow-domains 127.0.0.1

Notes:
  - Stop anytime and rerun to resume.
  - The workflow is loaded once and applied to every task.
`

func runBasic(args []string, logger *log.Logger) error {
	fs := flag.NewFlagSet("v0", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		targetURL   = fs.String("url", "https://example.com", "target URL")
		outDir      = fs.String("out", "artifacts", "artifacts output dir")
		headless    = fs.Bool("headless", true, "run chrome in headless mode")
		proxy       = fs.String("proxy", "", "proxy server, e.g. http://127.0.0.1:7890")
		userAgent   = fs.String("ua", "", "custom User-Agent")
		userDataDir = fs.String("user-data-dir", "", "Chrome user-data-dir (profile) for session reuse")
		timeout     = fs.Duration("timeout", 90*time.Second, "overall timeout")

		allowDomains = fs.String("allow-domains", "example.com", "comma-separated allowlist domains")

		captureURLPattern = fs.String("capture-url", "", "regex to capture response bodies by url (empty disables)")
		maxCaptureBytes   = fs.Int("capture-max-bytes", 1_000_000, "max bytes per captured response body")
		blockImages       = fs.Bool("block-images", false, "block image requests via Network domain")
		addHeader         = fs.String("add-header", "", "add request header in k=v form via Fetch interception (demo; optional)")

		loginURL       = fs.String("login-url", "", "optional login page url; if set, will navigate and perform form login")
		loginUser      = fs.String("login-user", "", "login username (or set env CRAWL_LOGIN_USER)")
		loginPass      = fs.String("login-pass", "", "login password (or set env CRAWL_LOGIN_PASS)")
		loginUserSel   = fs.String("login-user-sel", "", "username input CSS selector")
		loginPassSel   = fs.String("login-pass-sel", "", "password input CSS selector")
		loginSubmitSel = fs.String("login-submit-sel", "", "submit button CSS selector")
		loginAfterSel  = fs.String("login-after-sel", "", "selector visible after login (optional)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	runlog, err := obs.NewRunLogger(*outDir)
	if err != nil {
		return err
	}
	defer func() { _ = runlog.Close() }()
	logger = runlog.Logger

	policy := guard.Policy{
		AllowedDomains: splitCSV(*allowDomains),
		DeniedSchemes:  []string{"file", "chrome"},
		Budget: guard.Budget{
			Timeout:        *timeout,
			MaxScreenshots: 5,
			MaxHTMLBytes:   2_000_000,
		},
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir out dir: %w", err)
	}

	var captureRe *regexp.Regexp
	var err error
	if strings.TrimSpace(*captureURLPattern) != "" {
		captureRe, err = regexp.Compile(*captureURLPattern)
		if err != nil {
			return fmt.Errorf("invalid -capture-url regex: %w", err)
		}
	}

	task := runner.Task{
		ID:        fmt.Sprintf("task-%d", time.Now().Unix()),
		TargetURL: *targetURL,
		OutDir:    *outDir,
		Capture: runner.CaptureConfig{
			URLRegex:       captureRe,
			MaxBodyBytes:   *maxCaptureBytes,
			BlockImages:    *blockImages,
			EnableObserver: captureRe != nil,
		},
	}

	opts := browser.Options{
		Headless:    *headless,
		Proxy:       *proxy,
		UserAgent:   *userAgent,
		UserDataDir: *userDataDir,
		WindowWidth:  1366,
		WindowHeight: 768,
	}

	telemetry := obs.New(task.OutDir, logger)
	defer telemetry.Close()

	exec := runner.New(runner.Options{
		BrowserOptions:  opts,
		Policy:          policy,
		Telemetry:       telemetry,
		NetworkObserver: network.NewObserver(network.ObserverOptions{CaptureURLRegex: captureRe}),
	})

	if strings.TrimSpace(*addHeader) != "" {
		kv := strings.SplitN(*addHeader, "=", 2)
		if len(kv) != 2 || strings.TrimSpace(kv[0]) == "" {
			return fmt.Errorf("invalid -add-header, want k=v")
		}
		exec.SetInterceptHeader(strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1]))
	}

	if strings.TrimSpace(*loginURL) != "" {
		u := firstNonEmpty(*loginUser, os.Getenv("CRAWL_LOGIN_USER"))
		p := firstNonEmpty(*loginPass, os.Getenv("CRAWL_LOGIN_PASS"))
		exec.SetFormLogin(runner.FormLoginConfig{
			LoginURL:          *loginURL,
			Username:          u,
			Password:          p,
			UsernameSelector:  *loginUserSel,
			PasswordSelector:  *loginPassSel,
			SubmitSelector:    *loginSubmitSel,
			AfterSelector:     *loginAfterSel,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	result, err := exec.Run(ctx, task)
	if err != nil {
		telemetry.RecordError(task.ID, err)
		return err
	}

	outPath := filepath.Join(task.OutDir, "result.json")
	b, _ := json.MarshalIndent(result, "", "  ")
	if werr := os.WriteFile(outPath, b, 0o644); werr != nil {
		logger.Printf("write result.json failed: %v", werr)
	}
	logger.Printf("ok: wrote %s", outPath)
	return nil
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func runQueue(args []string, logger *log.Logger) error {
	fs := flag.NewFlagSet("v1-queue", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		seedFile    = fs.String("seed-file", "", "path to a newline-delimited url list file")
		dbPath      = fs.String("db", "queue.db", "sqlite db path for queue/state/dedup")
		outDir      = fs.String("out", "artifacts_queue", "artifacts output dir root")
		workers     = fs.Int("workers", runtime.NumCPU(), "worker count")
		timeout     = fs.Duration("timeout", 10*time.Minute, "overall timeout")
		maxAttempts = fs.Int("max-attempts", 3, "max attempts before giving up")

		allowDomains = fs.String("allow-domains", "", "comma-separated allowlist domains (recommended)")

		headless    = fs.Bool("headless", true, "run chrome in headless mode")
		proxy       = fs.String("proxy", "", "proxy server")
		userAgent   = fs.String("ua", "", "custom User-Agent")
		userDataDir = fs.String("user-data-dir", "", "Chrome user-data-dir (profile) for session reuse")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}

	runlog, err := obs.NewRunLogger(*outDir)
	if err != nil {
		return err
	}
	defer func() { _ = runlog.Close() }()
	logger = runlog.Logger

	st, err := sqlstore.Open(*dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if strings.TrimSpace(*seedFile) != "" {
		enq, err := runner.SeedFromFile(ctx, st, *seedFile)
		if err != nil {
			return err
		}
		logger.Printf("seeded %d urls", enq)
	}
	if _, err := st.RequeueFailed(ctx, *maxAttempts); err != nil {
		return err
	}

	policy := guard.Policy{
		AllowedDomains: splitCSV(*allowDomains),
		DeniedSchemes:  []string{"file", "chrome"},
		Budget: guard.Budget{
			Timeout:        *timeout,
			MaxScreenshots: 2,
			MaxHTMLBytes:   500_000,
		},
	}

	opts := browser.Options{
		Headless:     *headless,
		Proxy:        *proxy,
		UserAgent:    *userAgent,
		UserDataDir:  *userDataDir,
		WindowWidth:  1366,
		WindowHeight: 768,
	}

	// Per-task runner wrapper to set outdir as out/<taskId>/...
	base := runner.New(runner.Options{
		BrowserOptions:  opts,
		Policy:          policy,
		Telemetry:       obs.New(*outDir, logger),
		NetworkObserver: network.NewObserver(network.ObserverOptions{}),
	})

	src := runner.SQLiteQueue{Store: st}
	sink := runner.SQLiteSink{Store: st}

	pool := runner.Pool{
		Runner:   base,
		Workers:  *workers,
	}

	// Adapt TaskSource to inject outDir per task.
	adaptedSrc := taskSourceFunc(func(ctx context.Context) (runner.Task, error) {
		t, err := src.Next(ctx)
		if err != nil {
			return runner.Task{}, err
		}
		t.OutDir = filepath.Join(*outDir, t.ID)
		_ = os.MkdirAll(t.OutDir, 0o755)
		return t, nil
	})

	return pool.Run(ctx, adaptedSrc, sink)
}

func runCapture(args []string, logger *log.Logger) error {
	fs := flag.NewFlagSet("v1-capture", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		targetURL = fs.String("url", "https://example.com", "target URL")
		outDir    = fs.String("out", "artifacts_capture", "artifacts output dir")
		timeout   = fs.Duration("timeout", 2*time.Minute, "overall timeout")

		allowDomains      = fs.String("allow-domains", "example.com", "comma-separated allowlist domains")
		captureURLPattern = fs.String("capture-url", "", "regex to capture response bodies by url (required)")
		maxCaptureBytes   = fs.Int("capture-max-bytes", 1_000_000, "max bytes per captured response body")

		loginURL       = fs.String("login-url", "", "optional login page url; if set, will navigate and perform form login")
		loginUser      = fs.String("login-user", "", "login username (or set env CRAWL_LOGIN_USER)")
		loginPass      = fs.String("login-pass", "", "login password (or set env CRAWL_LOGIN_PASS)")
		loginUserSel   = fs.String("login-user-sel", "", "username input CSS selector")
		loginPassSel   = fs.String("login-pass-sel", "", "password input CSS selector")
		loginSubmitSel = fs.String("login-submit-sel", "", "submit button CSS selector")
		loginAfterSel  = fs.String("login-after-sel", "", "selector visible after login (optional)")

		triggerEval = fs.String("trigger-eval", "", "optional JS to evaluate after navigation (e.g. fetch('/api/profile'))")
		triggerWait = fs.Duration("trigger-wait", 1200*time.Millisecond, "wait after trigger-eval to allow XHR to complete")

		headless    = fs.Bool("headless", true, "run chrome in headless mode")
		proxy       = fs.String("proxy", "", "proxy server")
		userAgent   = fs.String("ua", "", "custom User-Agent")
		userDataDir = fs.String("user-data-dir", "", "Chrome user-data-dir (profile) for session reuse")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*captureURLPattern) == "" {
		return fmt.Errorf("-capture-url is required for v1-capture")
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}

	runlog, err := obs.NewRunLogger(*outDir)
	if err != nil {
		return err
	}
	defer func() { _ = runlog.Close() }()
	logger = runlog.Logger

	captureRe, err := regexp.Compile(*captureURLPattern)
	if err != nil {
		return err
	}

	policy := guard.Policy{
		AllowedDomains: splitCSV(*allowDomains),
		DeniedSchemes:  []string{"file", "chrome"},
		Budget: guard.Budget{
			Timeout:         *timeout,
			MaxScreenshots:  2,
			MaxHTMLBytes:    500_000,
			MaxCaptureBytes: *maxCaptureBytes,
		},
	}

	task := runner.Task{
		ID:        fmt.Sprintf("capture-%d", time.Now().Unix()),
		TargetURL: *targetURL,
		OutDir:    *outDir,
		Capture: runner.CaptureConfig{
			URLRegex:       captureRe,
			MaxBodyBytes:   *maxCaptureBytes,
			EnableObserver: true,
		},
		TriggerEval: *triggerEval,
		TriggerWait: *triggerWait,
	}

	opts := browser.Options{
		Headless:     *headless,
		Proxy:        *proxy,
		UserAgent:    *userAgent,
		UserDataDir:  *userDataDir,
		WindowWidth:  1366,
		WindowHeight: 768,
	}

	telemetry := obs.New(task.OutDir, logger)
	defer telemetry.Close()

	exec := runner.New(runner.Options{
		BrowserOptions:  opts,
		Policy:          policy,
		Telemetry:       telemetry,
		NetworkObserver: network.NewObserver(network.ObserverOptions{CaptureURLRegex: captureRe}),
	})

	if strings.TrimSpace(*loginURL) != "" {
		u := firstNonEmpty(*loginUser, os.Getenv("CRAWL_LOGIN_USER"))
		p := firstNonEmpty(*loginPass, os.Getenv("CRAWL_LOGIN_PASS"))
		exec.SetFormLogin(runner.FormLoginConfig{
			LoginURL:          *loginURL,
			Username:          u,
			Password:          p,
			UsernameSelector:  *loginUserSel,
			PasswordSelector:  *loginPassSel,
			SubmitSelector:    *loginSubmitSel,
			AfterSelector:     *loginAfterSel,
		})
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	res, err := exec.Run(ctx, task)
	if err != nil {
		return err
	}

	// Persist captured bodies to disk only (no DB writes for now).
	for i, c := range res.Network {
		bodyPath := ""
		if c.BodyText != "" {
			bodyPath = filepath.Join(task.OutDir, fmt.Sprintf("capture_%03d.json", i+1))
			_ = os.WriteFile(bodyPath, []byte(c.BodyText), 0o644)
		} else if c.BodyBase64 != "" {
			bodyPath = filepath.Join(task.OutDir, fmt.Sprintf("capture_%03d.b64", i+1))
			_ = os.WriteFile(bodyPath, []byte(c.BodyBase64), 0o644)
		}
		_ = bodyPath // reserved for future DB writeback
	}

	outPath := filepath.Join(task.OutDir, "result.json")
	b, _ := json.MarshalIndent(res, "", "  ")
	_ = os.WriteFile(outPath, b, 0o644)
	logger.Printf("ok: wrote %s", outPath)
	return nil
}

func runV2Queue(args []string, logger *log.Logger) error {
	fs := flag.NewFlagSet("v2-queue", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		workflowPath = fs.String("workflow", "", "path to workflow JSON file (required)")
		seedFile     = fs.String("seed-file", "", "path to a newline-delimited url list file")
		dbPath       = fs.String("db", "queue_v2.db", "sqlite db path for queue/state/dedup")
		outDir       = fs.String("out", "artifacts_v2_queue", "artifacts output dir root")
		workers      = fs.Int("workers", runtime.NumCPU(), "worker count")
		timeout      = fs.Duration("timeout", 10*time.Minute, "overall timeout")
		maxAttempts  = fs.Int("max-attempts", 3, "max attempts before giving up")
		allowDomains = fs.String("allow-domains", "", "comma-separated allowlist domains (recommended)")

		headless    = fs.Bool("headless", true, "run chrome in headless mode")
		proxy       = fs.String("proxy", "", "proxy server")
		userAgent   = fs.String("ua", "", "custom User-Agent")
		userDataDir = fs.String("user-data-dir", "", "Chrome user-data-dir (profile) for session reuse")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*workflowPath) == "" {
		return fmt.Errorf("-workflow is required")
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}

	runlog, err := obs.NewRunLogger(*outDir)
	if err != nil {
		return err
	}
	defer func() { _ = runlog.Close() }()
	logger = runlog.Logger

	wf, err := runner.LoadWorkflow(*workflowPath)
	if err != nil {
		return err
	}

	st, err := sqlstore.Open(*dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if strings.TrimSpace(*seedFile) != "" {
		enq, err := runner.SeedFromFile(ctx, st, *seedFile)
		if err != nil {
			return err
		}
		logger.Printf("seeded %d urls", enq)
	}
	if _, err := st.RequeueFailed(ctx, *maxAttempts); err != nil {
		return err
	}

	policy := guard.Policy{
		AllowedDomains: splitCSV(*allowDomains),
		DeniedSchemes:  []string{"file", "chrome"},
		Budget: guard.Budget{
			Timeout:        *timeout,
			MaxScreenshots: 3,
			MaxHTMLBytes:   800_000,
		},
	}

	opts := browser.Options{
		Headless:     *headless,
		Proxy:        *proxy,
		UserAgent:    *userAgent,
		UserDataDir:  *userDataDir,
		WindowWidth:  1366,
		WindowHeight: 768,
	}

	telemetry := obs.New(*outDir, logger)

	base := runner.New(runner.Options{
		BrowserOptions:  opts,
		Policy:          policy,
		Telemetry:       telemetry,
		NetworkObserver: nil, // configured by workflow if capture step exists
	})

	src := runner.SQLiteQueue{Store: st}
	sink := runner.SQLiteSink{Store: st}

	pool := runner.Pool{
		Runner:  base,
		Workers: *workers,
	}

	adaptedSrc := taskSourceFunc(func(ctx context.Context) (runner.Task, error) {
		t, err := src.Next(ctx)
		if err != nil {
			return runner.Task{}, err
		}
		t.OutDir = filepath.Join(*outDir, t.ID)
		_ = os.MkdirAll(t.OutDir, 0o755)

		if err := base.ApplyWorkflow(ctx, &t, wf); err != nil {
			return runner.Task{}, err
		}
		return t, nil
	})

	return pool.Run(ctx, adaptedSrc, sink)
}

type taskSourceFunc func(ctx context.Context) (runner.Task, error)

func (f taskSourceFunc) Next(ctx context.Context) (runner.Task, error) { return f(ctx) }

func runV2(args []string, logger *log.Logger) error {
	fs := flag.NewFlagSet("v2", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		workflowPath = fs.String("workflow", "", "path to workflow JSON file (required)")
		targetURL    = fs.String("url", "", "target URL")
		outDir       = fs.String("out", "artifacts_v2", "artifacts output dir")
		timeout      = fs.Duration("timeout", 2*time.Minute, "overall timeout")
		allowDomains = fs.String("allow-domains", "", "comma-separated allowlist domains (recommended)")

		headless    = fs.Bool("headless", true, "run chrome in headless mode")
		proxy       = fs.String("proxy", "", "proxy server")
		userAgent   = fs.String("ua", "", "custom User-Agent")
		userDataDir = fs.String("user-data-dir", "", "Chrome user-data-dir (profile) for session reuse")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*workflowPath) == "" {
		return fmt.Errorf("-workflow is required")
	}
	if strings.TrimSpace(*targetURL) == "" {
		return fmt.Errorf("-url is required")
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		return err
	}

	runlog, err := obs.NewRunLogger(*outDir)
	if err != nil {
		return err
	}
	defer func() { _ = runlog.Close() }()
	logger = runlog.Logger

	wf, err := runner.LoadWorkflow(*workflowPath)
	if err != nil {
		return err
	}

	policy := guard.Policy{
		AllowedDomains: splitCSV(*allowDomains),
		DeniedSchemes:  []string{"file", "chrome"},
		Budget: guard.Budget{
			Timeout:        *timeout,
			MaxScreenshots: 5,
			MaxHTMLBytes:   1_000_000,
		},
	}

	task := runner.Task{
		ID:        fmt.Sprintf("v2-%d", time.Now().Unix()),
		TargetURL: *targetURL,
		OutDir:    *outDir,
	}

	opts := browser.Options{
		Headless:     *headless,
		Proxy:        *proxy,
		UserAgent:    *userAgent,
		UserDataDir:  *userDataDir,
		WindowWidth:  1366,
		WindowHeight: 768,
	}

	telemetry := obs.New(task.OutDir, logger)
	defer telemetry.Close()

	exec := runner.New(runner.Options{
		BrowserOptions:  opts,
		Policy:          policy,
		Telemetry:       telemetry,
		NetworkObserver: nil, // will be configured by workflow if capture step exists
	})

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	if err := exec.ApplyWorkflow(ctx, &task, wf); err != nil {
		return err
	}

	res, err := exec.Run(ctx, task)
	if err != nil {
		return err
	}

	// Persist captured bodies to disk (same as v1-capture).
	for i, c := range res.Network {
		if c.BodyText != "" {
			_ = os.WriteFile(filepath.Join(task.OutDir, fmt.Sprintf("capture_%03d.json", i+1)), []byte(c.BodyText), 0o644)
		} else if c.BodyBase64 != "" {
			_ = os.WriteFile(filepath.Join(task.OutDir, fmt.Sprintf("capture_%03d.b64", i+1)), []byte(c.BodyBase64), 0o644)
		}
	}

	outPath := filepath.Join(task.OutDir, "result.json")
	b, _ := json.MarshalIndent(res, "", "  ")
	_ = os.WriteFile(outPath, b, 0o644)
	logger.Printf("ok: wrote %s", outPath)
	return nil
}

