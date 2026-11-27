package ingress

import (
	"net/http"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// languageCookie holds the language the patient picked.
const languageCookie = "lang"

// otherLanguage returns the language the switch offers on a page in lang, or "" when
// lang is not a portal language. It is the allowlist of language cookie values.
func otherLanguage(lang string) string {
	switch lang {
	case "en":
		return "fr"
	case "fr":
		return "en"
	default:
		return ""
	}
}

// localizer picks the page language: the patient's language cookie, then the browser's
// Accept-Language, then the catalogue's default, English. A cookie outside the allowlist
// is ignored.
func (h *Handler) localizer(r *http.Request) *i18n.Localizer {
	var picked string
	if c, err := r.Cookie(languageCookie); err == nil && otherLanguage(c.Value) != "" {
		picked = c.Value
	}

	return i18n.NewLocalizer(h.catalogue, picked, r.Header.Get("Accept-Language"))
}
