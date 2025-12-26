package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Level represents log severity levels
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel converts a string to a Level
func ParseLevel(s string) Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "WARN", "WARNING":
		return WARN
	case "ERROR":
		return ERROR
	case "FATAL":
		return FATAL
	default:
		return INFO
	}
}

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp  string                 `json:"timestamp"`
	Level      string                 `json:"level"`
	Message    string                 `json:"message"`
	Component  string                 `json:"component,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty"`
	File       string                 `json:"file,omitempty"`
	Line       int                    `json:"line,omitempty"`
	Fields     map[string]interface{} `json:"fields,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Duration   string                 `json:"duration,omitempty"`
	StackTrace string                 `json:"stack_trace,omitempty"`
}

// Logger is a structured logger
type Logger struct {
	mu           sync.Mutex
	output       io.Writer
	level        Level
	component    string
	traceID      string
	fields       map[string]interface{}
	includeFile  bool
	jsonFormat   bool
}

// Config holds logger configuration
type Config struct {
	Level       string `json:"level"`
	Output      string `json:"output"`       // "stdout", "stderr", or file path
	Component   string `json:"component"`
	IncludeFile bool   `json:"include_file"` // Include file and line number
	JSONFormat  bool   `json:"json_format"`  // Output as JSON
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// New creates a new logger with the given configuration
func New(cfg *Config) *Logger {
	var output io.Writer = os.Stdout

	if cfg.Output == "stderr" {
		output = os.Stderr
	} else if cfg.Output != "" && cfg.Output != "stdout" {
		file, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err == nil {
			output = file
		}
	}

	return &Logger{
		output:      output,
		level:       ParseLevel(cfg.Level),
		component:   cfg.Component,
		includeFile: cfg.IncludeFile,
		jsonFormat:  cfg.JSONFormat,
		fields:      make(map[string]interface{}),
	}
}

// Default returns the default logger instance
func Default() *Logger {
	once.Do(func() {
		defaultLogger = New(&Config{
			Level:       "INFO",
			Output:      "stdout",
			Component:   "app",
			IncludeFile: false,
			JSONFormat:  true,
		})
	})
	return defaultLogger
}

// SetDefault sets the default logger
func SetDefault(l *Logger) {
	defaultLogger = l
}

// WithComponent returns a new logger with the specified component
func (l *Logger) WithComponent(component string) *Logger {
	newLogger := l.clone()
	newLogger.component = component
	return newLogger
}

// WithTraceID returns a new logger with the specified trace ID
func (l *Logger) WithTraceID(traceID string) *Logger {
	newLogger := l.clone()
	newLogger.traceID = traceID
	return newLogger
}

// WithField returns a new logger with an additional field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := l.clone()
	newLogger.fields[key] = value
	return newLogger
}

// WithFields returns a new logger with additional fields
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	newLogger := l.clone()
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// WithError returns a new logger with an error field
func (l *Logger) WithError(err error) *Logger {
	if err == nil {
		return l
	}
	newLogger := l.clone()
	newLogger.fields["error"] = err.Error()
	return newLogger
}

// WithDuration returns a new logger with duration field
func (l *Logger) WithDuration(d time.Duration) *Logger {
	newLogger := l.clone()
	newLogger.fields["duration"] = d.String()
	return newLogger
}

func (l *Logger) clone() *Logger {
	fields := make(map[string]interface{}, len(l.fields))
	for k, v := range l.fields {
		fields[k] = v
	}
	return &Logger{
		output:      l.output,
		level:       l.level,
		component:   l.component,
		traceID:     l.traceID,
		fields:      fields,
		includeFile: l.includeFile,
		jsonFormat:  l.jsonFormat,
	}
}

// log writes a log entry
func (l *Logger) log(level Level, msg string, args ...interface{}) {
	if level < l.level {
		return
	}

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level.String(),
		Message:   msg,
		Component: l.component,
		TraceID:   l.traceID,
	}

	// Copy existing fields
	if len(l.fields) > 0 {
		entry.Fields = make(map[string]interface{}, len(l.fields)+len(args)/2)
		for k, v := range l.fields {
			entry.Fields[k] = v
		}
	}

	// Handle args - support both printf-style and structured key-value pairs
	if len(args) > 0 {
		// Check if args look like key-value pairs (even count, first arg is string)
		if len(args) >= 2 && len(args)%2 == 0 {
			if _, ok := args[0].(string); ok {
				// Treat as key-value pairs
				if entry.Fields == nil {
					entry.Fields = make(map[string]interface{}, len(args)/2)
				}
				for i := 0; i < len(args); i += 2 {
					if key, ok := args[i].(string); ok {
						// Convert errors to strings for proper JSON serialization
						if err, isErr := args[i+1].(error); isErr {
							if err != nil {
								entry.Fields[key] = err.Error()
							} else {
								entry.Fields[key] = nil
							}
						} else {
							entry.Fields[key] = args[i+1]
						}
					}
				}
			} else {
				// Printf-style formatting
				entry.Message = fmt.Sprintf(msg, args...)
			}
		} else {
			// Printf-style formatting
			entry.Message = fmt.Sprintf(msg, args...)
		}
	}

	if l.includeFile {
		_, file, line, ok := runtime.Caller(2)
		if ok {
			// Get just the filename, not the full path
			parts := strings.Split(file, "/")
			entry.File = parts[len(parts)-1]
			entry.Line = line
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.jsonFormat {
		data, _ := json.Marshal(entry)
		fmt.Fprintln(l.output, string(data))
	} else {
		l.writeText(entry)
	}
}

func (l *Logger) writeText(entry LogEntry) {
	var b strings.Builder

	// Timestamp
	b.WriteString(entry.Timestamp[:19]) // Trim nanoseconds for text format
	b.WriteString(" ")

	// Level with color codes for terminal
	levelStr := fmt.Sprintf("[%-5s]", entry.Level)
	b.WriteString(levelStr)
	b.WriteString(" ")

	// Component
	if entry.Component != "" {
		b.WriteString("[")
		b.WriteString(entry.Component)
		b.WriteString("] ")
	}

	// TraceID
	if entry.TraceID != "" {
		b.WriteString("{")
		b.WriteString(entry.TraceID[:8])
		b.WriteString("} ")
	}

	// Message
	b.WriteString(entry.Message)

	// Fields
	if len(entry.Fields) > 0 {
		b.WriteString(" | ")
		first := true
		for k, v := range entry.Fields {
			if !first {
				b.WriteString(", ")
			}
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(fmt.Sprintf("%v", v))
			first = false
		}
	}

	// File location
	if entry.File != "" {
		b.WriteString(fmt.Sprintf(" (%s:%d)", entry.File, entry.Line))
	}

	fmt.Fprintln(l.output, b.String())
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.log(DEBUG, msg, args...)
}

// Info logs an info message
func (l *Logger) Info(msg string, args ...interface{}) {
	l.log(INFO, msg, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, args ...interface{}) {
	l.log(WARN, msg, args...)
}

// Error logs an error message
func (l *Logger) Error(msg string, args ...interface{}) {
	l.log(ERROR, msg, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.log(FATAL, msg, args...)
	os.Exit(1)
}

// Package-level functions for default logger

// Debug logs a debug message using the default logger
func Debug(msg string, args ...interface{}) {
	Default().Debug(msg, args...)
}

// Info logs an info message using the default logger
func Info(msg string, args ...interface{}) {
	Default().Info(msg, args...)
}

// Warn logs a warning message using the default logger
func Warn(msg string, args ...interface{}) {
	Default().Warn(msg, args...)
}

// Error logs an error message using the default logger
func Error(msg string, args ...interface{}) {
	Default().Error(msg, args...)
}

// Fatal logs a fatal message using the default logger
func Fatal(msg string, args ...interface{}) {
	Default().Fatal(msg, args...)
}

// WithComponent returns a new logger with the specified component
func WithComponent(component string) *Logger {
	return Default().WithComponent(component)
}

// WithTraceID returns a new logger with the specified trace ID
func WithTraceID(traceID string) *Logger {
	return Default().WithTraceID(traceID)
}

// WithField returns a new logger with an additional field
func WithField(key string, value interface{}) *Logger {
	return Default().WithField(key, value)
}

// WithFields returns a new logger with additional fields
func WithFields(fields map[string]interface{}) *Logger {
	return Default().WithFields(fields)
}

// WithError returns a new logger with an error field
func WithError(err error) *Logger {
	return Default().WithError(err)
}
