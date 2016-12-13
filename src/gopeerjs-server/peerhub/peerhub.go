package peerhub

import (
	"github.com/garyburd/redigo/redis"
	"time"
	"encoding/json"
	"sync"
	"gopeerjs-server/libs/logger"
	"github.com/streadway/amqp"
)

const (
	AMQP_MESSAGE_EXCHANGE = "peer-messages"

	REDIS_MESSAGE_STORAGE = "tmpmessage:%s"
	REDIS_MESSAGE_TTL = 5 // Message TTL in seconds

	WS_WRITE_WAIT = 5 * time.Second // Websocket write timeout
)

var (
	log = logger.New()
)

type PeerHub struct {
	sync.RWMutex

	redisPool  *redis.Pool
	amqpConn   *amqp.Connection
	peers      map[string]*PeerClient

	transmitMessage chan *Message
}

func (h *PeerHub) GetPeer(id string) *PeerClient {
	log.Info("[PeerHub] GetPeer")

	h.RLock()
	defer h.RUnlock()

	log.Infof("[PeerHub] GetPeer. peer_id=%s", id)

	if cl, ok := h.peers[id]; ok == true {
		return cl
	}
	return nil
}

func (h *PeerHub) AddPeer(client *PeerClient) {
	log.Info("[PeerHub] AddPeer")

	h.Lock()
	defer h.Unlock()

	log.Infof("[PeerHub] AddPeer. peer=%+v", client)

	if _, ok := h.peers[client.Id]; ok == false {
		h.peers[client.Id] = client
	}
}

func (h *PeerHub) RemovePeer(id string) {
	log.Info("[PeerHub] RemovePeer")

	h.Lock()
	defer h.Unlock()

	log.Infof("[PeerHub] Remove Peer. peer_id=%s", id)

	if _, ok := h.peers[id]; ok == true {
		delete(h.peers, id)
	}
}


func (h *PeerHub) TransmitMessage(message *Message) {
	log.Info("[PeerHub] TransmitMessage")

	h.transmitMessage <- message

	log.Infof("[PeerHub] TransmitMessage. Send")
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

	for {
		select {
		case amqpMessage := <-returnQueue:
			log.Infof("[PeerHub] not delivered. %+v", amqpMessage)
			if amqpMessage.ReplyCode > 0 {
				var message Message
				if err := json.Unmarshal(amqpMessage.Body, &message); err == nil {
					if cl := h.GetPeer(message.Src); cl != nil {
						var msg = NewExpireMessage(message.Dst, message.Src)
						cl.outMessages <- msg
					}
				} else {
					log.Infof("[PeerHub] not delivered message. unmarshal error %s", err)
				}
			}
		case clientMessage := <-h.transmitMessage:
			var src = clientMessage.Src
			var dst = clientMessage.Dst
			var cl = h.GetPeer(dst)

			log.Infof("[PeerHub] TransmitMessage from %s to %s", src, dst)

			if cl != nil && cl.IsConnected() {
				log.Infof("[PeerHub] TransmitMessage from %s to %s. transmit to client", src, dst)

				cl.outMessages <- clientMessage
			} else {
				log.Infof("[PeerHub] TransmitMessage from %s to %s. transmit to rabbitmq", src, dst)
				err := ch.Publish(AMQP_MESSAGE_EXCHANGE, dst, true, false, amqp.Publishing{
					ContentType: "application/json",
					Expiration: "5000",
					Body: clientMessage.Bytes(),
				})
				if err != nil {
					log.Infof("[PeerHub] TransmitMessage from %s to %s. error = %s", src, dst, err)
				}
			}
		}
	}
}

func NewHub(redisPool *redis.Pool, amqpConn *amqp.Connection) *PeerHub {
	return &PeerHub{
		redisPool: redisPool,
		amqpConn: amqpConn,
		peers: make(map[string]*PeerClient),
		transmitMessage: make(chan *Message),
	}
}








