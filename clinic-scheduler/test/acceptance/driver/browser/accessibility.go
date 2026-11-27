package browser

import (
	"context"
	_ "embed" // axeSource
	"fmt"

	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/chromedp"
)

// axeSource is axe-core 4.14.1 (axe/axe.min.js, MPL-2.0, from the npm package), the
// accessibility rules engine the browser suite runs on every page (ADR 0017).
//
//go:embed axe/axe.min.js
var axeSource string

// axeRun runs the WCAG 2.0 and 2.1 level A and AA rules on the document and keeps, for each
// violation, the fields a failure message needs.
const axeRun = `axe.run(document, {runOnly: {type: "tag", values: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]}})
	.then(r => r.violations.map(v => ({
		id: v.id,
		impact: v.impact,
		help: v.help,
		targets: v.nodes.map(n => n.target.join(" ")),
	})))`

// Violation is one accessibility rule a page breaks, as axe-core reports it.
type Violation struct {
	// ID names the axe-core rule, for example "color-contrast".
	ID string `json:"id"`
	// Impact is how much the violation hurts: "minor", "moderate", "serious", or
	// "critical".
	Impact string `json:"impact"`
	// Help says what the rule asks for.
	Help string `json:"help"`
	// Targets are the CSS selectors of the elements that break the rule.
	Targets []string `json:"targets"`
}

// AccessibilityViolations opens url in a new browser, with no cookies, on a device that
// prefers the colorScheme ("light" or "dark"), and returns the WCAG 2.1 level A and AA
// violations axe-core finds on the loaded page.
func AccessibilityViolations(ctx context.Context, url, colorScheme string) ([]Violation, error) {
	tabCtx, closeTab := newTab(ctx)
	defer closeTab()

	_, err := chromedp.Call(tabCtx, emulation.SetEmulatedMedia, emulation.SetEmulatedMediaParams{
		Features: []*emulation.MediaFeature{{Name: "prefers-color-scheme", Value: colorScheme}},
	})
	if err != nil {
		return nil, fmt.Errorf("prefer the %s colour scheme: %w", colorScheme, err)
	}

	if err = chromedp.Do(tabCtx, chromedp.Navigate(url)); err != nil {
		return nil, fmt.Errorf("open %s: %w", url, err)
	}

	// DevTools evaluation is not bound by the page's Content-Security-Policy, so axe-core
	// loads although the portal allows no inline or third-party script.
	if _, err = chromedp.Run(tabCtx, chromedp.Evaluate[chromedp.Void](axeSource)); err != nil {
		return nil, fmt.Errorf("load axe-core into %s: %w", url, err)
	}

	violations, err := chromedp.Run(tabCtx, chromedp.Evaluate[[]Violation](axeRun, chromedp.EvalAwaitPromise))
	if err != nil {
		return nil, fmt.Errorf("run axe-core on %s: %w", url, err)
	}

	return violations, nil
}
