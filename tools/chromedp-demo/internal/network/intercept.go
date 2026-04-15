package network

import (
	"context"
	"fmt"
	"regexp"

	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
)

type InterceptRule struct {
	URLRegex *regexp.Regexp
	Action   InterceptAction
}

type InterceptAction struct {
	Block bool
	// AddHeaders adds/overrides request headers (key is case-insensitive by Chrome).
	AddHeaders map[string]string
}

type Interceptor struct {
	rules []InterceptRule
}

func NewInterceptor(rules []InterceptRule) *Interceptor {
	return &Interceptor{rules: rules}
}

func (i *Interceptor) Enable(ctx context.Context) error {
	// Enable Fetch domain for request interception.
	return fetch.Enable().Do(ctx)
}

func (i *Interceptor) Handle(ctx context.Context, ev *fetch.EventRequestPaused) error {
	if ev == nil || ev.Request == nil {
		return nil
	}
	url := ev.Request.URL

	var action *InterceptAction
	for _, r := range i.rules {
		if r.URLRegex != nil && r.URLRegex.MatchString(url) {
			action = &r.Action
			break
		}
	}
	if action == nil {
		return fetch.ContinueRequest(ev.RequestID).Do(ctx)
	}

	if action.Block {
		return fetch.FailRequest(ev.RequestID, network.ErrorReasonBlockedByClient).Do(ctx)
	}

	cont := fetch.ContinueRequest(ev.RequestID)
	if len(action.AddHeaders) > 0 {
		var hdrs []*fetch.HeaderEntry
		for k, v := range action.AddHeaders {
			hdrs = append(hdrs, &fetch.HeaderEntry{Name: k, Value: v})
		}
		cont = cont.WithHeaders(hdrs)
	}

	if err := cont.Do(ctx); err != nil {
		return fmt.Errorf("continue request failed: %w", err)
	}
	return nil
}

