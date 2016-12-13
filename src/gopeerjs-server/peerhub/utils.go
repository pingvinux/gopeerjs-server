package peerhub

import (
	"fmt"
	"github.com/satori/go.uuid"
)

func GeneratePeerId() string {
	return fmt.Sprintf("%s", uuid.NewV4())
}

func CheckKey(key string) (bool, error) {
	return true, nil
}
