package dsl

import (
	"context"
	"fmt"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/driver/browser"
)

// Theme is a portal colour theme, or a device's colour scheme preference.
type Theme string

// The portal themes (ADR 0013).
const (
	LightTheme Theme = "light"
	DarkTheme  Theme = "dark"
)

// FirstPaint is how a page looked when the browser first showed it.
type FirstPaint struct {
	// Theme is the theme whose background the page shows, or "" when it matches none.
	Theme Theme
	// Background is the page background as the browser reports it, for example
	// "rgb(15, 23, 42)".
	Background string
}

// OpenPatientPortalHomeForTheFirstTime opens the home page in a new browser, with no
// preference cookies, on a device that prefers the theme. Scripts are disabled, so
// nothing can restyle the page once it loads: the theme it shows is the theme it showed
// on first paint.
func OpenPatientPortalHomeForTheFirstTime(ctx context.Context, prefers Theme) (*FirstPaint, error) {
	background, err := browser.Background(ctx, patientPortalURL, string(prefers))
	if err != nil {
		return nil, fmt.Errorf("open the patient portal for the first time: %w", err)
	}

	return &FirstPaint{Theme: themeOf(background), Background: background}, nil
}

// themeOf names the theme whose bg token (ADR 0013) is background, written as browsers
// report a computed colour.
func themeOf(background string) Theme {
	switch background {
	case "rgb(255, 255, 255)":
		return LightTheme
	case "rgb(15, 23, 42)":
		return DarkTheme
	default:
		return ""
	}
}
