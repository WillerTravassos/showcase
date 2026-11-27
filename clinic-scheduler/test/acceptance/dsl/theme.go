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

// PatientOnDevice is one patient's browser session on the patient portal, on a device
// with a colour scheme preference. It keeps their preferences from one page to the next.
type PatientOnDevice struct {
	session *browser.Session
}

// NewPatientOnDevice starts a browser session, with no preference cookies, on a device
// that prefers the theme. Scripts are disabled, so the theme each page shows is the theme
// it showed on first paint.
func NewPatientOnDevice(ctx context.Context, prefers Theme) (*PatientOnDevice, error) {
	session, err := browser.NewSession(ctx, string(prefers))
	if err != nil {
		return nil, fmt.Errorf("start a patient session on a %s device: %w", prefers, err)
	}

	return &PatientOnDevice{session: session}, nil
}

// Leave closes the patient's browser.
func (p *PatientOnDevice) Leave() {
	p.session.Close()
}

// OpenHome opens the patient portal home page and returns how it looks.
func (p *PatientOnDevice) OpenHome() (*FirstPaint, error) {
	if err := p.session.Open(patientPortalURL); err != nil {
		return nil, fmt.Errorf("open the patient portal home page: %w", err)
	}

	return p.shown()
}

// PickTheme presses the switch to the theme on the page the patient is on, and returns
// how the page the portal shows next looks. It fails when the page has no such switch.
func (p *PatientOnDevice) PickTheme(theme Theme) (*FirstPaint, error) {
	label := themeSwitchLabel(theme)

	pressed, err := p.session.Press(label)
	if err != nil {
		return nil, fmt.Errorf("pick the %s theme: %w", theme, err)
	}

	if !pressed {
		return nil, fmt.Errorf("pick the %s theme: the page offers no %q switch", theme, label)
	}

	return p.shown()
}

// shown reads the theme of the page the patient is on.
func (p *PatientOnDevice) shown() (*FirstPaint, error) {
	background, err := p.session.Background()
	if err != nil {
		return nil, fmt.Errorf("read the page theme: %w", err)
	}

	return &FirstPaint{Theme: themeOf(background), Background: background}, nil
}

// themeSwitchLabel is the label of the switch to theme on an English page.
func themeSwitchLabel(theme Theme) string {
	switch theme {
	case LightTheme:
		return "Light"
	case DarkTheme:
		return "Dark"
	default:
		return string(theme)
	}
}
