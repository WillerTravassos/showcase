package acceptance_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// projectRoot is the clinic-scheduler project directory, relative to this package's
// directory.
const projectRoot = "../.."

// ansiEscape matches the terminal colour codes go-arch-lint writes even when its output
// is not a terminal.
var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func TestDomainPackageMayNotImportAnotherDomainPackage(t *testing.T) {
	t.Parallel()
	project := t.TempDir()
	for _, name := range []string{"go.mod", "go.sum", ".go-arch-lint.yml"} {
		content, err := os.ReadFile(filepath.Join(projectRoot, name))
		require.NoError(t, err)
		writeFile(t, filepath.Join(project, name), string(content))
	}
	writeFile(t, filepath.Join(project, "services/fixture/internal/patient/patient.go"), `package patient

func Name() string { return "" }
`)
	writeFile(t, filepath.Join(project, "services/fixture/internal/appointment/appointment.go"), `package appointment

import "github.com/WillerTravassos/showcase/clinic-scheduler/services/fixture/internal/patient"

var _ = patient.Name
`)

	out, err := exec.CommandContext(t.Context(), "make", "-C", repoRoot, "go-arch", "PROJECT_DIR="+project).
		CombinedOutput()

	report := ansiEscape.ReplaceAllString(string(out), "")
	require.Error(t, err, "architecture check passed:\n%s", report)
	assert.Contains(t, report, "services/fixture/internal/appointment/appointment.go")
	assert.Contains(t, report, "clinic-scheduler/services/fixture/internal/patient")
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}
