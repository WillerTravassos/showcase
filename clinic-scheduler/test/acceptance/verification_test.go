package acceptance_test

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// repoRoot is the showcase repository root, where the Makefile lives, relative to this
// package's directory (the working directory of go test).
const repoRoot = "../../.."

func TestVerificationPassesOnTheScaffold(t *testing.T) {
	t.Parallel()

	out, err := exec.CommandContext(t.Context(), "make", "-C", repoRoot, "go-verify", "PROJECT=clinic-scheduler").
		CombinedOutput()

	require.NoError(t, err, "make go-verify failed:\n%s", out)
}
