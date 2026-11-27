// Command patient-web serves the patient portal.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/WillerTravassos/showcase/clinic-scheduler/services/patient-web/internal/service"
)

// version is the application version, set at build time with
// -ldflags "-X main.version=<version>".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := service.Run(ctx, version)

	stop()

	if err != nil {
		// Errors after telemetry starts are already logged; this covers the ones before.
		fmt.Fprintln(os.Stderr, "patient-web:", err)
		os.Exit(1)
	}
}
