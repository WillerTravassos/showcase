package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"time"

	"github.com/rs/zerolog"
)

var contextKeys = []fmt.Stringer{}

// A zerolog wrapper that implements the contract defined by the Logger interface.
type ZerologWrapper struct {
	logger          *zerolog.Logger
	enableLogOrigin bool
}

func newZerologLogger(settings Settings) ZerologWrapper {
	zerolog.TimestampFieldName = settings.TimestampFieldName
	zerolog.TimeFieldFormat = settings.TimestampFormat
	//nolint:gosec // MinLogLevel does not exceed int8 values and as such, it cannot overflow
	zerolog.SetGlobalLevel(zerolog.Level(settings.MinLogLevel))

	var logWriter io.Writer = os.Stdout

	if settings.LogWriter != nil {
		logWriter = settings.LogWriter
	}

	loggerContext := zerolog.New(logWriter).
		With().
		Timestamp()

	for k, v := range settings.DefaultLogFields {
		loggerContext = setLoggerField(loggerContext, k, v)
	}

	logger := loggerContext.Logger()

	return ZerologWrapper{
		logger:          &logger,
		enableLogOrigin: settings.LogOrigin,
	}
}

//nolint:gocyclo,cyclop,funlen // Function handles multiple types, makes it hard to read if switch is broken up.
func setLoggerField(ctx zerolog.Context, k string, v any) zerolog.Context {
	switch v := v.(type) {
	case string:
		return ctx.Str(k, v)
	case []string:
		return ctx.Strs(k, v)
	case int:
		return ctx.Int(k, v)
	case []int:
		return ctx.Ints(k, v)
	case int8:
		return ctx.Int8(k, v)
	case []int8:
		return ctx.Ints8(k, v)
	case int16:
		return ctx.Int16(k, v)
	case []int16:
		return ctx.Ints16(k, v)
	case int32:
		return ctx.Int32(k, v)
	case []int32:
		return ctx.Ints32(k, v)
	case int64:
		return ctx.Int64(k, v)
	case []int64:
		return ctx.Ints64(k, v)
	case uint:
		return ctx.Uint(k, v)
	case []uint:
		return ctx.Uints(k, v)
	case uint8:
		return ctx.Uint8(k, v)
	case []uint8:
		return ctx.Uints8(k, v)
	case uint16:
		return ctx.Uint16(k, v)
	case []uint16:
		return ctx.Uints16(k, v)
	case uint32:
		return ctx.Uint32(k, v)
	case []uint32:
		return ctx.Uints32(k, v)
	case uint64:
		return ctx.Uint64(k, v)
	case []uint64:
		return ctx.Uints64(k, v)
	case bool:
		return ctx.Bool(k, v)
	case float32:
		return ctx.Float32(k, v)
	case []float32:
		return ctx.Floats32(k, v)
	case float64:
		return ctx.Float64(k, v)
	case []float64:
		return ctx.Floats64(k, v)
	case time.Time:
		return ctx.Time(k, v)
	case time.Duration:
		return ctx.Dur(k, v)
	case fmt.Stringer:
		return ctx.Stringer(k, v)
	case error:
		return ctx.Err(v)
	default:
		return ctx.Any(k, v)
	}
}

func (lw ZerologWrapper) Debug(msg string) {
	lw.logger.Debug().Msg(msg)
}

func (lw ZerologWrapper) Debugf(format string, args ...any) {
	lw.logger.Debug().Msgf(format, args...)
}

func (lw ZerologWrapper) Error(msg string) {
	lw.withLogOrigin(lw.logger.Error()).Msg(msg)
}

func (lw ZerologWrapper) Errorf(format string, args ...any) {
	lw.withLogOrigin(lw.logger.Error()).Msgf(format, args...)
}

func (lw ZerologWrapper) Fatal(msg string) {
	lw.withLogOrigin(lw.logger.Fatal()).Msg(msg)
}

func (lw ZerologWrapper) Fatalf(format string, args ...any) {
	lw.withLogOrigin(lw.logger.Fatal()).Msgf(format, args...)
}

func (lw ZerologWrapper) Info(msg string) {
	lw.logger.Info().Msg(msg)
}

func (lw ZerologWrapper) Infof(format string, args ...any) {
	lw.logger.Info().Msgf(format, args...)
}

func (lw ZerologWrapper) Panic(msg string) {
	lw.withLogOrigin(lw.logger.Panic()).Msg(msg)
}

func (lw ZerologWrapper) Panicf(format string, args ...any) {
	lw.withLogOrigin(lw.logger.Panic()).Msgf(format, args...)
}

func (lw ZerologWrapper) Warn(msg string) {
	lw.withLogOrigin(lw.logger.Warn()).Msg(msg)
}

func (lw ZerologWrapper) Warnf(format string, args ...any) {
	lw.withLogOrigin(lw.logger.Warn()).Msgf(format, args...)
}

func (lw ZerologWrapper) GetContextKeys() []fmt.Stringer {
	return contextKeys
}

func (lw ZerologWrapper) TrackContextKey(key fmt.Stringer) {
	if slices.Contains(contextKeys, key) {
		return
	}

	contextKeys = append(contextKeys, key)
}

func (lw ZerologWrapper) WithContext(ctx context.Context) Logger {
	if ctx == nil || len(contextKeys) == 0 {
		return lw
	}

	contextLogger := lw.logger.With().Logger()

	contextLogger.UpdateContext(func(c zerolog.Context) zerolog.Context {
		for _, key := range contextKeys {
			if value := ctx.Value(key); value != nil {
				c = c.Any(key.String(), value)
			}
		}
		return c
	})

	lw.logger = &contextLogger

	return lw
}

func (lw ZerologWrapper) WithError(err error) Logger {
	errorLogger := lw.logger.With().Err(err).Logger()
	lw.logger = &errorLogger

	return lw
}

func (lw ZerologWrapper) WithField(key string, value any) Logger {
	logEvent := lw.logger.With()

	switch loggedValue := value.(type) {
	case bool:
		logEvent = logEvent.Bool(key, loggedValue)
	case int:
		logEvent = logEvent.Int(key, loggedValue)
	case int8:
		logEvent = logEvent.Int8(key, loggedValue)
	case int16:
		logEvent = logEvent.Int16(key, loggedValue)
	case int32:
		logEvent = logEvent.Int32(key, loggedValue)
	case int64:
		logEvent = logEvent.Int64(key, loggedValue)
	case float32:
		logEvent = logEvent.Float32(key, loggedValue)
	case float64:
		logEvent = logEvent.Float64(key, loggedValue)
	case string:
		logEvent = logEvent.Str(key, loggedValue)
	case []string:
		logEvent = logEvent.Strs(key, loggedValue)
	case uint:
		logEvent = logEvent.Uint(key, loggedValue)
	case uint8:
		logEvent = logEvent.Uint8(key, loggedValue)
	case uint16:
		logEvent = logEvent.Uint16(key, loggedValue)
	case uint32:
		logEvent = logEvent.Uint32(key, loggedValue)
	case uint64:
		logEvent = logEvent.Uint64(key, loggedValue)
	}

	fieldLogger := logEvent.Logger()
	lw.logger = &fieldLogger

	return lw
}

func (lw ZerologWrapper) withLogOrigin(event *zerolog.Event) *zerolog.Event {
	if lw.enableLogOrigin {
		return event.Caller(zerolog.CallerSkipFrameCount)
	}

	return event
}
