package dsl

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"sync"

	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/driver/cluster"
	"github.com/WillerTravassos/showcase/clinic-scheduler/test/acceptance/driver/http"
)

const (
	// patientPortal is the patient portal's deployment and container name.
	patientPortal = "patient-web"
	// patients is how many patients load the home page at the same time.
	patients = 4
	// failuresKept bounds how many failed requests a restart reports in detail.
	failuresKept = 10
)

// PortalRestart is what patients experienced while the patient portal restarted.
type PortalRestart struct {
	// Requests counts the home page loads sent during the restart.
	Requests int
	// FailedRequests counts the home page loads that did not succeed.
	FailedRequests int
	// FirstFailures describes up to ten failed home page loads.
	FirstFailures []string
	// ReplacedPortalExitCodes holds the exit code of each replaced portal.
	ReplacedPortalExitCodes []int
}

// RestartPatientPortalUnderLoad has patients load the patient portal home page
// continuously while the portal is restarted, and keeps them loading until every
// replaced portal is gone.
func RestartPatientPortalUnderLoad(ctx context.Context) (*PortalRestart, error) {
	client, err := http.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("restart the patient portal under load: %w", err)
	}

	pods, err := cluster.PodNames(ctx, patientPortal)
	if err != nil {
		return nil, fmt.Errorf("restart the patient portal under load: %w", err)
	}

	if len(pods) == 0 {
		return nil, errors.New("restart the patient portal under load: no patient portal is running")
	}

	watches := make([]*cluster.TerminationWatch, 0, len(pods))

	for _, pod := range pods {
		watch, watchErr := cluster.WatchTermination(ctx, pod, patientPortal)
		if watchErr != nil {
			return nil, fmt.Errorf("restart the patient portal under load: %w", watchErr)
		}

		watches = append(watches, watch)
	}

	loadCtx, stopLoad := context.WithCancel(ctx)
	defer stopLoad()

	restart := &PortalRestart{}

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	for range patients {
		wg.Go(func() {
			for loadCtx.Err() == nil {
				failure := loadHome(loadCtx, client)

				mu.Lock()
				restart.record(failure)
				mu.Unlock()
			}
		})
	}

	if err = cluster.RestartDeployment(ctx, patientPortal); err != nil {
		return nil, fmt.Errorf("restart the patient portal under load: %w", err)
	}

	for _, watch := range watches {
		termination, waitErr := watch.Wait()
		if waitErr != nil {
			return nil, fmt.Errorf("restart the patient portal under load: %w", waitErr)
		}

		restart.ReplacedPortalExitCodes = append(restart.ReplacedPortalExitCodes, termination.ExitCode)
	}

	stopLoad()
	wg.Wait()

	return restart, nil
}

// loadHome loads the home page once and describes the failure, or returns "" when the
// page loaded. A load cut short because the test stopped the load is not a failure.
func loadHome(ctx context.Context, client *http.Client) string {
	page, err := client.Get(ctx, patientPortalURL)

	switch {
	case ctx.Err() != nil:
		return ""
	case err != nil:
		return err.Error()
	case page.Status != nethttp.StatusOK:
		return fmt.Sprintf("status %d", page.Status)
	default:
		return ""
	}
}

// record counts one home page load and keeps its failure, if any, among the first ones.
func (r *PortalRestart) record(failure string) {
	r.Requests++

	if failure == "" {
		return
	}

	r.FailedRequests++

	if len(r.FirstFailures) < failuresKept {
		r.FirstFailures = append(r.FirstFailures, failure)
	}
}
