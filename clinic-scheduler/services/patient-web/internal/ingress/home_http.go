package ingress

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/WillerTravassos/showcase/clinic-scheduler/services/patient-web/web/pages"
)

// home renders the patient portal home page in the patient's language.
func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	view, err := h.homeView(h.localizer(r), r.URL.Path)
	if err == nil {
		err = pages.Home(view).Render(ctx, w)
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
func (h *Handler) homeView(localizer *i18n.Localizer, path string) (pages.HomeView, error) {
	name, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: "PortalName"})
	if err != nil {
		return pages.HomeView{}, fmt.Errorf("localize PortalName: %w", err)
	}

	layout, err := h.layoutView(localizer, path, name)
	if err != nil {
		return pages.HomeView{}, err
	}

	return pages.HomeView{Layout: layout, PortalName: name}, nil
}
