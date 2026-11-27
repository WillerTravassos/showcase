package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/dsl"
)

// Scenario: Theme: the device preference is followed on first paint.
//
// Needs a browser: only a browser applies the device's colour scheme preference.
func TestPatientPortalFollowsADarkDevicePreferenceOnFirstPaint(t *testing.T) {
	t.Parallel()

	home, err := dsl.OpenPatientPortalHomeForTheFirstTime(t.Context(), dsl.DarkTheme)

	require.NoError(t, err, "the home page should open in a browser")
	assert.Equal(
		t,
		dsl.DarkTheme,
		home.Theme,
		"the home page should show the dark theme from its first paint; its background is %s",
		home.Background,
	)
}
