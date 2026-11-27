package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/dsl"
)

// Scenario: No failed requests: the portal restarts under load.
//
//nolint:paralleltest // Restarts the shared patient portal; runs before the parallel tests start.
func TestPatientPortalRestartsUnderLoadWithoutFailedRequests(t *testing.T) {
	restart, err := dsl.RestartPatientPortalUnderLoad(t.Context())

	require.NoError(t, err, "the patient portal should restart while patients load it")
	require.Positive(t, restart.Requests, "patients should have loaded the home page during the restart")
	assert.Zero(
		t,
		restart.FailedRequests,
		"every patient request should succeed while the portal restarts; %d of %d failed, first: %v",
		restart.FailedRequests,
		restart.Requests,
		restart.FirstFailures,
	)
	require.NotEmpty(t, restart.ReplacedPortalExitCodes, "a running patient portal should have been replaced")

	for _, code := range restart.ReplacedPortalExitCodes {
		assert.Zero(t, code, "the replaced portal should exit cleanly on its own before its grace period ends")
	}
}
