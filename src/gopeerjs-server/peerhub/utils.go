package peerhub

import (
	"fmt"
	"github.com/satori/go.uuid"
	"gopeerjs-server/config"
)

func GeneratePeerId() string {
	return fmt.Sprintf("%s", uuid.NewV4())
}

func CheckKey(key string) (bool, error) {
	return true, nil
}

func CheckAppkey(key string) bool {
	if key == config.AppKey {
		return true
	}
	return false
}
