package app

import "os"

const (
	DefaultDirMode  = os.FileMode(0o744)
	DefaultFileMode = os.FileMode(0o600)
)
