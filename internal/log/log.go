package log

import (
	"fmt"
	"log"
	"os"
	"time"
)

var logger *log.Logger

func Init() {
	name := fmt.Sprintf("/tmp/iaws-%s.log", time.Now().Format("20060102-150405"))
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// 无法创建日志文件时静默降级
		logger = log.New(os.Stderr, "[iaws] ", log.LstdFlags|log.Lshortfile)
		return
	}
	logger = log.New(f, "", log.LstdFlags|log.Lshortfile)
	logger.Printf("iaws started, log file: %s", name)
}

func Info(format string, args ...interface{}) {
	if logger != nil {
		logger.Output(2, fmt.Sprintf("[INFO] "+format, args...))
	}
}

func Error(format string, args ...interface{}) {
	if logger != nil {
		logger.Output(2, fmt.Sprintf("[ERROR] "+format, args...))
	}
}

func Debug(format string, args ...interface{}) {
	if logger != nil {
		logger.Output(2, fmt.Sprintf("[DEBUG] "+format, args...))
	}
}
