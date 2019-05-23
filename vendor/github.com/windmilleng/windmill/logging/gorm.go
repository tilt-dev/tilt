package logging

import (
	"github.com/jinzhu/gorm"
)

func GormDefaultLogger() defaultLogger {
	return defaultLogger{}
}

type defaultLogger struct{}

func (l defaultLogger) Print(v ...interface{}) {
	Global().Print(gorm.LogFormatter(v...)...)
}
