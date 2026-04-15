package network

import (
	"context"
	"regexp"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
)

type ObserverOptions struct {
	CaptureURLRegex *regexp.Regexp
}

type CapturedResponse struct {
	URL         string    `json:"url"`
	Status      int64     `json:"status"`
	MIMEType    string    `json:"mimeType"`
	RemoteIP    string    `json:"remoteIP,omitempty"`
	FromDisk    bool      `json:"fromDiskCache,omitempty"`
	FromService bool      `json:"fromServiceWorker,omitempty"`
	BodyBase64  string    `json:"bodyBase64,omitempty"`
	BodyText    string    `json:"bodyText,omitempty"`
	CapturedAt  time.Time `json:"capturedAt"`
	Truncated   bool      `json:"truncated,omitempty"`
}

type Observer struct {
	opts ObserverOptions

	mu        sync.Mutex
	responses []CapturedResponse
}

func NewObserver(opts ObserverOptions) *Observer {
	return &Observer{opts: opts}
}

func (o *Observer) ShouldCapture(url string) bool {
	if o.opts.CaptureURLRegex == nil {
		return false
	}
	return o.opts.CaptureURLRegex.MatchString(url)
}

func (o *Observer) AddCaptured(r CapturedResponse) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.responses = append(o.responses, r)
}

func (o *Observer) Snapshot() []CapturedResponse {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]CapturedResponse, len(o.responses))
	copy(out, o.responses)
	return out
}

type ResponseMeta struct {
	RequestID network.RequestID
	URL       string
	Status    int64
	MIMEType  string
}

type MetaStore struct {
	mu sync.Mutex
	m  map[network.RequestID]ResponseMeta
}

func NewMetaStore() *MetaStore { return &MetaStore{m: map[network.RequestID]ResponseMeta{}} }

func (s *MetaStore) Put(meta ResponseMeta) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[meta.RequestID] = meta
}

func (s *MetaStore) Get(id network.RequestID) (ResponseMeta, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.m[id]
	return meta, ok
}

// CaptureResponseBody reads response body for a request id.
// NOTE: Caller must ensure Network domain is enabled and events provide the request id.
func CaptureResponseBody(ctx context.Context, reqID network.RequestID) (body []byte, base64Encoded bool, err error) {
	return network.GetResponseBody(reqID).Do(ctx)
}

