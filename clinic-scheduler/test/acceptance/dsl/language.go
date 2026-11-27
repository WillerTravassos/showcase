package dsl

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/driver/http"
)

// Language is a portal language, named by the label its switch shows in that language.
type Language string

// French is the portal's French language.
const French Language = "Français"

// Patient is one patient's session on the patient portal, keeping their preferences
// from one page to the next.
type Patient struct {
	client *http.Client
	page   *http.Page
}

// LocalizedPage is the language a patient portal page is shown in and its heading.
type LocalizedPage struct {
	// Language is the page's BCP 47 language tag, for example "fr".
	Language string
	Heading  string
}

// NewPatient starts a patient session with no preferences.
func NewPatient(ctx context.Context) (*Patient, error) {
	client, err := http.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("start a patient session: %w", err)
	}

	return &Patient{client: client}, nil
}

// OpenHome opens the patient portal home page.
func (p *Patient) OpenHome(ctx context.Context) (*LocalizedPage, error) {
	page, err := p.client.Get(ctx, patientPortalURL)
	if err != nil {
		return nil, fmt.Errorf("open the patient portal home page: %w", err)
	}

	if page.Status != nethttp.StatusOK {
		return nil, fmt.Errorf("open the patient portal home page: status %d", page.Status)
	}

	p.page = page

	return localized(page), nil
}

// SwitchLanguageTo presses the language switch labelled language on the page the patient
// is on, and returns the page the portal shows next. It fails when the page has no such
// switch.
func (p *Patient) SwitchLanguageTo(ctx context.Context, language Language) (*LocalizedPage, error) {
	if p.page == nil {
		return nil, errors.New("switch the language: the patient has no page open")
	}

	button := p.page.Document.Find("form button").FilterFunction(func(_ int, b *goquery.Selection) bool {
		return strings.TrimSpace(b.Text()) == string(language)
	})
	if button.Length() == 0 {
		return nil, fmt.Errorf("switch the language: the page offers no switch to %s", language)
	}

	answer, err := p.client.Submit(ctx, p.page, button.First())
	if err != nil {
		return nil, fmt.Errorf("switch the language to %s: %w", language, err)
	}

	if answer.Status != nethttp.StatusSeeOther {
		return nil, fmt.Errorf("switch the language to %s: status %d, want 303 See Other", language, answer.Status)
	}

	next, err := answer.URL.Parse(answer.Location)
	if err != nil {
		return nil, fmt.Errorf("switch the language to %s: parse the redirect address: %w", language, err)
	}

	page, err := p.client.Get(ctx, next.String())
	if err != nil {
		return nil, fmt.Errorf("switch the language to %s: %w", language, err)
	}

	p.page = page

	return localized(page), nil
}

// localized reads the language and heading of page.
func localized(page *http.Page) *LocalizedPage {
	return &LocalizedPage{
		Language: page.Document.Find("html").AttrOr("lang", ""),
		Heading:  strings.TrimSpace(page.Document.Find("h1").First().Text()),
	}
}
