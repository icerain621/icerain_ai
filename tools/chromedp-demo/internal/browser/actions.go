package browser

import (
	"context"

	"github.com/chromedp/chromedp"
)

type PageSnapshot struct {
	Title    string
	URL      string
	HTML     string
	Text     string
}

func BasicSnapshot(ctx context.Context, htmlSelector string, maxHTMLBytes int) (PageSnapshot, error) {
	var (
		title string
		url   string
		html  string
		text  string
	)

	actions := []chromedp.Action{
		chromedp.Title(&title),
		chromedp.Location(&url),
		chromedp.OuterHTML(htmlSelector, &html, chromedp.ByQuery),
		chromedp.Text(htmlSelector, &text, chromedp.ByQuery, chromedp.NodeVisible),
	}

	if err := chromedp.Run(ctx, actions...); err != nil {
		return PageSnapshot{}, err
	}
	if maxHTMLBytes > 0 && len(html) > maxHTMLBytes {
		html = html[:maxHTMLBytes]
	}
	return PageSnapshot{Title: title, URL: url, HTML: html, Text: text}, nil
}

