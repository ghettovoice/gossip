package log

import (
	"io"

	"github.com/sirupsen/logrus"
)

const (
	// PanicLevel level, highest level of severity. Logs and then calls panic with the
	// message passed to Debug, Info, ...
	PanicLevel = logrus.PanicLevel
	// FatalLevel level. Logs and then calls `os.Exit(1)`. It will exit even if the
	// logging level is set to Panic.
	FatalLevel = logrus.FatalLevel
	// ErrorLevel level. Logs. Used for errors that should definitely be noted.
	// Commonly used for hooks to send errors to an error tracking service.
	ErrorLevel = logrus.ErrorLevel
	// WarnLevel level. Non-critical entries that deserve eyes.
	WarnLevel = logrus.WarnLevel
	// InfoLevel level. General operational entries about what's going on inside the
	// application.
	InfoLevel = logrus.InfoLevel
	// DebugLevel level. Usually only enabled when debugging. Very verbose logging.
	DebugLevel = logrus.DebugLevel
)

func init() {
	logrus.AddHook(&StackHook{})
	logrus.SetFormatter(NewFormatter(true))
}

type Logger interface {
	logrus.FieldLogger
}

type WithLocalLogger interface {
	// Log returns Logger instance
	Log() Logger
}

func StandardLogger() logrus.FieldLogger {
	return logrus.StandardLogger()
}

func SetOutput(out io.Writer) {
	logrus.SetOutput(out)
}

func SetLevel(level logrus.Level) {
	logrus.SetLevel(level)
}

func GetLevel() logrus.Level {
	return logrus.GetLevel()
}

// WithError creates an entry from the standard logger and adds an error to it, using the value defined in ErrorKey as key.
func WithError(err error) Logger {
	return logrus.WithField(logrus.ErrorKey, err)
}

// WithField creates an entry from the standard logger and adds a field to
// it. If you want multiple fields, use `WithFields`.
//
// Note that it doesn't log until you call Debug, Print, Info, Warn, Fatal
// or Panic on the Entry it returns.
func WithField(key string, value interface{}) Logger {
	return logrus.WithField(key, value)
}

// WithFields creates an entry from the standard logger and adds multiple
// fields to it. This is simply a helper for `WithField`, invoking it
// once for each field.
//
// Note that it doesn't log until you call Debug, Print, Info, Warn, Fatal
// or Panic on the Entry it returns.
func WithFields(fields map[string]interface{}) Logger {
	return logrus.WithFields(fields)
}

// Debug logs a message at level Debug on the standard logger.
func Debug(msg string, args ...interface{}) {
	Debugf(msg, args...)
}

// Print logs a message at level Info on the standard logger.
func Print(msg string, args ...interface{}) {
	Printf(msg, args...)
}

// Info logs a message at level Info on the standard logger.
func Info(msg string, args ...interface{}) {
	Infof(msg, args...)
}

// Warn logs a message at level Warn on the standard logger.
func Warn(msg string, args ...interface{}) {
	Warnf(msg, args...)
}

// Warning logs a message at level Warn on the standard logger.
func Warning(msg string, args ...interface{}) {
	Warning(msg, args...)
}

// Error logs a message at level Error on the standard logger.
func Error(msg string, args ...interface{}) {
	Errorf(msg, args...)
}

// Panic logs a message at level Panic on the standard logger.
func Panic(msg string, args ...interface{}) {
	Panicf(msg, args...)
}

// Fatal logs a message at level Fatal on the standard logger.
func Fatal(msg string, args ...interface{}) {
	Fatalf(msg, args...)
}

// Debugf logs a message at level Debug on the standard logger.
func Debugf(format string, args ...interface{}) {
	logrus.Debugf(format, args...)
}

// Printf logs a message at level Info on the standard logger.
func Printf(format string, args ...interface{}) {
	logrus.Printf(format, args...)
}

// Infof logs a message at level Info on the standard logger.
func Infof(format string, args ...interface{}) {
	logrus.Infof(format, args...)
}

// Warnf logs a message at level Warn on the standard logger.
func Warnf(format string, args ...interface{}) {
	logrus.Warnf(format, args...)
}

// Warningf logs a message at level Warn on the standard logger.
func Warningf(format string, args ...interface{}) {
	logrus.Warningf(format, args...)
}

// Errorf logs a message at level Error on the standard logger.
func Errorf(format string, args ...interface{}) {
	logrus.Errorf(format, args...)
}

// Panicf logs a message at level Panic on the standard logger.
func Panicf(format string, args ...interface{}) {
	logrus.Panicf(format, args...)
}

// Fatalf logs a message at level Fatal on the standard logger.
func Fatalf(format string, args ...interface{}) {
	logrus.Fatalf(format, args...)
}

// Debugln logs a message at level Debug on the standard logger.
func Debugln(args ...interface{}) {
	logrus.Debugln(args...)
}

// Println logs a message at level Info on the standard logger.
func Println(args ...interface{}) {
	logrus.Println(args...)
}

// Infoln logs a message at level Info on the standard logger.
func Infoln(args ...interface{}) {
	logrus.Infoln(args...)
}

// Warnln logs a message at level Warn on the standard logger.
func Warnln(args ...interface{}) {
	logrus.Warnln(args...)
}

// Warningln logs a message at level Warn on the standard logger.
func Warningln(args ...interface{}) {
	logrus.Warningln(args...)
}

// Errorln logs a message at level Error on the standard logger.
func Errorln(args ...interface{}) {
	logrus.Errorln(args...)
}

// Panicln logs a message at level Panic on the standard logger.
func Panicln(args ...interface{}) {
	logrus.Panicln(args...)
}

// Fatalln logs a message at level Fatal on the standard logger.
func Fatalln(args ...interface{}) {
	logrus.Fatalln(args...)
}

//------ OLD INTERNAL LOGGER BACKWARD COMPARABLE FUNCTIONS -------------------------------
type Level struct {
	Name  string
	Level int
}

var (
	unspecified = Level{"NIL", 0} // Discard first value so 0 can be a placeholder.
	DEBUG       = Level{"DEBUG", 1}
	FINE        = Level{"FINE", 2}
	INFO        = Level{"INFO", 3}
	WARN        = Level{"WARN", 4}
	SEVERE      = Level{"SEVERE", 5}
)

func translateLevel(level Level) logrus.Level {
	switch level.Level {
	case DEBUG.Level:
		fallthrough
	case FINE.Level:
		return logrus.DebugLevel
	case INFO.Level:
		return logrus.InfoLevel
	case WARN.Level:
		return logrus.WarnLevel
	case SEVERE.Level:
		return logrus.ErrorLevel
	default:
		return logrus.DebugLevel
	}
}

func Fine(msg string, args ...interface{}) {
	Debug(msg, args...)
}

func Severe(msg string, args ...interface{}) {
	Error(msg, args...)
}

func SetDefaultLogLevel(level Level) {
	SetLevel(translateLevel(level))
}

// ----- OLD INTERNAL LOGGER --------------------------------------------------------------
//const c_STACK_BUFFER_SIZE int = 8192
//
//// Number of frames to search up the stack for a function not in the log package.
//// Used to find the function name of the entry point.
//const c_NUM_STACK_FRAMES int = 5
//
//// Format for stack info.
//// The lengths of these two format strings should add up so that logs line up correctly.
//// c_STACK_INFO_FMT takes three parameters: filename, line number, function name.
//// c_NO_STACK_FMT takes one parameter: [Unidentified Location]
//const c_STACK_INFO_FMT string = "%20.20s %04d %40.40s"
//const c_NO_STACK_FMT string = "%-66s"
//
//// Format for log level. ie. [FINE ]
//const c_LOG_LEVEL_FMT string = "[%-5.5s]"
//
//// Format for timestamps.
//const c_TIMESTAMP_FMT string = "2006-01-02 15:04:05.000"
//
//type Level struct {
//	Name  string
//	Level int
//}
//
//var (
//	unspecified = Level{"NIL", 0} // Discard first value so 0 can be a placeholder.
//	DEBUG       = Level{"DEBUG", 1}
//	FINE        = Level{"FINE", 2}
//	INFO        = Level{"INFO", 3}
//	WARN        = Level{"WARN", 4}
//	SEVERE      = Level{"SEVERE", 5}
//)
//
//var c_DEFAULT_LOGGING_LEVEL = INFO
//var c_DEFAULT_STACKTRACE_LEVEL = SEVERE
//
//const (
//	Datetime = 1 << iota
//	Stack
//	LevelInfo
//	StdFlags = Datetime | Stack | LevelInfo
//)
//
//type Logger interface {
//	SetLevel(level Level)
//	SetFlags(flags int)
//	SetOutput(writer io.Writer)
//	Log(level Level, msg string, args ...interface{})
//	Debug(msg string, args ...interface{})
//	Fine(msg string, args ...interface{})
//	Info(msg string, args ...interface{})
//	Warn(msg string, args ...interface{})
//	Severe(msg string, args ...interface{})
//}
//
//type DefaultLogger struct {
//	*log.Logger
//	Level           Level
//	StackTraceLevel Level
//	Flags           int
//}
//
//var defaultLogger Logger
//
//func New(out io.Writer, prefix string, flags int) *DefaultLogger {
//	logger := &DefaultLogger{
//		Logger:          log.New(out, prefix, 0),
//		Level:           c_DEFAULT_LOGGING_LEVEL,
//		StackTraceLevel: c_DEFAULT_STACKTRACE_LEVEL,
//		Flags:           flags,
//	}
//
//	return logger
//}
//
//func (l *DefaultLogger) Log(level Level, msg string, args ...interface{}) {
//	if level.Level < l.Level.Level {
//		return
//	}
//
//	// Get current time.
//	now := time.Now()
//
//	// Get information about the stack.
//	// Try and find the first stack frame outside the logging package.
//	// Only search up a few frames, it should never be very far.
//	stackInfo := fmt.Sprintf(c_NO_STACK_FMT, "[Unidentified Location]")
//	for depth := 0; depth < c_NUM_STACK_FRAMES; depth++ {
//		if pc, file, line, ok := runtime.Caller(depth); ok {
//			funcName := runtime.FuncForPC(pc).Name()
//			funcName = path.Base(funcName)
//
//			// Go up another stack frame if this function is in the logging package.
//			isLog := strings.HasPrefix(funcName, "log.")
//			if isLog {
//				continue
//			}
//
//			// Now generate the string.
//			stackInfo = fmt.Sprintf(c_STACK_INFO_FMT,
//				filepath.Base(file),
//				line,
//				funcName)
//			break
//		}
//
//		// If we get here, we failed to retrieve the stack information.
//		// Just give up.
//		break
//	}
//
//	// Write all the data into a buffer.
//	// Format is:
//	// [LEVEL]<timestamp> <file> <line> <function> - <message>
//	var buffer bytes.Buffer
//	if l.Flags&LevelInfo != 0 {
//		buffer.WriteString(fmt.Sprintf(c_LOG_LEVEL_FMT, level.Name))
//	}
//	if l.Flags&Datetime != 0 {
//		buffer.WriteString(now.Format(c_TIMESTAMP_FMT))
//	}
//	if l.Flags&(Datetime|LevelInfo) != 0 {
//		buffer.WriteString(" ")
//	}
//	if l.Flags&Stack != 0 {
//		buffer.WriteString(stackInfo)
//		buffer.WriteString(" - ")
//	}
//	buffer.WriteString(fmt.Sprintf(msg, args...))
//	buffer.WriteString("\n")
//
//	if l.Flags&Stack != 0 && level.Level >= l.StackTraceLevel.Level {
//		buffer.WriteString("--- BEGIN stacktrace: ---\n")
//		buffer.WriteString(stackTrace())
//		buffer.WriteString("--- END stacktrace ---\n\n")
//	}
//
//	l.Logger.Printf(buffer.String())
//}
//
//func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
//	l.Log(DEBUG, msg, args...)
//}
//
//func (l *DefaultLogger) Fine(msg string, args ...interface{}) {
//	l.Log(FINE, msg, args...)
//}
//
//func (l *DefaultLogger) Info(msg string, args ...interface{}) {
//	l.Log(INFO, msg, args...)
//}
//
//func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
//	l.Log(WARN, msg, args...)
//}
//
//func (l *DefaultLogger) Severe(msg string, args ...interface{}) {
//	l.Log(SEVERE, msg, args...)
//}
//
//func (l *DefaultLogger) PrintStack() {
//	l.Logger.Printf(stackTrace())
//}
//
//func (l *DefaultLogger) SetLevel(level Level) {
//	l.Level = level
//}
//
//func (l *DefaultLogger) SetFlags(flags int) {
//	l.Flags = flags
//}
//
//func stackTrace() string {
//	trace := make([]byte, c_STACK_BUFFER_SIZE)
//	count := runtime.Stack(trace, true)
//	return string(trace[:count])
//}
//
//func Debug(msg string, args ...interface{}) {
//	GetDefaultLogger().Debug(msg, args...)
//}
//
//func Fine(msg string, args ...interface{}) {
//	GetDefaultLogger().Fine(msg, args...)
//}
//
//func Info(msg string, args ...interface{}) {
//	GetDefaultLogger().Info(msg, args...)
//}
//
//func Warn(msg string, args ...interface{}) {
//	GetDefaultLogger().Warn(msg, args...)
//}
//
//func Severe(msg string, args ...interface{}) {
//	GetDefaultLogger().Severe(msg, args...)
//}
//
//func SetDefaultLogLevel(level Level) {
//	GetDefaultLogger().SetLevel(level)
//}
//
//func SetDefaultFlags(flags int) {
//	GetDefaultLogger().SetFlags(flags)
//}
//
//func SetDefaultOutput(out io.Writer) {
//	GetDefaultLogger().SetOutput(out)
//}
//
//func GetDefaultLogger() Logger {
//	if defaultLogger == nil {
//		defaultLogger = New(os.Stderr, "", StdFlags)
//	}
//
//	return defaultLogger
//}
//
//func SetDefaultLogger(logger Logger) {
//	defaultLogger = logger
//}
