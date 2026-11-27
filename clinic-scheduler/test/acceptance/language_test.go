package acceptance_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/dsl"
)

// Scenario: Language: a patient switches to French.
func TestPatientSwitchesThePortalToFrench(t *testing.T) {
	t.Parallel()

	patient, err := dsl.NewPatient(t.Context())
	require.NoError(t, err)

	home, err := patient.OpenHome(t.Context())
	require.NoError(t, err, "the patient should open the home page")
	require.Equal(t, "en", home.Language, "the home page should start in English")

	home, err = patient.SwitchLanguageTo(t.Context(), dsl.French)
	require.NoError(t, err, "the patient should switch the language to French")
	assert.Equal(t, "fr", home.Language, "the home page should be in French")
	assert.Equal(t, "Portail patient", home.Heading, "the home page should be in French")

	next, err := patient.OpenHome(t.Context())
	require.NoError(t, err, "the patient should open the next page")
	assert.Equal(t, "fr", next.Language, "the next page should also be in French")
}
