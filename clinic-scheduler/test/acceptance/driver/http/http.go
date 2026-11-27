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
	"os/exec"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// requestTimeout bounds every request, so a gateway that accepts and never answers fails
// the test instead of hanging it.
const requestTimeout = 10 * time.Second

// Page is a portal response: its status, its redirect target, and its parsed HTML.
type Page struct {
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
// 127.0.0.1 whatever its host name, as browsers do for *.localhost (ADR 0011), and
// redirects are returned instead of followed.
func New(ctx context.Context) (*Client, error) {
	bundle, err := exec.CommandContext(ctx, "kubectl", "--context", "kind-local",
		"get", "configmap", "local-ca-bundle", "--namespace", "clinic-scheduler",
		"--output", `jsonpath={.data.ca\.crt}`).Output()
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
	return &Client{http: &nethttp.Client{
		Transport: transport,
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
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", url, err)
	}
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", url, err)
	}
	return &Page{Status: resp.StatusCode, Location: resp.Header.Get("Location"), Document: doc}, nil
}
