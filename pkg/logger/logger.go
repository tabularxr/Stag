package logger

import (
	"github.com/sirupsen/logrus"
)

// Logger interface defines logging methods
type Logger interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	WithField(key string, value interface{}) Logger
	WithFields(fields map[string]interface{}) Logger
	SetLevel(level logrus.Level)
}

// LogrusLogger wraps logrus.Logger to implement our Logger interface
type LogrusLogger struct {
	*logrus.Logger
}

// New creates a new logger instance
func New() Logger {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: "2006-01-02T15:04:05.000Z07:00",
	})
	return &LogrusLogger{Logger: log}
}

// WithField creates a new logger with a single field
func (l *LogrusLogger) WithField(key string, value interface{}) Logger {
	return &LogrusLogger{Logger: l.Logger.WithField(key, value).Logger}
}

// WithFields creates a new logger with multiple fields
func (l *LogrusLogger) WithFields(fields map[string]interface{}) Logger {
	logrusFields := logrus.Fields{}
	for k, v := range fields {
		logrusFields[k] = v
	}
	return &LogrusLogger{Logger: l.Logger.WithFields(logrusFields).Logger}
}