package config

import (
	"fmt"
	"os"
	"time"

	"github.com/natefinch/lumberjack"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var Logger *zap.Logger

// InitLogger initializes the Zap logger with Lumberjack log rotation and a 'logs' folder
func InitLogger() {
	// Ensure the 'logs' directory exists
	err := os.MkdirAll("logs", os.ModePerm)
	if err != nil {
		panic(fmt.Sprintf("Failed to create logs directory: %v", err))
	}

	// Set up log rotation using Lumberjack
	logFile := &lumberjack.Logger{
		Filename:   fmt.Sprintf("logs/%s.log", time.Now().Format("2006-01-02")), // Logs will be named by date
		MaxSize:    10,                                                          // Megabytes (log file size before rotation ie new file 10mb default)
		MaxBackups: 7,                                                           // Keep the last 7 backups
		MaxAge:     28,                                                          // Days (keep logs for 28 days)
		Compress:   true,                                                        // Enable compression of old log files
	}

	// Set up the encoder (human-readable for development)
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoder := zapcore.NewConsoleEncoder(encoderConfig)

	// Create the core with only file output
	core := zapcore.NewCore(
		encoder,
		zapcore.AddSync(logFile),
		zapcore.InfoLevel,
	)

	// Initialize the logger with the core
	Logger = zap.New(core)

	// Ensure logs are flushed to the file
	defer Logger.Sync()
}