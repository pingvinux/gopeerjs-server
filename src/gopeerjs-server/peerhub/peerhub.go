package peerhub

import (
	"github.com/garyburd/redigo/redis"
	"encoding/json"
	"sync"
	"github.com/streadway/amqp"
	"fmt"
	"time"
	"strings"
	"log"
)

var (
	AMQP_MESSAGE_EXCHANGE = "peer-messages"
	AMQP_MESSAGE_TTL = "5000"

	REDIS_PEER_LIST = "peers:%s:%d"
	REDIS_PEER_SEGMENTS = "segments:%s:%d"
	REDIS_PEER_LIST_PREFIX = time.Now().Unix()
	REDIS_PEER_LIST_TTL = 6 * 60 * 60
)

func redisPeersKey(key string) string {
	return fmt.Sprintf(REDIS_PEER_LIST, key, REDIS_PEER_LIST_PREFIX)
}

func redisSegmentsKey(key string) string {
	return fmt.Sprintf(REDIS_PEER_SEGMENTS, key, REDIS_PEER_LIST_PREFIX)
}

type PeerHub struct {
	sync.RWMutex

	peers        map[string]*PeerClient

	redisPool    *redis.Pool
	amqpConn     *amqp.Connection
	peerMessage  chan *Message
	peerAdd      chan *PeerClient
	peerSegments chan *PeerClient
	peerRemove   chan *PeerClient
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

func (h *PeerHub) RemovePeer(client *PeerClient) error {
	h.peerRemove <- client
	return nil
}

func (h *PeerHub) GetAllPeer(key string) (map[string]string, error) {
	conn := h.redisPool.Get()
	defer conn.Close()

	return redis.StringMap(conn.Do("HGETALL", redisPeersKey(key)))
}

func (h *PeerHub) GetAllSegments(key string) (map[string]string, error) {
	conn := h.redisPool.Get()
	defer conn.Close()

	return redis.StringMap(conn.Do("HGETALL", redisSegmentsKey(key)))
}

func (h *PeerHub) UpdatePeerSegments(key, peerId string, segments []string) error {
	conn := h.redisPool.Get()
	defer conn.Close()

	redisKey := redisSegmentsKey(key)

	if _, err := conn.Do("MULTI"); err != nil {
		return err
	}
	if _, err := conn.Do("HSET", redisKey, peerId, strings.Join(segments, ",")); err != nil {
		return err
	}
	if _, err := conn.Do("EXPIRE", redisKey, REDIS_PEER_LIST_TTL); err != nil {
		return err
	}
	if _, err := conn.Do("EXEC"); err != nil {
		return err
	}

	return nil
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
		case peerClient := <-h.peerSegments:
			conn := h.redisPool.Get()

			segmentsStr, err := redis.String(conn.Do("HGET", redisSegmentsKey(peerClient.Key), peerClient.Id))
			if err != nil && err != redis.ErrNil {
				panic(err)
			}

			peers, err := redis.StringMap(conn.Do("HGETALL", redisPeersKey(peerClient.Key)))
			if err != nil && err != redis.ErrNil {
				panic(err)
			}

			if len(segmentsStr) > 0 && len(peers) > 1 {
				delete(peers, peerClient.Id)

				segments := map[string][]string{
					peerClient.Id: strings.Split(segmentsStr, ","),
				}
				for peerId := range peers {
					msg := NewMessage(peerClient.Id, peerId, "SEGMENTS", segments)
					amqpSend(peerId, msg)
				}
			}
			conn.Close()
		case peerClient := <-h.peerAdd:
			h.Lock()
			if _, ok := h.peers[peerClient.Id]; ok == false {
				conn := h.redisPool.Get()

				if _, err := conn.Do("HSET", redisPeersKey(peerClient.Key), peerClient.Id, ""); err != nil {
					panic(err)
				}
				if _, err := conn.Do("EXPIRE", redisPeersKey(peerClient.Key), REDIS_PEER_LIST_TTL); err != nil {
					panic(err)
				}

				h.peers[peerClient.Id] = peerClient

				peers, err := redis.StringMap(conn.Do("HGETALL", redisPeersKey(peerClient.Key)))
				if err != nil && err != redis.ErrNil {
					panic(err)
				}
				if len(peers) > 1 {
					msg := NewUpdateMessage()
					for peerId := range peers {
						log.Printf("Generate update message to %s", peerId)

						amqpSend(peerId, msg)
					}
				}
				conn.Close()
			}
			h.Unlock()

		case peerClient := <-h.peerRemove:
			h.Lock()

			if peerClient, ok := h.peers[peerClient.Id]; ok == true {
				delete(h.peers, peerClient.Id)
			}

			conn := h.redisPool.Get()
			if _, err := conn.Do("HDEL", redisPeersKey(peerClient.Key), peerClient.Id); err != nil && err != redis.ErrNil {
				panic(err)
			}
			if _, err := conn.Do("HDEL", redisSegmentsKey(peerClient.Key), peerClient.Id); err != nil && err != redis.ErrNil {
				panic(err)
			}

			conn.Close()
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
		peerSegments: make(chan *PeerClient),
		peerRemove: make(chan *PeerClient),
	}
}