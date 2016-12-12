package logger

import (
	"github.com/Sirupsen/logrus"
	"sync"
)

var logger *logrus.Logger
var once sync.Once

func New() *logrus.Logger {
	once.Do(func(){
		logger = logrus.New()
	})
	return logger
}
