package main

import (
	"fmt"
	"os"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARNING
	ERROR
)

type Logger struct {
	level LogLevel
	file  *os.File
}

func NewLogger(logLevel LogLevel, logFile string) (*Logger, error) {
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}

	return &Logger{level: logLevel, file: file}, nil
}

func (l *Logger) Close() {
	l.file.Close()
}

func (l *Logger) log(level LogLevel, msg string, args ...interface{}) {
	if level >= l.level {
		currentTime := time.Now().Format("2006-01-02 15:04:05")
		prefix := ""
		switch level {
		case DEBUG:
			prefix = "DEBUG"
		case INFO:
			prefix = "INFO"
		case WARNING:
			prefix = "WARNING"
		case ERROR:
			prefix = "ERROR"
		}
		logMsg := fmt.Sprintf("[%s %s] ", currentTime, prefix) + fmt.Sprintf(msg, args...) + "\n"
		l.file.WriteString(logMsg)
	}
}

func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(DEBUG, msg, args...)
}

func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(INFO, msg, args...)
}

func (l *Logger) Warning(msg string, args ...interface{}) {
	l.log(WARNING, msg, args...)
}

func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(ERROR, msg, args...)
}
