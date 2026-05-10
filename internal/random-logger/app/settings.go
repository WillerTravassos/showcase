package app

import (
	"time"

	"github.com/WillerTravassos/showcase/pkg/kubernetes"
)

type Settings struct {
	// ShutdownGracePeriod is the configurable grace time value which app go routines to gracefully shutdown.
	ShutdownGracePeriod time.Duration
}

func NewSettings() (*Settings, error) {
	settings := Settings{
		ShutdownGracePeriod: kubernetes.DefaultKubernetesShutdownGracePeriod,
	}

	return &settings, nil
}
