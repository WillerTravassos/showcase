/*
Package logger implements a common interface for logging message in applications.

-----------------------------------------------------------------------------------------------------------------------
All documentation written within the [Go Doc Comments] guidelines.

[Go Doc Comments]: https://go.dev/doc/comment
*/
package log

import (
	"context"
	"fmt"
	"io"
	"os"
)

// For better comprehension, the go time format document can be found here: https://go.dev/src/time/format.go
const DefaultLogEntryTimeFormat = "2006-01-02T15:04:05.000Z07:00"

type Settings struct {
	DefaultLogFields   map[string]any
	LogOrigin          bool
	LogWriter          io.Writer
	MinLogLevel        Level
	TimestampFieldName string
	TimestampFormat    string
}

// log exposes structured logging to any application that installs this package. It exposes a build pattern style
// interface that allows developers to build structured leveled logs.
var log = newDefaultLogger()

func NewLogger(settings Settings) Logger {
	return newZerologLogger(settings)
}

func SetLogger(logger Logger) {
	log = logger
}

func newDefaultLogger() Logger {
	defaultSettings := newDefaultSettings()
	logger := newZerologLogger(defaultSettings)

	return logger
}

func newDefaultSettings() Settings {
	minLogLevel := os.Getenv("LOG_LEVEL")
	timestampFormat := os.Getenv("TIMESTAMP_FORMAT")
	if timestampFormat == "" {
		timestampFormat = DefaultLogEntryTimeFormat
	}

	logOrigin := os.Getenv("LOG_ORIGIN") == "true"

	return Settings{
		DefaultLogFields:   map[string]any{},
		LogOrigin:          logOrigin,
		LogWriter:          os.Stdout,
		MinLogLevel:        LevelFromString(minLogLevel),
		TimestampFieldName: Timestamp,
		TimestampFormat:    timestampFormat,
	}
}

func Debug(msg string) {
	log.Debug(msg)
}

func Debugf(format string, args ...any) {
	log.Debugf(format, args...)
}

func Error(msg string) {
	log.Error(msg)
}

func Errorf(format string, args ...any) {
	log.Errorf(format, args...)
}

// Fatal is used for logging errors in an application. When called, the logger logs the given
// message and immediately exits the application, i.e., call os.Exit(1).
// Never use Fatal in library packages or go routines, since Fatal will not wait for deferred
// go routines to complete execution.
func Fatal(msg string) {
	log.Fatal(msg)
}

func Fatalf(format string, args ...any) {
	log.Fatalf(format, args...)
}

func Info(msg string) {
	log.Info(msg)
}

func Infof(format string, args ...any) {
	log.Infof(format, args...)
}

// Panic is for logging an unrecoverable states and code errors. When called, logger logs the given
// message and panics, i.e., prints a stack trace and terminates.
func Panic(msg string) {
	log.Panic(msg)
}

// Panic is for logging an unrecoverable states and code errors. When called, logger logs the given
// message and panics, i.e., prints a stack trace and terminates.
func Panicf(format string, args ...any) {
	log.Panicf(format, args...)
}

// Warn is for logging messages that indicate that there is the possibility of an error occurring soon.
// This can be used by external systems to track repeated warnings and treat it as an error if passing
// a threshold.
func Warn(msg string) {
	log.Warn(msg)
}

// Warn is for logging messages that indicate that there is the possibility of an error occurring soon.
// This can be used by external systems to track repeated warnings and treat it as an error if passing
// a threshold.
func Warnf(format string, args ...any) {
	log.Warnf(format, args...)
}

// TrackContextKey adds keys that the logger will automatically extract from a context.Context,
// and add it to the output of a structure log.
// This function is not thread safe and should be used as part of the initialization of your application.
func TrackContextKey(key fmt.Stringer) {
	log.TrackContextKey(key)
}

// WithContext extracts metadata from a context.Context.
// It adds values attached to keys, added via [TrackContextKey], to the output of a structured log,
// only and only if the value is not empty.
func WithContext(ctx context.Context) Logger {
	return log.WithContext(ctx)
}

// WithError adds error data to the output of a structured log.
func WithError(err error) Logger {
	return log.WithError(err)
}

// WithField adds fields to the output of a structured log.
func WithField(key string, value any) Logger {
	return log.WithField(key, value)
}

// Returns list of log keys that are set to captured, if present in a handler's context.
func GetContextKeys() []fmt.Stringer {
	return log.GetContextKeys()
}
