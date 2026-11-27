// Package http drives the deployed portals over HTTP, as a browser without JavaScript
// would (ADR 0017).
package http

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	nethttp "net/http"
	"net/http/cookiejar"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// requestTimeout bounds every request, so a gateway that accepts and never answers fails
// the test instead of hanging it.
const requestTimeout = 10 * time.Second

// Page is a portal response: its address, its status, its redirect target, and its
// parsed HTML.
type Page struct {
	URL    *url.URL
	Status int
	// Location is the Location header, empty unless the response is a redirect.
	Location string
	Document *goquery.Document
}

// Client sends requests to the portals through the gateway on 127.0.0.1. It is safe for
// concurrent use.
type Client struct {
	http *nethttp.Client
}

// New returns a Client that trusts only the local certificate authority, read from the
// trust bundle that trust-manager publishes in the cluster. Every request connects to
// 127.0.0.1 whatever its host name, as browsers do for *.localhost (ADR 0011), cookies
// are kept for the Client's lifetime as one browser session, and redirects are returned
// instead of followed.
func New(ctx context.Context) (*Client, error) {
	bundle, err := exec.CommandContext(
		ctx,
		"kubectl",
		"--context",
		"kind-local",
		"get",
		"configmap",
		"local-ca-bundle",
		"--namespace",
		"clinic-scheduler",
		"--output",
		`jsonpath={.data.ca\.crt}`,
	).Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, fmt.Errorf("read the local trust bundle: %w: %s", err, exit.Stderr)
		}

		return nil, fmt.Errorf("read the local trust bundle: %w", err)
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(bundle) {
		return nil, errors.New("read the local trust bundle: no certificate in it")
	}

	var dialer net.Dialer
	transport := &nethttp.Transport{
		TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			_, port, splitErr := net.SplitHostPort(addr)
			if splitErr != nil {
				return nil, fmt.Errorf("split %s: %w", addr, splitErr)
			}

			return dialer.DialContext(ctx, network, net.JoinHostPort("127.0.0.1", port))
		},
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create the cookie jar: %w", err)
	}

	return &Client{http: &nethttp.Client{
		Transport: transport,
		Jar:       jar,
		Timeout:   requestTimeout,
		CheckRedirect: func(*nethttp.Request, []*nethttp.Request) error {
			return nethttp.ErrUseLastResponse
		},
	}}, nil
}

// Get fetches url and returns the response as a [Page].
func (c *Client) Get(ctx context.Context, url string) (*Page, error) {
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", url, err)
	}

	return c.do(req)
}

// Submit sends the form that holds button, as a browser does when the patient presses
// it: the form's inputs and the button's name and value, URL-encoded, to the form's
// action resolved against page, with the page's origin in the Origin header.
func (c *Client) Submit(ctx context.Context, page *Page, button *goquery.Selection) (*Page, error) {
	form := button.Closest("form")
	if form.Length() == 0 {
		return nil, errors.New("submit: the button is not inside a form")
	}

	action, err := page.URL.Parse(form.AttrOr("action", ""))
	if err != nil {
		return nil, fmt.Errorf("submit: parse the form action: %w", err)
	}

	values := url.Values{}

	form.Find("input[name]").Each(func(_ int, input *goquery.Selection) {
		values.Add(input.AttrOr("name", ""), input.AttrOr("value", ""))
	})

	if name := button.AttrOr("name", ""); name != "" {
		values.Add(name, button.AttrOr("value", ""))
	}

	method := strings.ToUpper(form.AttrOr("method", nethttp.MethodGet))
	if method != nethttp.MethodPost {
		return nil, fmt.Errorf("submit: form method %s, only POST is supported", method)
	}

	req, err := nethttp.NewRequestWithContext(ctx, method, action.String(), strings.NewReader(values.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", action, err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", page.URL.Scheme+"://"+page.URL.Host)

	return c.do(req)
}

// do sends req and returns the response as a [Page].
func (c *Client) do(req *nethttp.Request) (*Page, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", req.Method, req.URL, err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", req.URL, err)
	}

	return &Page{
		URL:      resp.Request.URL,
		Status:   resp.StatusCode,
		Location: resp.Header.Get("Location"),
		Document: doc,
	}, nil
}
