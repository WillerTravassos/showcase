package ingress

import (
	"log/slog"
	"net/http"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/WillerTravassos/showcase/clinic-scheduler/services/patient-web/web"
)

// home renders the patient portal home page in English.
//
// shortcut: the language is always English; ticket #3 picks it from the patient's choice
// and Accept-Language.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	localizer := i18n.NewLocalizer(h.catalogue, "en")

	name, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: "PortalName"})
	if err == nil {
		err = web.Home(web.HomeView{Lang: "en", PortalName: name}).Render(ctx, w)
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
