package peerhub

import (
	"github.com/garyburd/redigo/redis"
	"encoding/json"
	"sync"
	"github.com/streadway/amqp"
	"fmt"
	"time"
)


var (
	AMQP_MESSAGE_EXCHANGE = "peer-messages"
	AMQP_MESSAGE_TTL = "5000"

	REDIS_PEER_LIST = "peers:%s:%d"
	REDIS_PEER_LIST_PREFIX = time.Now().Unix()
	REDIS_PEER_LIST_TTL = 6*60*60
)

func redisPeersKey(key string) string {
	return fmt.Sprintf(REDIS_PEER_LIST, key, REDIS_PEER_LIST_PREFIX)
}

type PeerHub struct {
	sync.RWMutex

	peers       map[string]*PeerClient

	redisPool   *redis.Pool
	amqpConn    *amqp.Connection
	peerMessage chan *Message
	peerAdd	    chan *PeerClient
	peerRemove  chan string
}

func (h *PeerHub) GetPeer(id string) *PeerClient {
	h.RLock()
	defer h.RUnlock()

	if cl, ok := h.peers[id]; ok == true {
		return cl
	}
	return nil
}

func (h *PeerHub) AddPeer(client *PeerClient) error {
	h.peerAdd <- client
	return nil
}

func (h *PeerHub) RemovePeer(id string) error {
	h.peerRemove <- id
	return nil
}

func (h *PeerHub) GetAllPeer(key string) (map[string]string, error) {
	conn := h.redisPool.Get()
	defer conn.Close()

	return redis.StringMap(conn.Do("HGETALL", redisPeersKey(key)))
}

func (h *PeerHub) TransmitMessage(message *Message) {
	h.peerMessage <- message
}

func (h *PeerHub) Run() {
	ch, err := h.amqpConn.Channel()
	if err != nil {
		panic(err)
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(AMQP_MESSAGE_EXCHANGE, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		panic(err)
	}

	returnQueue := make(chan amqp.Return)
	ch.NotifyReturn(returnQueue)

	amqpSend := func(dst string, msg *Message) {
		err := ch.Publish(AMQP_MESSAGE_EXCHANGE, dst, true, false, amqp.Publishing{
			ContentType: "application/json",
			Expiration: AMQP_MESSAGE_TTL,
			Body: msg.Bytes(),
		})
		if err != nil {
			panic(err)
		}
	}

	for {
		select {
		case amqpMessage := <-returnQueue:
			if amqpMessage.ReplyCode > 0 {
				var message Message
				if err := json.Unmarshal(amqpMessage.Body, &message); err == nil {
					if cl := h.GetPeer(message.Src); cl != nil {
						var msg = NewExpireMessage(message.Dst, message.Src)
						cl.outMessages <- msg
					}
				}
			}
		case peerAdd := <-h.peerAdd:
			h.Lock()
			if _, ok := h.peers[peerAdd.Id]; ok == false {
				conn := h.redisPool.Get()
				if _, err := conn.Do("HSET", redisPeersKey(peerAdd.Key), peerAdd.Id, peerAdd.Id); err != nil {
					panic(err)
				}
				if _, err := conn.Do("EXPIRE", redisPeersKey(peerAdd.Key), REDIS_PEER_LIST_TTL); err != nil {
					panic(err)
				}

				peers, err := redis.StringMap(conn.Do("HGETALL", redisPeersKey(peerAdd.Key)))
				if err != nil && err != redis.ErrNil {
					panic(err)
				}

				if len(peers) > 1 {
					msg := NewUpdateMessage()
					for peerId := range peers {
						amqpSend(peerId, msg)
					}
				}

				h.peers[peerAdd.Id] = peerAdd
				conn.Close()
			}
			h.Unlock()
		case peerRemoveId := <-h.peerRemove:
			h.Lock()
			if peerRemove, ok := h.peers[peerRemoveId]; ok == true {
				conn := h.redisPool.Get()
				if _, err := conn.Do("HDEL", redisPeersKey(peerRemove.Key), peerRemove.Id); err != nil && err != redis.ErrNil {
					panic(err)
				}

				delete(h.peers, peerRemove.Id)
			}
			h.Unlock()
		case peerMessage := <-h.peerMessage:
			var dst string
			var msg *Message

			switch peerMessage.Type {
			case "LEAVE":
				fallthrough
			case "CANDIDATE":
				fallthrough
			case "OFFER":
				fallthrough
			case "ANSWER":
				dst = peerMessage.Dst
				msg = &Message{
					Type: peerMessage.Type,
					Src: peerMessage.Src,
					Dst: peerMessage.Dst,
					Payload: peerMessage.Payload,
				}
			default:
				dst = peerMessage.Src
				msg = NewErrorMessage(ERROR_MESSAGE_TAKEN)
			}

			cl := h.GetPeer(dst)
			if cl != nil && cl.IsConnected() {
				cl.outMessages <- msg
			} else {
				amqpSend(dst, msg)
			}
		}
	}
}

func NewHub(redisPool *redis.Pool, amqpConn *amqp.Connection) *PeerHub {
	return &PeerHub{
		redisPool: redisPool,
		amqpConn: amqpConn,
		peers: make(map[string]*PeerClient),
		peerMessage: make(chan *Message),
		peerAdd: make(chan *PeerClient),
		peerRemove: make(chan string),
	}
}