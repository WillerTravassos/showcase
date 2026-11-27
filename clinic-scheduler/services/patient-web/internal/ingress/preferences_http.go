package ingress

import (
	"net/http"
	"strings"
	"time"
)

// preferenceMaxAge keeps a preference cookie for a year after the patient's last change.
const preferenceMaxAge = 365 * 24 * time.Hour

// savePreferences stores the language or theme the patient picked in a cookie and sends
// them back to the page they came from with 303 See Other. A value outside its allowlist
// is ignored. Cross-origin posts never reach it ([http.CrossOriginProtection]).
func (h *Handler) savePreferences(w http.ResponseWriter, r *http.Request) {
	if lang := r.PostFormValue("lang"); otherLanguage(lang) != "" {
		setPreference(w, languageCookie, lang)
	}

	if theme := r.PostFormValue("theme"); validTheme(theme) {
		setPreference(w, themeCookie, theme)
	}

	//nolint:gosec // localPath keeps the redirect on this site.
	http.Redirect(w, r, localPath(r.PostFormValue("return")), http.StatusSeeOther)
}

// setPreference sets a preference cookie. It carries no personal data, so it only needs
// to be unreadable by scripts and sent over HTTPS with same-site requests.
func setPreference(w http.ResponseWriter, name, value string) {
	http.SetCookie(w, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(preferenceMaxAge.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// localPath returns path when it is a path on this site, and "/" otherwise, so the
// redirect after a form can never send the patient to another site (an open redirect).
// Browsers read "//host" and "/\host" as another host.
func localPath(path string) string {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.HasPrefix(path, `/\`) {
		return "/"
	}

	return path
}
