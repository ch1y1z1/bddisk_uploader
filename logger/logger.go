package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

var (
	levelNames = map[LogLevel]string{
		DEBUG: "DEBUG",
		INFO:  "INFO",
		WARN:  "WARN",
		ERROR: "ERROR",
		FATAL: "FATAL",
	}
)

type Logger struct {
	logger   *log.Logger
	level    LogLevel
	mu       sync.RWMutex
	writers  []io.Writer
	progress bool // 是否显示进度信息
}

var defaultLogger *Logger
var once sync.Once

// Init 初始化默认日志器
func Init(level LogLevel, logFile string, showProgress bool) error {
	var err error
	once.Do(func() {
		defaultLogger, err = NewLogger(level, logFile, showProgress)
	})
	return err
}

// NewLogger 创建新的日志器
func NewLogger(level LogLevel, logFile string, showProgress bool) (*Logger, error) {
	writers := []io.Writer{os.Stdout}

	// 如果指定了日志文件，同时写入文件
	if logFile != "" {
		// 确保日志目录存在
		if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
			return nil, fmt.Errorf("创建日志目录失败: %v", err)
		}

		file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("打开日志文件失败: %v", err)
		}
		writers = append(writers, file)
	}

	multiWriter := io.MultiWriter(writers...)
	logger := log.New(multiWriter, "", log.LstdFlags)

	return &Logger{
		logger:   logger,
		level:    level,
		writers:  writers,
		progress: showProgress,
	}, nil
}

// GetDefault 获取默认日志器
func GetDefault() *Logger {
	if defaultLogger == nil {
		// 如果没有初始化，使用默认设置
		defaultLogger, _ = NewLogger(INFO, "", true)
	}
	return defaultLogger
}

// SetLevel 设置日志级别
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// GetLevel 获取当前日志级别
func (l *Logger) GetLevel() LogLevel {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.level
}

// shouldLog 判断是否应该记录该级别的日志
func (l *Logger) shouldLog(level LogLevel) bool {
	return level >= l.GetLevel()
}

// log 内部日志记录方法
func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if !l.shouldLog(level) {
		return
	}

	levelName := levelNames[level]
	message := fmt.Sprintf(format, args...)
	l.logger.Printf("[%s] %s", levelName, message)
}

// Debug 调试日志
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info 信息日志
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn 警告日志
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error 错误日志
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal 致命错误日志（会调用os.Exit(1)）
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
	os.Exit(1)
}

// Progress 进度信息（如果启用了进度显示）
func (l *Logger) Progress(format string, args ...interface{}) {
	if l.progress {
		message := fmt.Sprintf(format, args...)
		l.logger.Printf("[PROGRESS] %s", message)
	}
}

// 全局便捷方法
func Debug(format string, args ...interface{}) {
	GetDefault().Debug(format, args...)
}

func Info(format string, args ...interface{}) {
	GetDefault().Info(format, args...)
}

func Warn(format string, args ...interface{}) {
	GetDefault().Warn(format, args...)
}

func Error(format string, args ...interface{}) {
	GetDefault().Error(format, args...)
}

func Fatal(format string, args ...interface{}) {
	GetDefault().Fatal(format, args...)
}

func Progress(format string, args ...interface{}) {
	GetDefault().Progress(format, args...)
}

func SetLevel(level LogLevel) {
	GetDefault().SetLevel(level)
}