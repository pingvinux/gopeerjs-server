package peerhub

import (
	"github.com/garyburd/redigo/redis"
	"encoding/json"
	"sync"
	"github.com/streadway/amqp"
)


var (
	AMQP_MESSAGE_EXCHANGE = "peer-messages"
	AMQP_MESSAGE_TTL = "5000"
)


type PeerHub struct {
	sync.RWMutex

	peers      map[string]*PeerClient

	redisPool  *redis.Pool
	amqpConn   *amqp.Connection
	transmitMessage chan *Message
}

func (h *PeerHub) GetPeer(id string) *PeerClient {
	h.RLock()
	defer h.RUnlock()

	if cl, ok := h.peers[id]; ok == true {
		return cl
	}
	return nil
}

func (h *PeerHub) AddPeer(client *PeerClient) {
	h.Lock()
	defer h.Unlock()

	if _, ok := h.peers[client.Id]; ok == false {
		h.peers[client.Id] = client
	}
}

func (h *PeerHub) RemovePeer(id string) {
	h.Lock()
	defer h.Unlock()

	if _, ok := h.peers[id]; ok == true {
		delete(h.peers, id)
	}
}


func (h *PeerHub) TransmitMessage(message *Message) {
	h.transmitMessage <- message
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
			if amqpMessage.ReplyCode > 0 {
				var message Message
				if err := json.Unmarshal(amqpMessage.Body, &message); err == nil {
					if cl := h.GetPeer(message.Src); cl != nil {
						var msg = NewExpireMessage(message.Dst, message.Src)
						cl.outMessages <- msg
					}
				}
			}
		case clientMessage := <-h.transmitMessage:
			var dst string
			var msg *Message

			switch clientMessage.Type {
			case "LEAVE":
				fallthrough
			case "CANDIDATE":
				fallthrough
			case "OFFER":
				fallthrough
			case "ANSWER":
				dst = clientMessage.Dst
				msg = &Message{
					Type: clientMessage.Type,
					Src: clientMessage.Src,
					Dst: clientMessage.Dst,
					Payload: clientMessage.Payload,
				}
			default:
				dst = clientMessage.Src
				msg = NewErrorMessage(ERROR_MESSAGE_TAKEN)
			}


			var cl = h.GetPeer(dst)
			if cl != nil && cl.IsConnected() {
				//log.Printf("[peerHub] transmit-local message=%s from %s to %s", msg.Type, msg.Src, msg.Dst)

				cl.outMessages <- msg
			} else {
				//log.Printf("[peerHub] transmit-remote message=%s from %s to %s", msg.Type, msg.Src, msg.Dst)

				err := ch.Publish(AMQP_MESSAGE_EXCHANGE, dst, true, false, amqp.Publishing{
					ContentType: "application/json",
					Expiration: AMQP_MESSAGE_TTL,
					Body: msg.Bytes(),
				})
				if err != nil {
					panic(err)
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