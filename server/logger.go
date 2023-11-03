package server

// This file implements a logging utility for the GCA server.

import (
	"fmt"
	"os"
	"time"
)

// LogLevel represents the level of logging.
type LogLevel int

// Constants for different log levels.
const (
	DEBUG LogLevel = iota // Debug level (0)
	INFO                  // Information level (1)
	WARN                  // Warning level (2)
	ERROR                 // Error level (3)
	FATAL                 // Fatal error level (4)
)

// Logger holds the configuration for a logger.
type Logger struct {
	level LogLevel // Minimum log level to output
	file  *os.File // File to write logs to
}

// NewLogger initializes a new logger.
// logLevel: Level of log messages to display.
// logFile: File name to which logs will be written.
// Returns a pointer to a Logger or an error if any.
func NewLogger(logLevel LogLevel, logFile string) (*Logger, error) {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return &Logger{level: logLevel, file: file}, nil
}

// Close closes the log file.
func (l *Logger) Close() {
	l.file.Close()
}

// Internal logging function. Handles actual file writes.
func (l *Logger) log(level LogLevel, msg string) {
	if level >= l.level {
		currentTime := time.Now().Format("2006-01-02 15:04:05")
		prefix := ""

		switch level {
		case DEBUG:
			prefix = "DEBUG"
		case INFO:
			prefix = "INFO"
		case WARN:
			prefix = "WARN"
		case ERROR:
			prefix = "ERROR"
		case FATAL:
			prefix = "FATAL"
		}

		logMsg := fmt.Sprintf("[%s %s] %s\n", currentTime, prefix, msg)
		l.file.WriteString(logMsg)

		if level == FATAL {
			panic(logMsg)
		}
	}
}

// Debug logs debug messages using string concatenation.
func (l *Logger) Debug(args ...interface{}) {
	l.log(DEBUG, fmt.Sprint(args...))
}

// Debugf logs debug messages using format directives.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DEBUG, fmt.Sprintf(format, args...))
}

// Info logs informational messages using string concatenation.
func (l *Logger) Info(args ...interface{}) {
	l.log(INFO, fmt.Sprint(args...))
}

// Infof logs informational messages using format directives.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(INFO, fmt.Sprintf(format, args...))
}

// Warn logs warning messages using string concatenation.
func (l *Logger) Warn(args ...interface{}) {
	l.log(WARN, fmt.Sprint(args...))
}

// Warnf logs warning messages using format directives.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WARN, fmt.Sprintf(format, args...))
}

// Error logs error messages using string concatenation.
func (l *Logger) Error(args ...interface{}) {
	l.log(ERROR, fmt.Sprint(args...))
}

// Errorf logs error messages using format directives.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ERROR, fmt.Sprintf(format, args...))
}

// Fatal logs fatal messages using string concatenation and then panics.
func (l *Logger) Fatal(args ...interface{}) {
	l.log(FATAL, fmt.Sprint(args...))
}

// Fatalf logs fatal messages using format directives and then panics.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(FATAL, fmt.Sprintf(format, args...))
}
