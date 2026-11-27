package acceptance_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/dsl"
)

// Scenario: Cluster only: the internet cannot read the portal's operational status.
func TestTheInternetCannotReadThePatientPortalOperationalStatus(t *testing.T) {
	t.Parallel()

	answers, err := dsl.AskPatientPortalWhetherItIsAliveOrReady(t.Context())

	require.NoError(t, err, "the patient portal should answer through the gateway")
	assert.Equal(t, http.StatusNotFound, answers.Alive, "asking whether the portal is alive should find no such page")
	assert.Equal(t, http.StatusNotFound, answers.Ready, "asking whether the portal is ready should find no such page")
}
