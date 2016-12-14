package config

import (
	"github.com/Sirupsen/logrus"
	"flag"
)

var (
	AppBindPath = ""
	AppBindAddress = ""

	AppRedisUrl = ""
	AppAmqpUrl = ""

	Debug bool
	Logger *logrus.Logger

)

func init() {
	flag.StringVar(&AppBindPath, "bind-path", "/", "server bind path prefix")
	flag.StringVar(&AppBindAddress, "bind-address", ":8888", "server bind address")
	flag.StringVar(&AppRedisUrl, "redis", "redis://0.0.0.0:6379/0", "redis connection settings")
	flag.StringVar(&AppAmqpUrl, "rabbitmq", "amqp://guest:guest@0.0.0.0:5672/", "Rabbitmq connection settings")
	flag.BoolVar(&Debug, "debug", true, "debug mod")
	flag.Parse()

	if AppBindPath == "/" {
		AppBindPath = ""
	}

	Logger = logrus.New()
}
