package login

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type Cookie struct {
	Name   string
	Value  string
	Domain string
	Path   string
	Secure bool
	HTTPOnly bool
}

// ApplyCookies best-effort sets cookies via CDP.
// Note: setting HttpOnly cookies may not work in some contexts; prefer profile reuse when possible.
func ApplyCookies(ctx context.Context, cookies []Cookie) error {
	for _, c := range cookies {
		act := network.SetCookie(c.Name, c.Value).
			WithDomain(c.Domain).
			WithPath(c.Path).
			WithSecure(c.Secure).
			WithHTTPOnly(c.HTTPOnly)
		ok, err := act.Do(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("set cookie failed: %s", c.Name)
		}
	}
	return nil
}

type FormSpec struct {
	UsernameSelector string
	PasswordSelector string
	SubmitSelector   string
	AfterSelector    string // selector that should appear after login
}

type Credentials struct {
	Username string
	Password string
}

func FormLogin(ctx context.Context, spec FormSpec, cred Credentials) error {
	if spec.UsernameSelector == "" || spec.PasswordSelector == "" || spec.SubmitSelector == "" {
		return fmt.Errorf("invalid form spec")
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	actions := []chromedp.Action{
		chromedp.WaitVisible(spec.UsernameSelector, chromedp.ByQuery),
		chromedp.SetValue(spec.UsernameSelector, "", chromedp.ByQuery),
		chromedp.SendKeys(spec.UsernameSelector, cred.Username, chromedp.ByQuery),
		chromedp.SetValue(spec.PasswordSelector, "", chromedp.ByQuery),
		chromedp.SendKeys(spec.PasswordSelector, cred.Password, chromedp.ByQuery),
		chromedp.Click(spec.SubmitSelector, chromedp.ByQuery),
	}
	if spec.AfterSelector != "" {
		actions = append(actions, chromedp.WaitVisible(spec.AfterSelector, chromedp.ByQuery))
	}
	return chromedp.Run(timeoutCtx, actions...)
}

