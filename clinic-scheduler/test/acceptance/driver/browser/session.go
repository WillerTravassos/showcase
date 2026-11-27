package browser

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"strings"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// Session is one browser tab with a fresh profile, on a device that prefers a colour
// scheme, with scripts disabled. Its cookies persist from one page to the next until
// [Session.Close]. It is not safe for concurrent use.
type Session struct {
	ctx   context.Context //nolint:containedctx // The tab lives as long as the session.
	close context.CancelFunc
}

// NewSession starts a browser on a device that prefers the colorScheme ("light" or
// "dark"). With scripts disabled, nothing restyles a page after it loads, so what a page
// shows is what it showed on first paint.
func NewSession(ctx context.Context, colorScheme string) (*Session, error) {
	tabCtx, closeTab := newTab(ctx)
	s := &Session{ctx: tabCtx, close: closeTab}

	_, err := chromedp.Call(tabCtx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{
		Features: []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: colorScheme}},
	})
	if err != nil {
		s.Close()

		return nil, fmt.Errorf("prefer the %s colour scheme: %w", colorScheme, err)
	}

	_, err = chromedp.Call(tabCtx, emulation.SetScriptExecutionDisabled, emulation.SetScriptExecutionDisabledParams{
		Value: true,
	})
	if err != nil {
		s.Close()

		return nil, fmt.Errorf("disable scripts: %w", err)
	}

	return s, nil
}

// Close closes the tab and the browser.
func (s *Session) Close() {
	s.close()
}

// Open loads url in the tab.
func (s *Session) Open(url string) error {
	if err := chromedp.Do(s.ctx, chromedp.Navigate(url)); err != nil {
		return fmt.Errorf("open %s: %w", url, err)
	}

	return nil
}

// Press clicks the button whose text is label and waits for the page it leads to. It
// returns false, and does nothing, when the page has no such button.
func (s *Session) Press(label string) (bool, error) {
	if strings.Contains(label, "'") {
		return false, fmt.Errorf("press %q: a label with an apostrophe is not supported", label)
	}

	buttons, err := chromedp.Run(
		s.ctx,
		chromedp.Nodes(chromedp.Search("//button[normalize-space()='"+label+"']"), chromedp.AtLeast(0)),
	)
	if err != nil {
		return false, fmt.Errorf("find the %s button: %w", label, err)
	}

	if len(buttons) == 0 {
		return false, nil
	}

	resp, err := chromedp.RunResponse(s.ctx, chromedp.Click(chromedp.NodeIDs{buttons[0].NodeID}))
	if err != nil {
		return false, fmt.Errorf("press the %s button: %w", label, err)
	}

	if resp.Status >= nethttp.StatusBadRequest {
		return false, fmt.Errorf("press the %s button: status %d", label, resp.Status)
	}

	return true, nil
}

// Background returns the computed background colour of the current page's body as the
// browser reports it, for example "rgb(15, 23, 42)".
func (s *Session) Background() (string, error) {
	styles, err := chromedp.Run(s.ctx, chromedp.ComputedStyle(chromedp.CSS("body")))
	if err != nil {
		return "", fmt.Errorf("read the body style: %w", err)
	}

	for _, style := range styles {
		if style.Name == "background-color" {
			return style.Value, nil
		}
	}

	return "", errors.New("the body has no computed background-color")
}
