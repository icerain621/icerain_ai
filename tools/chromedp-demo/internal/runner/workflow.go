package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	netobs "github.com/icerain621/icerain_ai/tools/chromedp-demo/internal/network"
)

// Workflow is a minimal, production-oriented workflow DSL (JSON).
// It is intentionally small: enough to compose the existing capabilities in Runner.
type Workflow struct {
	Version int            `json:"version"`
	Name    string         `json:"name,omitempty"`
	Steps   []WorkflowStep `json:"steps"`
}

type WorkflowStep struct {
	Kind string `json:"kind"`

	// navigate
	URL string `json:"url,omitempty"`

	// login_form
	LoginURL          string `json:"loginUrl,omitempty"`
	Username          string `json:"username,omitempty"` // can be "env:VAR"
	Password          string `json:"password,omitempty"` // can be "env:VAR"
	UsernameSelector  string `json:"usernameSelector,omitempty"`
	PasswordSelector  string `json:"passwordSelector,omitempty"`
	SubmitSelector    string `json:"submitSelector,omitempty"`
	AfterSelector     string `json:"afterSelector,omitempty"`

	// capture
	CaptureURLRegex string `json:"captureUrlRegex,omitempty"`
	MaxBodyBytes    int    `json:"maxBodyBytes,omitempty"`

	// trigger_eval
	JS      string `json:"js,omitempty"`
	WaitMs  int    `json:"waitMs,omitempty"`
}

func LoadWorkflow(path string) (*Workflow, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wf Workflow
	if err := json.Unmarshal(b, &wf); err != nil {
		return nil, err
	}
	if wf.Version == 0 {
		wf.Version = 1
	}
	if wf.Version != 1 {
		return nil, fmt.Errorf("unsupported workflow version: %d", wf.Version)
	}
	if len(wf.Steps) == 0 {
		return nil, fmt.Errorf("workflow has no steps")
	}
	return &wf, nil
}

// ApplyWorkflow mutates the runner configuration + task fields based on steps.
// It does not execute browser actions directly; it composes existing Runner logic.
func (r *Runner) ApplyWorkflow(ctx context.Context, task *Task, wf *Workflow) error {
	if task == nil || wf == nil {
		return fmt.Errorf("nil task/workflow")
	}

	var captureRe *regexp.Regexp
	maxBody := 0
	for _, st := range wf.Steps {
		switch strings.ToLower(strings.TrimSpace(st.Kind)) {
		case "navigate":
			// handled at execution time by setting task.TargetURL
		case "login_form":
			u := resolveSecret(ctx, st.Username)
			p := resolveSecret(ctx, st.Password)
			r.SetFormLogin(FormLoginConfig{
				LoginURL:          st.LoginURL,
				Username:          u,
				Password:          p,
				UsernameSelector:  st.UsernameSelector,
				PasswordSelector:  st.PasswordSelector,
				SubmitSelector:    st.SubmitSelector,
				AfterSelector:     st.AfterSelector,
			})
		case "capture":
			if st.CaptureURLRegex == "" {
				return fmt.Errorf("capture step missing captureUrlRegex")
			}
			re, err := regexp.Compile(st.CaptureURLRegex)
			if err != nil {
				return fmt.Errorf("invalid captureUrlRegex: %w", err)
			}
			captureRe = re
			if st.MaxBodyBytes > 0 {
				maxBody = st.MaxBodyBytes
			}
		case "trigger_eval":
			task.TriggerEval = st.JS
			if st.WaitMs > 0 {
				task.TriggerWait = time.Duration(st.WaitMs) * time.Millisecond
			}
		default:
			return fmt.Errorf("unknown workflow step kind: %s", st.Kind)
		}
	}

	if captureRe != nil {
		task.Capture.EnableObserver = true
		task.Capture.URLRegex = captureRe
		if maxBody > 0 {
			task.Capture.MaxBodyBytes = maxBody
		}
		// Ensure observer exists.
		if r.opts.NetworkObserver == nil {
			r.opts.NetworkObserver = netobs.NewObserver(netobs.ObserverOptions{CaptureURLRegex: captureRe})
		}
	}

	return nil
}

func resolveSecret(_ context.Context, v string) string {
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "env:") {
		key := strings.TrimSpace(strings.TrimPrefix(v, "env:"))
		if key == "" {
			return ""
		}
		return os.Getenv(key)
	}
	return v
}

