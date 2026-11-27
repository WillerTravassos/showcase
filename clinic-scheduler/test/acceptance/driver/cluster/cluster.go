// Package cluster drives the local Kubernetes cluster that Tilt runs, through kubectl.
package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

const (
	// kubeContext is the kind cluster that Tilt deploys to (tilt/cluster.yaml).
	kubeContext = "kind-local"
	// namespace holds every clinic-scheduler workload.
	namespace = "clinic-scheduler"
	// rolloutTimeout bounds how long a restart may take before the test fails.
	rolloutTimeout = "120s"
)

// PodNames returns the names of the running pods of deployment.
func PodNames(ctx context.Context, deployment string) ([]string, error) {
	out, err := kubectl(
		ctx,
		"get",
		"pods",
		"--selector",
		"app.kubernetes.io/name="+deployment,
		"--field-selector",
		"status.phase=Running",
		"--output",
		"jsonpath={.items[*].metadata.name}",
	)
	if err != nil {
		return nil, fmt.Errorf("list pods of %s: %w", deployment, err)
	}

	return strings.Fields(string(out)), nil
}

// RestartDeployment replaces every pod of deployment, as a rollout of a new version
// would, and returns once the rollout is complete.
func RestartDeployment(ctx context.Context, deployment string) error {
	if _, err := kubectl(ctx, "rollout", "restart", "deployment/"+deployment); err != nil {
		return fmt.Errorf("restart %s: %w", deployment, err)
	}

	if _, err := kubectl(ctx, "rollout", "status", "deployment/"+deployment, "--timeout", rolloutTimeout); err != nil {
		return fmt.Errorf("wait for %s to roll out: %w", deployment, err)
	}

	return nil
}

// Termination is how a container ended.
type Termination struct {
	ExitCode int    `json:"exitCode"`
	Reason   string `json:"reason"`
}

// watchEvent is one line of `kubectl get --watch --output-watch-events --output json`.
type watchEvent struct {
	Type   string `json:"type"`
	Object struct {
		Status struct {
			ContainerStatuses []struct {
				Name  string `json:"name"`
				State struct {
					Terminated *Termination `json:"terminated"`
				} `json:"state"`
			} `json:"containerStatuses"`
		} `json:"status"`
	} `json:"object"`
}

// TerminationWatch follows one pod until it is deleted.
type TerminationWatch struct {
	pod       string
	container string
	cmd       *exec.Cmd
	decoder   *json.Decoder
}

// WatchTermination starts watching pod and returns once the watch has seen the pod, so
// nothing that happens to it afterwards is missed.
func WatchTermination(ctx context.Context, pod, container string) (*TerminationWatch, error) {
	cmd := exec.CommandContext(
		ctx,
		"kubectl",
		"--context",
		kubeContext,
		"--namespace",
		namespace,
		"get",
		"pod",
		pod,
		"--watch",
		"--output-watch-events",
		"--output",
		"json",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("watch pod %s: %w", pod, err)
	}

	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("watch pod %s: %w", pod, err)
	}

	watch := &TerminationWatch{pod: pod, container: container, cmd: cmd, decoder: json.NewDecoder(stdout)}

	if _, _, err = watch.next(); err != nil {
		watch.stop()

		return nil, err
	}

	return watch, nil
}

// Wait blocks until the pod is deleted and returns how its container ended.
func (w *TerminationWatch) Wait() (*Termination, error) {
	defer w.stop()

	var last *Termination

	for {
		deleted, terminated, err := w.next()
		if err != nil {
			return nil, err
		}

		if terminated != nil {
			last = terminated
		}

		if deleted {
			if last == nil {
				return nil, fmt.Errorf("watch pod %s: deleted without a termination status", w.pod)
			}

			return last, nil
		}
	}
}

// next reads one watch event and reports whether it deleted the pod and the container's
// termination status, if the event carries one.
func (w *TerminationWatch) next() (bool, *Termination, error) {
	var event watchEvent

	if err := w.decoder.Decode(&event); err != nil {
		if errors.Is(err, io.EOF) {
			return false, nil, fmt.Errorf("watch pod %s: stream ended before the pod was deleted", w.pod)
		}

		return false, nil, fmt.Errorf("watch pod %s: %w", w.pod, err)
	}

	var terminated *Termination

	for _, status := range event.Object.Status.ContainerStatuses {
		if status.Name == w.container && status.State.Terminated != nil {
			terminated = status.State.Terminated
		}
	}

	return event.Type == "DELETED", terminated, nil
}

// stop ends the kubectl watch and reaps its process.
func (w *TerminationWatch) stop() {
	_ = w.cmd.Process.Kill()
	_ = w.cmd.Wait()
}

// kubectl runs a kubectl command against the clinic-scheduler namespace of the local
// cluster and returns its standard output.
func kubectl(ctx context.Context, args ...string) ([]byte, error) {
	var stderr bytes.Buffer

	//nolint:gosec // Arguments come from this driver's callers in test code, never from outside input.
	cmd := exec.CommandContext(
		ctx,
		"kubectl",
		append([]string{"--context", kubeContext, "--namespace", namespace}, args...)...,
	)
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("kubectl %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}

	return out, nil
}
