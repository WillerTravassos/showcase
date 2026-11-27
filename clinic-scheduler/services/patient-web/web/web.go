// Package web holds the patient portal's templ components, their view models, and the
// message catalogue (ADR 0002, ADR 0018).
package web

import (
	"embed"
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.toml
var locales embed.FS

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
