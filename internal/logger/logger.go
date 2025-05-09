package logger

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func Init() {
	var err error
	config := zap.NewProductionConfig()

	// Устанавливаем уровень логирования на основе переменной окружения
	logLevel := os.Getenv("LOG_LEVEL")
	var level zapcore.Level
	switch logLevel {
	case "DEBUG":
		level = zapcore.DebugLevel
	case "INFO":
		level = zapcore.InfoLevel
	case "WARN":
		level = zapcore.WarnLevel
	case "ERROR":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel // По умолчанию INFO
	}
	config.Level.SetLevel(level)

	_logger, err := config.Build()
	if err != nil {
		panic(err)
	}
	logger = _logger
}

// Проверка инициализации логгера
func ensureLogger() {
	if logger == nil {
		Init()
	}
}

func Fatal(msg string) {
	ensureLogger()
	logger.Fatal(msg)
}

func Fatalf(msg string, args ...any) {
	ensureLogger()
	logger.Fatal(fmt.Sprintf(msg, args...))
}

func FatalKV(msg string, kv ...any) {
	ensureLogger()
	logger.Fatal(msg, parseKV(kv...)...)
}

func Panic(msg string) {
	ensureLogger()
	logger.Panic(msg)
}

func Panicf(msg string, args ...any) {
	ensureLogger()
	logger.Panic(fmt.Sprintf(msg, args...))
}

func PanicKV(msg string, kv ...any) {
	ensureLogger()
	logger.Panic(msg, parseKV(kv...)...)
}

func Error(msg string) {
	ensureLogger()
	logger.Error(msg)
}

func Errorf(msg string, args ...any) {
	ensureLogger()
	logger.Error(fmt.Sprintf(msg, args...))
}

func ErrorKV(msg string, kv ...any) {
	ensureLogger()
	logger.Error(msg, parseKV(kv...)...)
}

func Warn(msg string) {
	ensureLogger()
	logger.Warn(msg)
}

func Warnf(msg string, args ...any) {
	ensureLogger()
	logger.Warn(fmt.Sprintf(msg, args...))
}

func WarnKV(msg string, kv ...any) {
	ensureLogger()
	logger.Warn(msg, parseKV(kv...)...)
}

func Info(msg string) {
	ensureLogger()
	logger.Info(msg)
}

func Infof(msg string, args ...any) {
	ensureLogger()
	logger.Info(fmt.Sprintf(msg, args...))
}

func InfoKV(msg string, kv ...any) {
	ensureLogger()
	logger.Info(msg, parseKV(kv...)...)
}

func Debug(msg string) {
	ensureLogger()
	logger.Debug(msg)
}

func Debugf(msg string, args ...any) {
	ensureLogger()
	logger.Debug(fmt.Sprintf(msg, args...))
}

func DebugKV(msg string, kv ...any) {
	ensureLogger()
	logger.Debug(msg, parseKV(kv...)...)
}

func parseKV(kv ...any) []zap.Field {
	if len(kv)%2 != 0 {
		Panic("kv must be pairs")
	}
	kvs := len(kv) / 2
	fields := make([]zap.Field, 0, kvs)
	for i := 0; i < kvs; i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			Panic("kv key must be string")
		}
		fields = append(fields, zap.Any(k, kv[i+1]))
	}
	return fields
}

func NewTestLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	logLevel := os.Getenv("LOG_LEVEL")
	var level zapcore.Level
	switch logLevel {
	case "DEBUG":
		level = zapcore.DebugLevel
	case "INFO":
		level = zapcore.InfoLevel
	case "WARN":
		level = zapcore.WarnLevel
	case "ERROR":
		level = zapcore.ErrorLevel
	default:
		level = zapcore.InfoLevel
	}
	config.Level.SetLevel(level)
	return config.Build()
}

func SetLogger(l *zap.Logger) {
	logger = l
}
