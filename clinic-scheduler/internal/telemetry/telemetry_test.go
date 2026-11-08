package telemetry_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"

	"github.com/WillerTravassos/showcase/clinic-scheduler/internal/telemetry"
)

// newCollector starts a fake OTLP/HTTP collector that hands every trace export to the
// returned channel and accepts metric and log exports without reading them.
func newCollector(t *testing.T) (string, <-chan *coltracepb.ExportTraceServiceRequest) {
	t.Helper()
	exports := make(chan *coltracepb.ExportTraceServiceRequest, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/traces" {
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		export := &coltracepb.ExportTraceServiceRequest{}
		if err = proto.Unmarshal(body, export); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		select {
		case exports <- export:
		default: // only the first export is asserted
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL, exports
}

// Not parallel: it sets environment variables and the global tracer provider.
func TestStartExportsSpansWithTheServiceResource(t *testing.T) {
	endpoint, exports := newCollector(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", endpoint)
	t.Setenv("OTEL_SERVICE_NAME", "booking")
	t.Setenv("OTEL_RESOURCE_ATTRIBUTES", "deployment.environment.name=test")

	tel, err := telemetry.Start(t.Context(), "1.2.3")
	require.NoError(t, err)
	_, span := otel.Tracer("test").Start(t.Context(), "appointment.book")
	span.End()
	require.NoError(t, tel.Shutdown(t.Context()))

	var export *coltracepb.ExportTraceServiceRequest
	select {
	case export = <-exports:
	default:
		require.FailNow(t, "no span export reached the collector before Shutdown returned")
	}
	require.Len(t, export.GetResourceSpans(), 1)
	resourceSpans := export.GetResourceSpans()[0]
	attrs := map[string]string{}
	for _, kv := range resourceSpans.GetResource().GetAttributes() {
		attrs[kv.GetKey()] = kv.GetValue().GetStringValue()
	}
	assert.Equal(t, "booking", attrs["service.name"])
	assert.Equal(t, "1.2.3", attrs["service.version"])
	assert.Equal(t, "test", attrs["deployment.environment.name"])
	require.Len(t, resourceSpans.GetScopeSpans(), 1)
	require.Len(t, resourceSpans.GetScopeSpans()[0].GetSpans(), 1)
	assert.Equal(t, "appointment.book", resourceSpans.GetScopeSpans()[0].GetSpans()[0].GetName())
}
