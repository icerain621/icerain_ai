package httpapi

import "time"

type BrowserOptions struct {
	Headless    bool   `json:"headless"`
	Proxy       string `json:"proxy,omitempty"`
	UserAgent   string `json:"userAgent,omitempty"`
	UserDataDir string `json:"userDataDir,omitempty"`
	WindowWidth int    `json:"windowWidth,omitempty"`
	WindowHeight int   `json:"windowHeight,omitempty"`
}

type GuardPolicy struct {
	AllowDomains []string `json:"allowDomains,omitempty"`
	DeniedSchemes []string `json:"deniedSchemes,omitempty"`
	TimeoutMs    int64  `json:"timeoutMs,omitempty"`
	MaxScreenshots int  `json:"maxScreenshots,omitempty"`
	MaxHTMLBytes int     `json:"maxHTMLBytes,omitempty"`
	MaxCaptureBytes int  `json:"maxCaptureBytes,omitempty"`
}

type FormLogin struct {
	LoginURL          string `json:"loginUrl,omitempty"`
	Username          string `json:"username,omitempty"`
	Password          string `json:"password,omitempty"`
	UsernameSelector  string `json:"usernameSelector,omitempty"`
	PasswordSelector  string `json:"passwordSelector,omitempty"`
	SubmitSelector    string `json:"submitSelector,omitempty"`
	AfterSelector     string `json:"afterSelector,omitempty"`
}

type Capture struct {
	CaptureURLRegex string `json:"captureUrlRegex,omitempty"`
	MaxBodyBytes    int    `json:"maxBodyBytes,omitempty"`
	TriggerEval     string `json:"triggerEval,omitempty"`
	TriggerWaitMs   int    `json:"triggerWaitMs,omitempty"`
	BlockImages     bool   `json:"blockImages,omitempty"`
}

type RunV0Request struct {
	URL     string       `json:"url"`
	OutDir  string       `json:"outDir"`
	Browser BrowserOptions `json:"browser,omitempty"`
	Policy  GuardPolicy   `json:"policy,omitempty"`
	Login   FormLogin     `json:"login,omitempty"`
	Capture Capture       `json:"capture,omitempty"`
}

type RunV1CaptureRequest struct {
	URL     string       `json:"url"`
	OutDir  string       `json:"outDir"`
	Browser BrowserOptions `json:"browser,omitempty"`
	Policy  GuardPolicy   `json:"policy,omitempty"`
	Login   FormLogin     `json:"login,omitempty"`
	Capture Capture       `json:"capture"`
}

type RunV2Request struct {
	URL        string       `json:"url"`
	OutDir     string       `json:"outDir"`
	WorkflowPath string     `json:"workflowPath"`
	Browser     BrowserOptions `json:"browser,omitempty"`
	Policy      GuardPolicy   `json:"policy,omitempty"`
}

type StartV2QueueRequest struct {
	SeedFile     string       `json:"seedFile,omitempty"`
	DBPath       string       `json:"dbPath,omitempty"`
	OutDir       string       `json:"outDir"`
	Workers      int          `json:"workers,omitempty"`
	MaxAttempts  int          `json:"maxAttempts,omitempty"`
	WorkflowPath string       `json:"workflowPath"`
	Browser      BrowserOptions `json:"browser,omitempty"`
	Policy       GuardPolicy   `json:"policy,omitempty"`
}

type RunResponse struct {
	OutDir string `json:"outDir"`
	Result any    `json:"result"`
}

type StartJobResponse struct {
	JobID string `json:"jobId"`
	OutDir string `json:"outDir"`
}

type HealthResponse struct {
	OK bool `json:"ok"`
	Now time.Time `json:"now"`
}

