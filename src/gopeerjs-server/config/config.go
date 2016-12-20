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

	AppKey = "43lN9ZyE_jqWvL_fmYZnoqO5bD7X42QV"
	AppSecretKey = []byte("wOr_agKfOzmeW71w-XX-elPiBXcKFikrakH792zQhuzo4oIzJozhPtMtGiUmxfH42MH8KlB2WAGZF-44eGnEQZG24k1Rx")

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
