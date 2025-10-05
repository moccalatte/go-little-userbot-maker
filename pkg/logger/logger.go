package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Level string

const (
	LevelInfo  Level = "INFO"
	LevelWarn  Level = "WARN"
	LevelError Level = "ERROR"
	LevelDebug Level = "DEBUG"
)

// Logger is a simple file-based logger.
type Logger struct {
	serviceName string
	telegramID  string
	logFile     *os.File
	writer      io.Writer
}

// New creates a new Logger instance for a specific service and telegram user.
// The telegramID can be a generic identifier like "main" for service-wide logs.
func New(serviceName, telegramID string) (*Logger, error) {
	l := &Logger{
		serviceName: serviceName,
		telegramID:  telegramID,
	}

	if err := l.configureOutput(); err != nil {
		return nil, err
	}

	// Start a goroutine to handle daily log rotation.
	go l.rotateLogFileDaily()
	// Start a goroutine for periodic log cleanup.
	go l.cleanupOldLogsPeriodically()

	return l, nil
}

func (l *Logger) getLogFilePath() (string, error) {
	date := time.Now().Format("2006-01-02")
	logDir := filepath.Join("logs", l.serviceName, l.telegramID)

	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create log directory: %w", err)
	}

	return filepath.Join(logDir, fmt.Sprintf("%s.log", date)), nil
}

func (l *Logger) configureOutput() error {
	path, err := l.getLogFilePath()
	if err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.logFile = file
	l.writer = io.MultiWriter(os.Stdout, file)
	log.SetOutput(l.writer)
	log.SetFlags(0) // Disable standard logger flags as we have our own format.

	return nil
}

func (l *Logger) rotateLogFileDaily() {
	for {
		// Calculate duration until midnight.
		now := time.Now()
		midnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 1, 0, now.Location())
		time.Sleep(time.Until(midnight))

		// Close the old file and open a new one.
		l.logFile.Close()
		if err := l.configureOutput(); err != nil {
			log.Printf("[ERROR] [logger] failed to rotate log file: %v", err)
		}
	}
}

func (l *Logger) cleanupOldLogsPeriodically() {
	// Run cleanup once on startup, then every 24 hours.
	l.cleanupOldLogs()
	for {
		time.Sleep(24 * time.Hour)
		l.cleanupOldLogs()
	}
}

// cleanupOldLogs removes log files older than 7 days.
func (l *Logger) cleanupOldLogs() {
	logDir := filepath.Join("logs", l.serviceName, l.telegramID)
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)

	err := filepath.Walk(logDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".log") {
			logDateStr := strings.TrimSuffix(info.Name(), ".log")
			logDate, err := time.Parse("2006-01-02", logDateStr)
			if err == nil && logDate.Before(sevenDaysAgo) {
				if removeErr := os.Remove(path); removeErr != nil {
					log.Printf("[ERROR] [logger] failed to remove old log file %s: %v", path, removeErr)
				} else {
					log.Printf("[INFO] [logger] removed old log file: %s", path)
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("[ERROR] [logger] failed to walk log directory for cleanup: %v", err)
	}
}

func (l *Logger) log(level Level, message string) {
	timestamp := time.Now().Format("2006-01-02T15:04:05Z")
	log.Printf("[%s] [%s] [%s] [%s] %s", timestamp, level, l.serviceName, l.telegramID, message)
}

// Info logs an informational message.
func (l *Logger) Info(format string, v ...interface{}) {
	l.log(LevelInfo, fmt.Sprintf(format, v...))
}

// Warn logs a warning message.
func (l *Logger) Warn(format string, v ...interface{}) {
	l.log(LevelWarn, fmt.Sprintf(format, v...))
}

// Error logs an error message.
func (l *Logger) Error(format string, v ...interface{}) {
	l.log(LevelError, fmt.Sprintf(format, v...))
}

// Debug logs a debug message.
func (l *Logger) Debug(format string, v ...interface{}) {
	l.log(LevelDebug, fmt.Sprintf(format, v...))
}

// SetOutput redirects stdout and stderr to the logger.
// This is a global change and should be used carefully at the start of the application.
func (l *Logger) SetOutput() error {
    // Pipe stdout
    stdoutR, stdoutW, _ := os.Pipe()
    os.Stdout = stdoutW
    go l.pipeToLog(stdoutR)

    // Pipe stderr
    stderrR, stderrW, _ := os.Pipe()
    os.Stderr = stderrW
    go l.pipeToLog(stderrR)

    return nil
}

func (l *Logger) pipeToLog(reader *os.File) {
    buf := make([]byte, 1024)
    for {
        n, err := reader.Read(buf)
        if err != nil {
            // This can happen if the pipe is closed.
            return
        }
        message := strings.TrimSpace(string(buf[:n]))
        if message != "" {
            l.log(LevelInfo, message) // Log everything from stdout/stderr as INFO
        }
    }
}