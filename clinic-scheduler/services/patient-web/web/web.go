// Package web holds the patient portal's embedded static files and its message catalogue
// (ADR 0002, ADR 0018). Pages and components live in its pages and components packages.
package web

import (
	"embed"
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:generate templ generate
//go:generate tailwindcss --input css/tailwind.css --output static/app.css --minify

//go:embed locales/*.toml
var locales embed.FS

// Static holds the files served under /static/, at their path in this package
// ("static/app.css").
//
//go:embed static
var Static embed.FS

// Catalogue loads the English and French messages. English is the default language.
func Catalogue() (*i18n.Bundle, error) {
	bundle := i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	for _, file := range []string{"locales/active.en.toml", "locales/active.fr.toml"} {
		if _, err := bundle.LoadMessageFileFS(locales, file); err != nil {
			return nil, fmt.Errorf("load messages %s: %w", file, err)
		}
	}

	return bundle, nil
}
