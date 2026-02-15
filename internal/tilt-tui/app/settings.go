package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/adrg/xdg"
)

const (
	TiltTuiAppName  = "tilt-tui"
	TiltTuiLogsfile = TiltTuiAppName + ".log"

	defaultShutdownGracePeriod = 2 * time.Second
)

type Settings struct {
	// LogFile tracks logs from tilt-tui
	LogFile string

	ShutdownGracePerion time.Duration
}

func NewSettings() (*Settings, error) {
	appDir, err := createAppSettingsDir()
	if err != nil {
		return nil, fmt.Errorf("failed to create app settings location: %w", err)
	}

	settings := Settings{
		LogFile:             filepath.Join(appDir, TiltTuiLogsfile),
		ShutdownGracePerion: defaultShutdownGracePeriod,
	}

	return &settings, nil
}

func createAppSettingsDir() (string, error) {
	appDir, err := xdg.StateFile(TiltTuiAppName)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(appDir); errors.Is(err, fs.ErrNotExist) {
		if mkdirErr := os.MkdirAll(appDir, DefaultDirMode); mkdirErr != nil {
			return "", err
		}
	}

	return appDir, nil
}
