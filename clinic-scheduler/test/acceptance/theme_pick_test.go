package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/dsl"
)

// Scenario: Theme: a patient picks the dark theme.
//
// Needs a browser: only a browser applies the device's colour scheme preference.
func TestPatientPicksTheDarkThemeOnALightDevice(t *testing.T) {
	t.Parallel()

	patient, err := dsl.NewPatientOnDevice(t.Context(), dsl.LightTheme)
	require.NoError(t, err)
	t.Cleanup(patient.Leave)

	home, err := patient.OpenHome()
	require.NoError(t, err, "the patient should open the home page")
	require.Equal(t, dsl.LightTheme, home.Theme, "the home page should start in the device's light theme")

	home, err = patient.PickTheme(dsl.DarkTheme)
	require.NoError(t, err, "the patient should pick the dark theme")
	assert.Equal(
		t,
		dsl.DarkTheme,
		home.Theme,
		"the page should be in the dark theme; its background is %s",
		home.Background,
	)

	next, err := patient.OpenHome()
	require.NoError(t, err, "the patient should open the next page")
	assert.Equal(
		t,
		dsl.DarkTheme,
		next.Theme,
		"the next page should also be in the dark theme; its background is %s",
		next.Background,
	)
}
