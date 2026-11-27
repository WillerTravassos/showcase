package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/dsl"
)

// Scenario: Accessibility: the home page passes in both themes.
//
// Needs a browser: axe-core checks the page as the browser renders it.
func TestPatientPortalHomePassesAccessibilityChecksInBothThemes(t *testing.T) {
	t.Parallel()

	for _, theme := range []dsl.Theme{dsl.LightTheme, dsl.DarkTheme} {
		violations, err := dsl.CheckPatientPortalHomeAccessibility(t.Context(), theme)

		require.NoError(t, err, "the home page should be checked in the %s theme", theme)
		assert.Empty(t, violations, "the home page should have no WCAG 2.1 AA violation in the %s theme", theme)
	}

	home, err := dsl.OpenPatientPortalHome(t.Context())

	require.NoError(t, err, "the patient should open the home page")
	assert.NotEmpty(t, home.Version, "the home page should show the application version")
	assert.NotEqual(t, "dev", home.Version, "the version should be the one the build stamped, not the unbuilt default")
}
