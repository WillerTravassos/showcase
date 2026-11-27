package ingress

import (
	"fmt"

	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/WillerTravassos/showcase/clinic-scheduler/services/patient-web/web/components"
)

// layoutView localizes the layout of the page at path, titled title.
func (h *Handler) layoutView(localizer *i18n.Localizer, path, title string) (components.LayoutView, error) {
	label, tag, err := localizer.LocalizeWithTag(&i18n.LocalizeConfig{MessageID: "SwitchLanguage"})
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

	lang := tag.String()

	return components.LayoutView{
		Lang:           lang,
		Title:          title,
		LanguageSwitch: components.LanguageSwitchView{Lang: otherLanguage(lang), Label: label, Return: path},
		Version:        version,
	}, nil
}
