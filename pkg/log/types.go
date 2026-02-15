package log

import (
	"context"
	"fmt"
	"strings"
)

type Level int

// Log levels used by log package.
const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	PanicLevel
)

// String representation of log levels.
const (
	DebugLevelName = "debug"
	InfoLevelName  = "info"
	WarnLevelName  = "warn"
	ErrorLevelName = "error"
	FatalLevelName = "fatal"
	PanicLevelName = "panic"
)

// LevelFromString returns the log [Level] extracted from a string value.
// If string doesn't match a level, function defaults to Info [Level].
func LevelFromString(level string) Level {
	switch strings.ToLower(level) {
	case DebugLevelName:
		return DebugLevel
	case WarnLevelName:
		return WarnLevel
	case ErrorLevelName:
		return ErrorLevel
	case FatalLevelName:
		return FatalLevel
	case PanicLevelName:
		return PanicLevel
	default:
		return InfoLevel
	}
}

type Logger interface {
	Debug(msg string)
	Debugf(format string, args ...any)
	Error(msg string)
	Errorf(format string, args ...any)
	Fatal(msg string)
	Fatalf(format string, args ...any)
	Info(msg string)
	Infof(format string, args ...any)
	Panic(msg string)
	Panicf(format string, args ...any)
	Warn(msg string)
	Warnf(format string, args ...any)
	GetContextKeys() []fmt.Stringer
	TrackContextKey(key fmt.Stringer)
	WithContext(context.Context) Logger
	WithError(err error) Logger
	WithField(key string, value any) Logger
}
