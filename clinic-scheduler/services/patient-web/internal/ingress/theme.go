package ingress

import "net/http"

// themeCookie holds the theme the patient picked.
const themeCookie = "theme"

// Theme cookie values: the allowlist.
const (
	themeLight  = "light"
	themeDark   = "dark"
	themeSystem = "system"
)

// validTheme reports whether theme is a theme cookie value the portal accepts.
func validTheme(theme string) bool {
	return theme == themeLight || theme == themeDark || theme == themeSystem
}

// pickedTheme is the theme the patient picked, "light" or "dark", or "" when the page
// follows the device: no cookie, "system", or a value outside the allowlist.
func pickedTheme(r *http.Request) string {
	c, err := r.Cookie(themeCookie)
	if err != nil || c.Value == themeSystem || !validTheme(c.Value) {
		return ""
	}

	return c.Value
}
