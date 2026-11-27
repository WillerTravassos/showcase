package dsl

import (
	"context"
	"fmt"
	"strings"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/driver/browser"
)

// CheckPatientPortalHomeAccessibility opens the home page in a new browser on a device
// that prefers the theme, checks it against WCAG 2.1 level AA, and describes each
// violation found, for example
// "color-contrast (serious): Elements must meet minimum color contrast ratio thresholds: button".
func CheckPatientPortalHomeAccessibility(ctx context.Context, theme Theme) ([]string, error) {
	violations, err := browser.AccessibilityViolations(ctx, patientPortalURL, string(theme))
	if err != nil {
		return nil, fmt.Errorf("check the patient portal home page in the %s theme: %w", theme, err)
	}

	described := make([]string, 0, len(violations))
	for _, v := range violations {
		described = append(
			described,
			fmt.Sprintf("%s (%s): %s: %s", v.ID, v.Impact, v.Help, strings.Join(v.Targets, ", ")),
		)
	}

	return described, nil
}
