package ingress

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/WillerTravassos/showcase/clinic-scheduler/services/patient-web/web"
)

// home renders the patient portal home page in the patient's language.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	view, err := homeView(h.localizer(r), r.URL.Path)
	if err == nil {
		err = web.Home(view).Render(ctx, w)
	}

	if err != nil {
		span := trace.SpanFromContext(ctx)
		span.RecordError(err)
		span.SetStatus(codes.Error, "render home page")
		h.logger.ErrorContext(
			ctx,
			"render home page failed",
			slog.String("error.type", "render"),
			slog.String("exception.message", err.Error()),
		)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	}
}

// homeView localizes the home page for the page at path.
func homeView(localizer *i18n.Localizer, path string) (web.HomeView, error) {
	name, tag, err := localizer.LocalizeWithTag(&i18n.LocalizeConfig{MessageID: "PortalName"})
	if err != nil {
		return web.HomeView{}, fmt.Errorf("localize PortalName: %w", err)
	}

	label, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: "SwitchLanguage"})
	if err != nil {
		return web.HomeView{}, fmt.Errorf("localize SwitchLanguage: %w", err)
	}

	lang := tag.String()

	return web.HomeView{
		Lang:           lang,
		PortalName:     name,
		LanguageSwitch: web.LanguageSwitchView{Lang: otherLanguage(lang), Label: label, Return: path},
	}, nil
}
