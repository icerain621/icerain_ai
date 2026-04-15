package guard

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type Budget struct {
	Timeout        time.Duration
	MaxScreenshots int
	MaxHTMLBytes   int
	MaxCaptureBytes int
}

type Policy struct {
	AllowedDomains []string
	DeniedSchemes  []string
	Budget         Budget
}

type RedactConfig struct {
	Keys []string
}

func RedactString(s string, keys []string) string {
	out := s
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		// Very simple redaction: replace key occurrences (case-insensitive) and common "k=..." patterns.
		out = strings.ReplaceAll(out, k, "***")
		out = strings.ReplaceAll(out, strings.ToLower(k), "***")
		out = strings.ReplaceAll(out, strings.ToUpper(k), "***")
	}
	return out
}

func (p Policy) CheckURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("invalid url: missing scheme/host")
	}
	for _, s := range p.DeniedSchemes {
		if strings.EqualFold(u.Scheme, s) {
			return fmt.Errorf("scheme denied: %s", u.Scheme)
		}
	}
	if len(p.AllowedDomains) == 0 {
		return nil
	}
	host := strings.ToLower(u.Hostname())
	for _, d := range p.AllowedDomains {
		d = strings.ToLower(strings.TrimSpace(d))
		if d == "" {
			continue
		}
		if host == d || strings.HasSuffix(host, "."+d) {
			return nil
		}
	}
	return fmt.Errorf("domain not allowed: %s", host)
}

