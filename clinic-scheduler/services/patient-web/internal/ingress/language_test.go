package ingress_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// homeBody fetches the home page with the given request headers and returns its HTML.
func homeBody(t *testing.T, headers map[string]string) string {
	t.Helper()
	handler, _ := newHandler(t)
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

	for name, value := range headers {
		req.Header.Set(name, value)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	body, err := io.ReadAll(rec.Result().Body)
	require.NoError(t, err)

	return string(body)
}

// postPreferences posts form to /preferences with the given request headers.
func postPreferences(t *testing.T, form url.Values, headers map[string]string) *http.Response {
	t.Helper()
	handler, _ := newHandler(t)
	req := httptest.NewRequestWithContext(
		t.Context(),
		http.MethodPost,
		"/preferences",
		strings.NewReader(form.Encode()),
	)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	for name, value := range headers {
		req.Header.Set(name, value)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	return rec.Result()
}

func TestHomeFollowsTheLanguageCookie(t *testing.T) {
	t.Parallel()

	body := homeBody(t, map[string]string{"Cookie": "lang=fr", "Accept-Language": "en"})

	assert.Contains(t, body, `<html lang="fr">`)
	assert.Contains(t, body, "<h1>Portail patient</h1>")
	assert.Contains(t, body, `value="en" lang="en">English</button>`, "a French page offers English")
}

func TestHomeFollowsAcceptLanguageWithoutACookie(t *testing.T) {
	t.Parallel()

	body := homeBody(t, map[string]string{"Accept-Language": "fr-CA,fr;q=0.9"})

	assert.Contains(t, body, `<html lang="fr">`)
}

func TestHomeIgnoresALanguageCookieOutsideTheAllowlist(t *testing.T) {
	t.Parallel()

	body := homeBody(t, map[string]string{"Cookie": "lang=de", "Accept-Language": "de"})

	assert.Contains(t, body, `<html lang="en">`)
	assert.Contains(t, body, `value="fr" lang="fr">Français</button>`, "an English page offers French")
}

func TestSavingALanguageStoresItAndReturnsToThePage(t *testing.T) {
	t.Parallel()

	resp := postPreferences(t, url.Values{"lang": {"fr"}, "return": {"/"}}, nil)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Equal(t, "/", resp.Header.Get("Location"))
	require.Len(t, resp.Cookies(), 1)
	cookie := resp.Cookies()[0]
	assert.Equal(t, "lang", cookie.Name)
	assert.Equal(t, "fr", cookie.Value)
	assert.True(t, cookie.Secure, "Secure")
	assert.True(t, cookie.HttpOnly, "HttpOnly")
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Positive(t, cookie.MaxAge, "the choice persists across visits")
}

func TestSavingALanguageOutsideTheAllowlistStoresNothing(t *testing.T) {
	t.Parallel()

	resp := postPreferences(t, url.Values{"lang": {"de"}, "return": {"/"}}, nil)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
	assert.Empty(t, resp.Cookies())
}

func TestSavingPreferencesNeverRedirectsToAnotherSite(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, returnTo, location string
	}{
		{name: "local path", returnTo: "/appointments?page=2", location: "/appointments?page=2"},
		{name: "missing", returnTo: "", location: "/"},
		{name: "absolute URL", returnTo: "https://evil.example/", location: "/"},
		{name: "scheme-relative", returnTo: "//evil.example/", location: "/"},
		{name: "backslash host", returnTo: `/\evil.example/`, location: "/"},
		{name: "relative path", returnTo: "evil.example", location: "/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			resp := postPreferences(t, url.Values{"lang": {"en"}, "return": {tc.returnTo}}, nil)
			require.NoError(t, resp.Body.Close())

			assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
			assert.Equal(t, tc.location, resp.Header.Get("Location"))
		})
	}
}

func TestAnotherSiteCannotSavePreferences(t *testing.T) {
	t.Parallel()

	resp := postPreferences(
		t,
		url.Values{"lang": {"fr"}, "return": {"/"}},
		map[string]string{"Sec-Fetch-Site": "cross-site"},
	)
	require.NoError(t, resp.Body.Close())

	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	assert.Empty(t, resp.Cookies())
}
