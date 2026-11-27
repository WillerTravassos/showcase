package ingress

import (
	"net/http"
	"strings"
	"time"
)

// preferenceMaxAge keeps a preference cookie for a year after the patient's last change.
const preferenceMaxAge = 365 * 24 * time.Hour

// savePreferences stores the language the patient picked in a cookie and sends them back
// to the page they came from with 303 See Other. A language outside the allowlist is
// ignored. Cross-origin posts never reach it ([http.CrossOriginProtection]).
func (h *Handler) savePreferences(w http.ResponseWriter, r *http.Request) {
	if lang := r.PostFormValue("lang"); otherLanguage(lang) != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     languageCookie,
			Value:    lang,
			Path:     "/",
			MaxAge:   int(preferenceMaxAge.Seconds()),
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	//nolint:gosec // localPath keeps the redirect on this site.
	http.Redirect(w, r, localPath(r.PostFormValue("return")), http.StatusSeeOther)
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
