package peerhub

import (
	"time"
	"github.com/dgrijalva/jwt-go"
	"gopeerjs-server/config"
	"fmt"
)

const (
	KEY_VERSION = 1
)

type PeerKey struct {
	Version int
	Key string
}

func NewKey(clientKey string) *PeerKey {
	return  &PeerKey{Version:KEY_VERSION, Key: clientKey}
}

func Key2Token(k *PeerKey) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"vrs": k.Version,
		"fky": k.Key,
		"nbf": time.Date(2015, 10, 10, 12, 0, 0, 0, time.UTC).Unix(),
	})

	fmt.Printf("%+v", token)

	return token.SignedString(config.AppSecretKey)
}

func Token2Key(tokenString string) (*PeerKey, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return config.AppSecretKey, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, err
	}

	peerKey := &PeerKey{}
	if v, ok := claims["vrs"]; ok == true {
		peerKey.Version, _ = v.(int)
	}
	if v, ok := claims["fky"]; ok == true {
		peerKey.Key, _ = v.(string)
	}

	if peerKey.Version <= 0 || peerKey.Version >= KEY_VERSION {
		return nil, fmt.Errorf("Invalis key version: %v", peerKey.Version)
	}
	if peerKey.Key == "" {
		return nil, fmt.Errorf("Invalis key value: %v", peerKey.Key)
	}

	return peerKey, nil
}