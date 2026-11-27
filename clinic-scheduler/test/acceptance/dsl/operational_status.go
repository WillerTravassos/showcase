package dsl

import (
	"context"
	"fmt"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/driver/http"
)

// Public addresses where the patient portal's probes would be if the gateway exposed them.
const (
	publicLivenessURL  = "https://patient.localhost/livez"
	publicReadinessURL = "https://patient.localhost/readyz"
)

// OperationalStatusAnswers holds the HTTP status the patient portal answers, through the
// gateway, when asked whether it is alive and whether it is ready.
type OperationalStatusAnswers struct {
	Alive int
	Ready int
}

// AskPatientPortalWhetherItIsAliveOrReady asks the patient portal, from the internet,
// whether it is alive and whether it is ready. It fails when the portal does not answer.
func AskPatientPortalWhetherItIsAliveOrReady(ctx context.Context) (*OperationalStatusAnswers, error) {
	client, err := http.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("ask the patient portal for its operational status: %w", err)
	}

	alive, err := client.Get(ctx, publicLivenessURL)
	if err != nil {
		return nil, fmt.Errorf("ask the patient portal whether it is alive: %w", err)
	}

	ready, err := client.Get(ctx, publicReadinessURL)
	if err != nil {
		return nil, fmt.Errorf("ask the patient portal whether it is ready: %w", err)
	}

	return &OperationalStatusAnswers{Alive: alive.Status, Ready: ready.Status}, nil
}
