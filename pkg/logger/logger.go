package logger

import (
	"errors"
	"fmt"
	"os"
	"path"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

var once sync.Once

// ErrLogLevel is returned when an attempt is made to create a logger with an unrecognized log level
var ErrLogLevel = errors.New("error level not recognized")

// Flags used for selecting the error level
const (
	DEBUG logrus.Level = logrus.DebugLevel
	INFO  logrus.Level = logrus.InfoLevel
	ERROR logrus.Level = logrus.ErrorLevel
)

//Logger extends log.Logger to add a file handle and the current logging level for use by the logging package. does not add any exported fields.
type Logger struct {
	logrus.Logger
	file          *os.File
	defaultFields *logrus.Entry
}

var logger *Logger

func init() {
	New()
}

// New creates a new logger and initializes to STDOUT
func New() *Logger {
	once.Do(func() {
		logger = &Logger{
			Logger: *logrus.New(),
			file:   os.Stdout,
		}
		logger.Level = DEBUG
	})

	return logger
}

func checkAndRotate(filename string) {
	fs, err := os.Stat(filename)
	if err != nil {
		return
	}
	if fs.Size() > 5_000_000 { //file is larger than 5MB
		//Delete the oldest and rename the remaining
		os.Remove(fmt.Sprintf("%s_old4", filename))
		for i := 3; i > 0; i-- {
			os.Rename(fmt.Sprintf("%s_old%d", filename, i), fmt.Sprintf("%s_%d", filename, i+1))
		}
		os.Rename(filename, fmt.Sprintf("%s_old%d", filename, 1))
	}
}

// Init creates a new logger and initializes it to use the provided file and log level
func Init(filename string, level logrus.Level, env string) (*Logger, error) {

	l := New()

	checkAndRotate(filename)

	f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("error opening file for logging: %w", err)
	}

	l.file = f
	l.SetReportCaller(true)
	l.SetOutput(f)

	if env == "dev" {
		l.SetFormatter(&logrus.TextFormatter{
			ForceColors:  true,
			DisableQuote: true,
			CallerPrettyfier: func(f *runtime.Frame) (function string, file string) {
				filename := path.Base(f.File)
				functionParts := strings.Split(f.Function, "/")
				return fmt.Sprintf("%s()", functionParts[len(functionParts)-1]), fmt.Sprintf("%s:%d", filename, f.Line)
			},
		})
		logger.defaultFields = l.WithFields(logrus.Fields{})
	} else {
		l.SetFormatter(&logrus.JSONFormatter{})
		logger.defaultFields = l.WithFields(logrus.Fields{
			"env": env,
		})
	}

	return l, SetLevel(level)
}

//Close releases the file handle opened for use by the logger.
func (l *Logger) Close() error {
	return l.file.Close()
}

//SetLevel allows changing the log level after a logger has been created.
func SetLevel(lev logrus.Level) error {
	if lev == DEBUG || lev == INFO || lev == ERROR {
		logger.Level = lev
		return nil
	}
	return ErrLogLevel
}

func GetLevel() logrus.Level {
	return logger.Level
}

// GetFileHandle returns the file handle in use by the logger
func (l *Logger) GetFileHandle() *os.File {
	return l.file
}

//PrintStack extends log.Printf by appending a stack trace
func PrintStack(level logrus.Level, format string, data ...interface{}) {
	if level == DEBUG && logger.GetLevel() >= level {
		logger.WithField("stack", string(debug.Stack())).Debugf(format, data...)
	}
	if level == INFO && logger.GetLevel() >= level {
		logger.WithField("stack", string(debug.Stack())).Infof(format, data...)
	}
	if level == ERROR && logger.GetLevel() >= level {
		logger.WithField("stack", string(debug.Stack())).Errorf(format, data...)
	}
}

func Get() *logrus.Entry {
	return logger.defaultFields
}

func GetCtx(c *gin.Context) *logrus.Entry {
	ctx := c.Request.Context()
	span, ok := tracer.SpanFromContext(ctx)
	if !ok {
		return logger.defaultFields
	}
	return logger.defaultFields.WithFields(logrus.Fields{
		"dd.trace_id": span.Context().TraceID(),
		"dd.span_id":  span.Context().SpanID(),
	})
}
