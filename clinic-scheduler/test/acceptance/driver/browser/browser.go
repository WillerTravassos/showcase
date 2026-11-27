// Package browser drives the deployed portals in headless Chromium through chromedp, for
// behaviour that only a real browser shows (ADR 0017).
package browser

import (
	"context"
	"fmt"
	"html"
	nethttp "net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// pageTimeout bounds starting the browser and loading one page with its frames.
const pageTimeout = 30 * time.Second

// FrameLoadError opens a page on another site (a local server on 127.0.0.1) that embeds
// url in an iframe, and returns the network error of the frame's document, for example
// "net::ERR_BLOCKED_BY_RESPONSE" when the browser refuses to show it. It returns an
// empty string when the frame loaded.
func FrameLoadError(ctx context.Context, url string) (string, error) {
	otherSite := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, _ *nethttp.Request) {
		_, _ = fmt.Fprintf(w, `<!doctype html><title>Another site</title><iframe src="%s"></iframe>`,
			html.EscapeString(url)) // a write error shows up as a failed page load below
	}))
	defer otherSite.Close()

	// Certificate trust is checked by the HTTP driver; here it would only hide the framing
	// behaviour behind a certificate error.
	opts := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.Flag("ignore-certificate-errors", true))
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()
	tabCtx, cancelTab := chromedp.NewContext(allocCtx)
	defer cancelTab()
	tabCtx, cancelTimeout := context.WithTimeout(tabCtx, pageTimeout)
	defer cancelTimeout()

	var (
		mu       sync.Mutex
		frameReq network.RequestID
		loadErr  string
	)
	err := chromedp.Listen(tabCtx, func(ev any) {
		mu.Lock()
		defer mu.Unlock()
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if e.Type == network.ResourceTypeDocument && e.Request.URL == url {
				frameReq = e.RequestID
			}
		case *network.EventLoadingFailed:
			if e.RequestID == frameReq {
				loadErr = e.ErrorText
			}
		}
	})
	if err != nil {
		return "", fmt.Errorf("listen to browser events: %w", err)
	}
	if _, err = chromedp.Call(tabCtx, network.Enable, network.EnableParams{}); err != nil {
		return "", fmt.Errorf("enable network events: %w", err)
	}
	if err = chromedp.Do(tabCtx, chromedp.Navigate(otherSite.URL)); err != nil {
		return "", fmt.Errorf("open %s: %w", otherSite.URL, err)
	}

	mu.Lock()
	defer mu.Unlock()
	if frameReq == "" {
		return "", fmt.Errorf("the frame never requested %s", url)
	}
	return loadErr, nil
}
