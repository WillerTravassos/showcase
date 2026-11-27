// Package dsl holds the domain-language steps that acceptance tests read as scenarios
// (docs/workflow/atdd.md). Each step hides which driver it uses.
package dsl

import (
	"context"
	"fmt"
	nethttp "net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/driver/browser"
	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/driver/http"
)

const (
	// patientPortalURL is the public address of the patient portal.
	patientPortalURL = "https://patient.localhost/"
	// plainHTTPPatientPortalURL is the patient portal without TLS.
	plainHTTPPatientPortalURL = "http://patient.localhost/"
)

// HomePage is what a patient sees on the patient portal home page.
type HomePage struct {
	Heading string
	// Version is the application version the page shows, or "" when it shows none.
	Version string
}

// shownVersion finds the application version in a page footer such as "Version 1a46e7b".
var shownVersion = regexp.MustCompile(`Version\s+(\S+)`)

// OpenPatientPortalHome opens the patient portal home page over HTTPS, trusting only the
// local certificate authority. It fails when the page does not load or its certificate
// is not trusted.
func OpenPatientPortalHome(ctx context.Context) (*HomePage, error) {
	client, err := http.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("open the patient portal: %w", err)
	}

	page, err := client.Get(ctx, patientPortalURL)
	if err != nil {
		return nil, fmt.Errorf("open the patient portal: %w", err)
	}

	if page.Status != nethttp.StatusOK {
		return nil, fmt.Errorf("open the patient portal: status %d", page.Status)
	}

	var version string
	if match := shownVersion.FindStringSubmatch(page.Document.Find("footer").Text()); match != nil {
		version = match[1]
	}

	return &HomePage{Heading: strings.TrimSpace(page.Document.Find("h1").First().Text()), Version: version}, nil
}

// OpenPatientPortalOverPlainHTTP opens the patient portal with http:// and returns the
// address it redirects to. It fails when the response is not a redirect.
func OpenPatientPortalOverPlainHTTP(ctx context.Context) (string, error) {
	client, err := http.New(ctx)
	if err != nil {
		return "", fmt.Errorf("open the patient portal over plain HTTP: %w", err)
	}

	page, err := client.Get(ctx, plainHTTPPatientPortalURL)
	if err != nil {
		return "", fmt.Errorf("open the patient portal over plain HTTP: %w", err)
	}

	if page.Status < nethttp.StatusMultipleChoices || page.Status >= nethttp.StatusBadRequest {
		return "", fmt.Errorf("open the patient portal over plain HTTP: status %d, not a redirect", page.Status)
	}

	return withoutDefaultPort(page.Location)
}

// withoutDefaultPort drops the scheme's default port from address, so
// "https://patient.localhost:443/" and "https://patient.localhost/" read the same.
func withoutDefaultPort(address string) (string, error) {
	u, err := url.Parse(address)
	if err != nil {
		return "", fmt.Errorf("parse redirect address %q: %w", address, err)
	}

	if (u.Scheme == "https" && u.Port() == "443") || (u.Scheme == "http" && u.Port() == "80") {
		u.Host = u.Hostname()
	}

	return u.String(), nil
}

// FramedPortal is what a patient sees when a page on another site frames the patient
// portal.
type FramedPortal struct {
	// Refused is true when the browser received the portal and refused to show it in
	// the frame.
	Refused bool
}

// OpenAnotherSiteFramingPatientPortal opens, in a browser, a page on another site that
// frames the patient portal. It fails when the portal does not answer at all, so a
// portal that is down never counts as refused.
func OpenAnotherSiteFramingPatientPortal(ctx context.Context) (*FramedPortal, error) {
	loadErr, err := browser.FrameLoadError(ctx, patientPortalURL)
	if err != nil {
		return nil, fmt.Errorf("open a page framing the patient portal: %w", err)
	}

	switch loadErr {
	case "":
		return &FramedPortal{Refused: false}, nil
	case "net::ERR_BLOCKED_BY_RESPONSE":
		return &FramedPortal{Refused: true}, nil
	default:
		return nil, fmt.Errorf("open a page framing the patient portal: frame load failed: %s", loadErr)
	}
}
