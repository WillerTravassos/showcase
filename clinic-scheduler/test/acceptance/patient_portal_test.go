package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/dsl"
)

// Scenario: HTTPS only: the portal loads securely.
func TestPatientPortalLoadsOverHTTPS(t *testing.T) {
	t.Parallel()

	home, err := dsl.OpenPatientPortalHome(t.Context())

	require.NoError(t, err, "the home page should load with a certificate the local certificate authority trusts")
	assert.Equal(t, "Patient portal", home.Heading)
}

// Scenario: HTTPS only: plain HTTP is redirected.
func TestPatientPortalRedirectsPlainHTTPToHTTPS(t *testing.T) {
	t.Parallel()

	redirectedTo, err := dsl.OpenPatientPortalOverPlainHTTP(t.Context())

	require.NoError(t, err, "the patient portal should answer plain HTTP with a redirect")
	assert.Equal(t, "https://patient.localhost/", redirectedTo)
}

// Scenario: Framing: another site cannot embed the portal.
func TestAnotherSiteCannotFrameThePatientPortal(t *testing.T) {
	t.Parallel()

	framed, err := dsl.OpenAnotherSiteFramingPatientPortal(t.Context())

	require.NoError(t, err, "the patient portal should answer inside the frame")
	assert.True(t, framed.Refused, "the browser should refuse to show the patient portal inside the frame")
}
