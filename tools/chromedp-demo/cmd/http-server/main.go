package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/browser"
	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/guard"
	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/httpapi"
	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/network"
	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/obs"
	"github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/runner"
	sqlstore "github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/store/sqlite"
)

func main() {
	r := gin.New()
	r.Use(gin.Recovery())

	jobs := httpapi.NewJobManager()

	// Serve OpenAPI spec for easy import into Swagger/Apifox/Postman.
	r.GET("/openapi.yaml", func(c *gin.Context) {
		c.File("docs/openapi.yaml")
	})

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, httpapi.HealthResponse{OK: true, Now: time.Now()})
	})

	api := r.Group("/api")
	{
		api.POST("/v0/run", func(c *gin.Context) {
			var req httpapi.RunV0Request
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			res, outDir, err := runV0(c.Request.Context(), req)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, httpapi.RunResponse{OutDir: outDir, Result: res})
		})

		api.POST("/v1/capture/run", func(c *gin.Context) {
			var req httpapi.RunV1CaptureRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			res, outDir, err := runV1Capture(c.Request.Context(), req)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, httpapi.RunResponse{OutDir: outDir, Result: res})
		})

		api.POST("/v2/run", func(c *gin.Context) {
			var req httpapi.RunV2Request
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			res, outDir, err := runV2(c.Request.Context(), req)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, httpapi.RunResponse{OutDir: outDir, Result: res})
		})

		// Queue workflows are long-running: run as async job.
		api.POST("/v2/queue/start", func(c *gin.Context) {
			var req httpapi.StartV2QueueRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			jobID := newID("job")
			outDir := req.OutDir
			if outDir == "" {
				outDir = filepath.Join("artifacts_http", jobID)
				_ = os.MkdirAll(outDir, 0o755)
			}
			jobs.Create(jobID, "v2-queue", outDir)
			jobs.RunAsync(context.Background(), jobID, func(ctx context.Context) (any, error) {
				return nil, runV2Queue(ctx, req, outDir)
			})
			c.JSON(http.StatusAccepted, httpapi.StartJobResponse{JobID: jobID, OutDir: outDir})
		})

		api.GET("/jobs/:id", func(c *gin.Context) {
			id := c.Param("id")
			j, ok := jobs.Get(id)
			if !ok {
				c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
				return
			}
			c.JSON(http.StatusOK, j)
		})
	}

	addr := getenv("HTTP_ADDR", "127.0.0.1:19090")
	_ = r.Run(addr)
}

func runV0(ctx context.Context, req httpapi.RunV0Request) (any, string, error) {
	if req.URL == "" {
		return nil, "", fmt.Errorf("url is required")
	}
	outDir := defaultOut(req.OutDir, "artifacts_http/v0")
	runlog, err := obs.NewRunLogger(outDir)
	if err != nil {
		return nil, outDir, err
	}
	defer func() { _ = runlog.Close() }()

	policy := toPolicy(req.Policy, 90*time.Second)
	opts := toBrowser(req.Browser)

	var captureRe *regexp.Regexp
	if req.Capture.CaptureURLRegex != "" {
		captureRe, err = regexp.Compile(req.Capture.CaptureURLRegex)
		if err != nil {
			return nil, outDir, err
		}
	}

	task := runner.Task{
		ID:        newID("v0"),
		TargetURL: req.URL,
		OutDir:    outDir,
		Capture: runner.CaptureConfig{
			URLRegex:       captureRe,
			MaxBodyBytes:   req.Capture.MaxBodyBytes,
			BlockImages:    req.Capture.BlockImages,
			EnableObserver: captureRe != nil,
		},
		TriggerEval: req.Capture.TriggerEval,
		TriggerWait: time.Duration(req.Capture.TriggerWaitMs) * time.Millisecond,
	}

	telemetry := obs.New(outDir, runlog.Logger)
	defer telemetry.Close()

	exec := runner.New(runner.Options{
		BrowserOptions:  opts,
		Policy:          policy,
		Telemetry:       telemetry,
		NetworkObserver: network.NewObserver(network.ObserverOptions{CaptureURLRegex: captureRe}),
	})

	if req.Login.LoginURL != "" {
		exec.SetFormLogin(runner.FormLoginConfig{
			LoginURL:          req.Login.LoginURL,
			Username:          req.Login.Username,
			Password:          req.Login.Password,
			UsernameSelector:  req.Login.UsernameSelector,
			PasswordSelector:  req.Login.PasswordSelector,
			SubmitSelector:    req.Login.SubmitSelector,
			AfterSelector:     req.Login.AfterSelector,
		})
	}

	timeout := policy.Budget.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := exec.Run(cctx, task)
	if err != nil {
		return nil, outDir, err
	}
	writeResult(outDir, res)
	return res, outDir, nil
}

func runV1Capture(ctx context.Context, req httpapi.RunV1CaptureRequest) (any, string, error) {
	if req.URL == "" {
		return nil, "", fmt.Errorf("url is required")
	}
	if req.Capture.CaptureURLRegex == "" {
		return nil, "", fmt.Errorf("capture.captureUrlRegex is required")
	}
	outDir := defaultOut(req.OutDir, "artifacts_http/v1_capture")
	runlog, err := obs.NewRunLogger(outDir)
	if err != nil {
		return nil, outDir, err
	}
	defer func() { _ = runlog.Close() }()

	captureRe, err := regexp.Compile(req.Capture.CaptureURLRegex)
	if err != nil {
		return nil, outDir, err
	}

	policy := toPolicy(req.Policy, 2*time.Minute)
	opts := toBrowser(req.Browser)

	task := runner.Task{
		ID:        newID("v1cap"),
		TargetURL: req.URL,
		OutDir:    outDir,
		Capture: runner.CaptureConfig{
			URLRegex:       captureRe,
			MaxBodyBytes:   req.Capture.MaxBodyBytes,
			BlockImages:    req.Capture.BlockImages,
			EnableObserver: true,
		},
		TriggerEval: req.Capture.TriggerEval,
		TriggerWait: time.Duration(req.Capture.TriggerWaitMs) * time.Millisecond,
	}

	telemetry := obs.New(outDir, runlog.Logger)
	defer telemetry.Close()

	exec := runner.New(runner.Options{
		BrowserOptions:  opts,
		Policy:          policy,
		Telemetry:       telemetry,
		NetworkObserver: network.NewObserver(network.ObserverOptions{CaptureURLRegex: captureRe}),
	})

	if req.Login.LoginURL != "" {
		exec.SetFormLogin(runner.FormLoginConfig{
			LoginURL:          req.Login.LoginURL,
			Username:          req.Login.Username,
			Password:          req.Login.Password,
			UsernameSelector:  req.Login.UsernameSelector,
			PasswordSelector:  req.Login.PasswordSelector,
			SubmitSelector:    req.Login.SubmitSelector,
			AfterSelector:     req.Login.AfterSelector,
		})
	}

	cctx, cancel := context.WithTimeout(ctx, policy.Budget.Timeout)
	defer cancel()
	res, err := exec.Run(cctx, task)
	if err != nil {
		return nil, outDir, err
	}

	for i, c := range res.Network {
		if c.BodyText != "" {
			_ = os.WriteFile(filepath.Join(outDir, fmt.Sprintf("capture_%03d.json", i+1)), []byte(c.BodyText), 0o644)
		} else if c.BodyBase64 != "" {
			_ = os.WriteFile(filepath.Join(outDir, fmt.Sprintf("capture_%03d.b64", i+1)), []byte(c.BodyBase64), 0o644)
		}
	}
	writeResult(outDir, res)
	return res, outDir, nil
}

func runV2(ctx context.Context, req httpapi.RunV2Request) (any, string, error) {
	if req.URL == "" {
		return nil, "", fmt.Errorf("url is required")
	}
	if req.WorkflowPath == "" {
		return nil, "", fmt.Errorf("workflowPath is required")
	}
	outDir := defaultOut(req.OutDir, "artifacts_http/v2")
	runlog, err := obs.NewRunLogger(outDir)
	if err != nil {
		return nil, outDir, err
	}
	defer func() { _ = runlog.Close() }()

	wf, err := runner.LoadWorkflow(req.WorkflowPath)
	if err != nil {
		return nil, outDir, err
	}

	policy := toPolicy(req.Policy, 2*time.Minute)
	opts := toBrowser(req.Browser)

	task := runner.Task{
		ID:        newID("v2"),
		TargetURL: req.URL,
		OutDir:    outDir,
	}

	telemetry := obs.New(outDir, runlog.Logger)
	defer telemetry.Close()

	exec := runner.New(runner.Options{
		BrowserOptions:  opts,
		Policy:          policy,
		Telemetry:       telemetry,
		NetworkObserver: nil,
	})

	cctx, cancel := context.WithTimeout(ctx, policy.Budget.Timeout)
	defer cancel()

	if err := exec.ApplyWorkflow(cctx, &task, wf); err != nil {
		return nil, outDir, err
	}
	res, err := exec.Run(cctx, task)
	if err != nil {
		return nil, outDir, err
	}
	for i, c := range res.Network {
		if c.BodyText != "" {
			_ = os.WriteFile(filepath.Join(outDir, fmt.Sprintf("capture_%03d.json", i+1)), []byte(c.BodyText), 0o644)
		} else if c.BodyBase64 != "" {
			_ = os.WriteFile(filepath.Join(outDir, fmt.Sprintf("capture_%03d.b64", i+1)), []byte(c.BodyBase64), 0o644)
		}
	}
	writeResult(outDir, res)
	return res, outDir, nil
}

func runV2Queue(ctx context.Context, req httpapi.StartV2QueueRequest, outDir string) error {
	if req.WorkflowPath == "" {
		return fmt.Errorf("workflowPath is required")
	}
	if outDir == "" {
		return fmt.Errorf("outDir is required")
	}
	wf, err := runner.LoadWorkflow(req.WorkflowPath)
	if err != nil {
		return err
	}

	dbPath := req.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(outDir, "queue_v2.db")
	}
	st, err := sqlstore.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	if req.SeedFile != "" {
		if _, err := runner.SeedFromFile(ctx, st, req.SeedFile); err != nil {
			return err
		}
	}
	maxAttempts := req.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if _, err := st.RequeueFailed(ctx, maxAttempts); err != nil {
		return err
	}

	policy := toPolicy(req.Policy, 10*time.Minute)
	opts := toBrowser(req.Browser)

	runlog, err := obs.NewRunLogger(outDir)
	if err != nil {
		return err
	}
	defer func() { _ = runlog.Close() }()

	telemetry := obs.New(outDir, runlog.Logger)
	defer telemetry.Close()

	exec := runner.New(runner.Options{
		BrowserOptions:  opts,
		Policy:          policy,
		Telemetry:       telemetry,
		NetworkObserver: nil,
	})

	workers := req.Workers
	if workers <= 0 {
		workers = 2
	}

	src := runner.SQLiteQueue{Store: st}
	sink := runner.SQLiteSink{Store: st}
	pool := runner.Pool{Runner: exec, Workers: workers}

	adaptedSrc := taskSourceFunc(func(ctx context.Context) (runner.Task, error) {
		t, err := src.Next(ctx)
		if err != nil {
			return runner.Task{}, err
		}
		t.OutDir = filepath.Join(outDir, t.ID)
		_ = os.MkdirAll(t.OutDir, 0o755)
		if err := exec.ApplyWorkflow(ctx, &t, wf); err != nil {
			return runner.Task{}, err
		}
		return t, nil
	})

	return pool.Run(ctx, adaptedSrc, sink)
}

type taskSourceFunc func(ctx context.Context) (runner.Task, error)

func (f taskSourceFunc) Next(ctx context.Context) (runner.Task, error) { return f(ctx) }

func toBrowser(b httpapi.BrowserOptions) browser.Options {
	ww := b.WindowWidth
	wh := b.WindowHeight
	if ww == 0 {
		ww = 1366
	}
	if wh == 0 {
		wh = 768
	}
	return browser.Options{
		Headless:     b.Headless || b.Headless == false, // default true handled by caller via JSON
		Proxy:        b.Proxy,
		UserAgent:    b.UserAgent,
		UserDataDir:  b.UserDataDir,
		WindowWidth:  ww,
		WindowHeight: wh,
	}
}

func toPolicy(p httpapi.GuardPolicy, defTimeout time.Duration) guard.Policy {
	timeout := defTimeout
	if p.TimeoutMs > 0 {
		timeout = time.Duration(p.TimeoutMs) * time.Millisecond
	}
	denied := p.DeniedSchemes
	if len(denied) == 0 {
		denied = []string{"file", "chrome"}
	}
	return guard.Policy{
		AllowedDomains: p.AllowDomains,
		DeniedSchemes:  denied,
		Budget: guard.Budget{
			Timeout:         timeout,
			MaxScreenshots:  maxI(p.MaxScreenshots, 5),
			MaxHTMLBytes:    maxI(p.MaxHTMLBytes, 1_000_000),
			MaxCaptureBytes: p.MaxCaptureBytes,
		},
	}
}

func maxI(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func defaultOut(given, def string) string {
	if given != "" {
		_ = os.MkdirAll(given, 0o755)
		return given
	}
	_ = os.MkdirAll(def, 0o755)
	return def
}

func writeResult(outDir string, res any) {
	b, _ := json.MarshalIndent(res, "", "  ")
	_ = os.WriteFile(filepath.Join(outDir, "result.json"), b, 0o644)
}

func newID(prefix string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return prefix + "-" + hex.EncodeToString(b[:])
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

