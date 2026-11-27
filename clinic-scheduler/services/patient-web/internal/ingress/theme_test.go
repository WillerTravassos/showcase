package ingress_test

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHomeShowsThePickedTheme(t *testing.T) {
	t.Parallel()

	body := homeBody(t, map[string]string{"Cookie": "theme=dark"})

	assert.Contains(t, body, `<html lang="en" data-theme="dark">`)
	assert.Contains(t, body, `value="dark" aria-pressed="true">Dark</button>`)
	assert.Contains(t, body, `value="system" aria-pressed="false">Match device</button>`)
}

func TestHomeMarksOnlyThePickedLightThemeAsCurrent(t *testing.T) {
	t.Parallel()

	body := homeBody(t, map[string]string{"Cookie": "theme=light"})

	assert.Contains(t, body, `<html lang="en" data-theme="light">`)
	assert.Contains(t, body, `value="light" aria-pressed="true">Light</button>`)
	assert.Contains(t, body, `value="dark" aria-pressed="false">Dark</button>`)
	assert.Contains(t, body, `value="system" aria-pressed="false">Match device</button>`)
}

func TestHomeFollowsTheDeviceWithoutAPickedTheme(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		headers map[string]string
	}{
		{name: "no cookie", headers: nil},
		{name: "match device", headers: map[string]string{"Cookie": "theme=system"}},
		{name: "outside the allowlist", headers: map[string]string{"Cookie": "theme=sepia"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			body := homeBody(t, tc.headers)

			assert.Contains(t, body, `<html lang="en">`)
			assert.NotContains(t, body, "data-theme")
			assert.Contains(t, body, `value="system" aria-pressed="true">Match device</button>`)
		})
	}
}

func TestHomeLabelsTheThemeSwitchInFrench(t *testing.T) {
	t.Parallel()

	body := homeBody(t, map[string]string{"Cookie": "lang=fr"})

	assert.Contains(t, body, "<legend>Thème</legend>")
	assert.Contains(t, body, `value="dark" aria-pressed="false">Sombre</button>`)
}

func TestSavingAThemeStoresIt(t *testing.T) {
	t.Parallel()

	for _, theme := range []string{"light", "dark", "system"} {
		t.Run(theme, func(t *testing.T) {
			t.Parallel()

			resp := postPreferences(t, url.Values{"theme": {theme}, "return": {"/"}}, nil)
			require.NoError(t, resp.Body.Close())

			assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
			require.Len(t, resp.Cookies(), 1)
			cookie := resp.Cookies()[0]
			assert.Equal(t, "theme", cookie.Name)
			assert.Equal(t, theme, cookie.Value)
			assert.True(t, cookie.Secure, "Secure")
			assert.True(t, cookie.HttpOnly, "HttpOnly")
			assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
		})
	}
}

func TestSavingAThemeOutsideTheAllowlistStoresNothing(t *testing.T) {
	t.Parallel()

	// Values on both sides of the allowed ones, so no ordering of strings passes as a check.
	for _, theme := range []string{"", "DARK", "sepia", "zebra"} {
		t.Run(theme, func(t *testing.T) {
			t.Parallel()

			resp := postPreferences(t, url.Values{"theme": {theme}, "return": {"/"}}, nil)
			require.NoError(t, resp.Body.Close())

			assert.Equal(t, http.StatusSeeOther, resp.StatusCode)
			assert.Empty(t, resp.Cookies())
		})
	}
}
