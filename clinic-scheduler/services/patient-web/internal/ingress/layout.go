package ingress

import (
	"fmt"
	"net/http"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/WillerTravassos/showcase/clinic-scheduler/services/patient-web/web/components"
)

// layoutView localizes the layout of the page r asked for, titled title.
func (h *Handler) layoutView(r *http.Request, localizer *i18n.Localizer, title string) (components.LayoutView, error) {
	switchLabel, tag, err := localizer.LocalizeWithTag(&i18n.LocalizeConfig{MessageID: "SwitchLanguage"})
	if err != nil {
		return components.LayoutView{}, fmt.Errorf("localize SwitchLanguage: %w", err)
	}

	version, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    "Version",
		TemplateData: map[string]string{"Version": h.version},
	})
	if err != nil {
		return components.LayoutView{}, fmt.Errorf("localize Version: %w", err)
	}

	theme := pickedTheme(r)

	themeSwitch, err := themeSwitchView(localizer, theme, r.URL.Path)
	if err != nil {
		return components.LayoutView{}, err
	}

	lang := tag.String()

	return components.LayoutView{
		Lang:  lang,
		Theme: theme,
		Title: title,
		LanguageSwitch: components.LanguageSwitchView{
			Lang:   otherLanguage(lang),
			Label:  switchLabel,
			Return: r.URL.Path,
		},
		ThemeSwitch: themeSwitch,
		Version:     version,
	}, nil
}

// themeSwitchView localizes the theme switch for the page at path, marking theme ("" for
// the device's theme) as the current one.
func themeSwitchView(localizer *i18n.Localizer, theme, path string) (components.ThemeSwitchView, error) {
	view := components.ThemeSwitchView{Return: path}

	legend, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: "ThemeLegend"})
	if err != nil {
		return view, fmt.Errorf("localize ThemeLegend: %w", err)
	}

	view.Legend = legend

	for _, option := range []struct{ value, messageID string }{
		{value: themeLight, messageID: "ThemeLight"},
		{value: themeDark, messageID: "ThemeDark"},
		{value: themeSystem, messageID: "ThemeSystem"},
	} {
		label, localizeErr := localizer.Localize(&i18n.LocalizeConfig{MessageID: option.messageID})
		if localizeErr != nil {
			return view, fmt.Errorf("localize %s: %w", option.messageID, localizeErr)
		}

		current := option.value == theme || (theme == "" && option.value == themeSystem)
		view.Options = append(view.Options, components.ThemeOption{Value: option.value, Label: label, Current: current})
	}

	return view, nil
}
