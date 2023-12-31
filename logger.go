package migrate

import (
	"io"
	"io/ioutil"
	"log"
	"os"
)

type LoggerInterface interface {
	Debug(v ...interface{})
	Debugf(format string, v ...interface{})
	Info(v ...interface{})
	Infof(format string, v ...interface{})
	Warn(v ...interface{})
	Warnf(format string, v ...interface{})
	Error(v ...interface{})
	Errorf(format string, v ...interface{})
}

var (
	logger LoggerInterface = defaultLogger()
)

// SetLogger sets the XorMigrate logger
func (x *XorMigrate) SetLogger(l LoggerInterface) {
	logger = l
}

func defaultLogger() *XormigrateLogger {
	return &XormigrateLogger{log.New(os.Stdout, "[xormigrate] ", 0)}
}

// DefaultLogger sets a XorMigrate logger with default settings
// eg. "[XorMigrate] message"
func (x *XorMigrate) DefaultLogger() {
	x.SetLogger(defaultLogger())
}

// NilLogger sets a XorMigrate logger that discards all messages
func (x *XorMigrate) NilLogger() {
	x.SetLogger(&XormigrateLogger{log.New(ioutil.Discard, "", 0)})
}

// NewLogger sets a XorMigrate logger with a specified io.Writer
func (x *XorMigrate) NewLogger(writer io.Writer) {
	x.SetLogger(&XormigrateLogger{log.New(writer, "", 0)})
}

type XormigrateLogger struct {
	*log.Logger
}

// Debug prints a Debug message
func (l *XormigrateLogger) Debug(v ...interface{}) {
	l.Logger.Print(v...)
}

// Debugf prints a formatted Debug message
func (l *XormigrateLogger) Debugf(format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}

// Info prints an Info message
func (l *XormigrateLogger) Info(v ...interface{}) {
	l.Logger.Print(v...)
}

// Infof prints a formatted Info message
func (l *XormigrateLogger) Infof(format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}

// Warn prints a Warning message
func (l *XormigrateLogger) Warn(v ...interface{}) {
	l.Logger.Print(v...)
}

// Warnf prints a formatted Warning message
func (l *XormigrateLogger) Warnf(format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}

// Error prints an Error message
func (l *XormigrateLogger) Error(v ...interface{}) {
	l.Logger.Print(v...)
}

// Errorf prints a formatted Error message
func (l *XormigrateLogger) Errorf(format string, v ...interface{}) {
	l.Logger.Printf(format, v...)
}
