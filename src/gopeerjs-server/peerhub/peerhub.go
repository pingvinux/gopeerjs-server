package peerhub

import (
	"github.com/garyburd/redigo/redis"
	"fmt"
	"time"
	"encoding/json"
	"sync"
	"github.com/satori/go.uuid"
	"gopeerjs-server/libs/logger"
)

const (
	REDIS_MESSAGE_STORAGE = "tmpmessage:%s"
	REDIS_MESSAGE_TTL = 5 // Message TTL in seconds

	WS_WRITE_WAIT = 5 * time.Second // Websocket write timeout
)

var (
	log = logger.New()
)

func redisGetMessageKey(clientId string) string {
	return fmt.Sprintf(REDIS_MESSAGE_STORAGE, clientId)
}

type PeerHub struct {
	sync.RWMutex

	redisPool *redis.Pool
	peers      map[string]*PeerClient

	Register   chan *PeerClient
	Unregister chan *PeerClient
}

func (h *PeerHub) CheckKey(key string) (bool, error) {
	return true, nil
}

func (h *PeerHub) GeneratePeerId() string {
	return fmt.Sprintf("%s", uuid.NewV4())
}

func (h *PeerHub) GetPeer(id string) *PeerClient {
	h.RLock()
	defer h.RUnlock()

	log.Infof("[PeerHub] Get Peer. peer_id=%s", id)

	if cl, ok := h.peers[id]; ok == true {
		return cl
	}
	return nil
}

func (h *PeerHub) AddPeer(client *PeerClient) {
	log.Infof("[PeerHub] Call AddPeer")

	h.Lock()
	defer h.Unlock()

	log.Infof("[PeerHub] Add Peer. peer=%+v", client)

	if _, ok := h.peers[client.Id]; ok == false {
		h.peers[client.Id] = client
	}
}

func (h *PeerHub) RemovePeer(id string) {
	log.Infof("[PeerHub] RemovePeer")

	h.Lock()
	defer h.Unlock()

	log.Infof("[PeerHub] Remove Peer. peer_id=%s", id)

	if _, ok := h.peers[id]; ok == true {
		delete(h.peers, id)
	}
}

func (h *PeerHub) StoreLostMessage(clientId string, message *Message) error {
	conn := h.redisPool.Get()
	defer conn.Close()

	messageKey := redisGetMessageKey(clientId)
	messageTime := time.Now().Unix()
	messageStr, _ := json.Marshal(message)

	_, err := conn.Do("WATCH", messageKey)
	if err != nil {
		return err
	}

	if _, err := conn.Do("MULTI"); err != nil {
		return err
	}
	if _, err := conn.Do("ZADD", messageKey, messageTime, messageStr); err != nil {
		return err
	}
	if _, err := conn.Do("EXPIRE", messageKey, REDIS_MESSAGE_TTL); err != nil {
		return err
	}
	if _, err := conn.Do("EXEC"); err != nil {
		return err
	}

	conn.Do("UNWATCH", messageKey)
	return nil
}

func (h *PeerHub) GetLostMessage(clientId string) ([]Message, error) {
	conn := h.redisPool.Get()
	defer conn.Close()

	messageKey := redisGetMessageKey(clientId)
	messageList := make([]Message, 0)

	messageBytes, err := redis.ByteSlices(conn.Do("ZRANGEBYSCORE", messageKey, "-inf", "+inf"))
	if err != nil && err != redis.ErrNil {
		return messageList, err
	} else if err == redis.ErrNil {
		return messageList, nil
	}

	if len(messageBytes) > 0 {
		for _, mBytes := range messageBytes {
			var tmpMessage Message
			if err := json.Unmarshal(mBytes, &tmpMessage); err != nil {
				continue
			}

			messageList = append(messageList, tmpMessage)
		}

		conn.Do("DEL", messageKey)
	}

	return messageList, nil
}

func (h *PeerHub) TransmitMessage(message *Message) {
	var src = message.Src
	var dst = message.Dst
	var cl = h.GetPeer(dst)

	log.Infof("[PeerHub] TransmitMessage. message=%+v", message)

	if cl != nil && cl.IsConnected() {
		cl.send <- message.Bytes()
	} else {
		if message.Type != "LEAVE" && message.Type != "EXPIRE" && dst != "" {
			h.StoreLostMessage(dst, message)
		} else if message.Type == "LEAVE" && dst == "" {
			h.RemovePeer(src)
		}
	}
}

func NewHub(redis *redis.Pool) *PeerHub {
	return &PeerHub{
		redisPool: redis,
		peers: make(map[string]*PeerClient),
	}
}








