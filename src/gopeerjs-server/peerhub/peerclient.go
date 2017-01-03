package peerhub

import (
	"time"
	"github.com/clevergo/websocket"
	"net"
	"bytes"
	"encoding/json"
	"sync"
	"github.com/streadway/amqp"
	"log"
)

var (
	writeWait = 5 * time.Second // Websocket write timeout
	newline = []byte("\n")
	space = []byte(" ")
)

type PeerClient struct {
	Id          string `json:"id"`
	Key         string `json:"key"`
	Token       string `json:"token"`
	IP          string `json:"ip"`

	isOpen      bool
	mutex       *sync.Mutex
	hub         *PeerHub
	wsConn      *websocket.Conn
	outMessages chan *Message
	close       chan error
}

func (c *PeerClient) SetConnection(conn *websocket.Conn) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.wsConn = conn
}

func (c *PeerClient) IsConnected() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.wsConn != nil {
		return true
	}
	return false
}

func (c *PeerClient) Open() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.isOpen == false {
		c.isOpen = true
		return true
	}
	return false
}

func (c *PeerClient) ReadPump() {
	for {
		_, message, err := c.wsConn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				c.close <- err
			} else {
				c.close <- nil
			}
			return
		}

		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		//log.Printf("[peerClient][%s] Read message=%s", c.Id, message)

		var clientMessage Message
		if err := json.Unmarshal(message, &clientMessage); err != nil {
			//log.Printf("[peerClient][%s] Unmarshal error=%s", c.Id, err)
			continue
		}

		if clientMessage.Type == MESSAGE_SEGMENTS {
			var segments []string;

			if err := json.Unmarshal(clientMessage.Payload, &segments); err == nil {
				if err := c.hub.UpdatePeerSegments(c.Key, c.Id, segments); err == nil {
					c.hub.peerSegments <- c
				}
			}
		} else {
			clientMessage.Src = c.Id
			c.hub.peerMessage <- &clientMessage
		}
		//log.Printf("[peerClient][%s] Send to hub message=%s", c.Id, message)
	}
}

func (c *PeerClient) write(mt int, payload []byte) error {
	c.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.wsConn.WriteMessage(mt, payload)
}

func (c *PeerClient) WritePump() {
	defer c.wsConn.Close()

	var ch *amqp.Channel
	var queue amqp.Queue
	var err error

	if ch, err = c.hub.amqpConn.Channel(); err != nil {
		c.close <- err
		return
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare(AMQP_MESSAGE_EXCHANGE, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		c.close <- err
		return
	}

	if queue, err = ch.QueueDeclare("", false, true, true, false, nil); err != nil {
		c.close <- err
		return
	}

	if err := ch.QueueBind(queue.Name, c.Id, AMQP_MESSAGE_EXCHANGE, false, nil); err != nil {
		c.close <- err
		return
	}

	amqpMessages, err := ch.Consume(
		queue.Name, // queue
		"", // consumer
		true, // auto ack
		false, // exclusive
		false, // no local
		false, // no wait
		nil, // args
	)
	if err != nil {
		c.close <- err
		return
	}
	go func() {
		for msg := range amqpMessages {
			var message Message
			if err := json.Unmarshal(msg.Body, &message); err == nil {
				if message.Type == MESSAGE_PEERS_UPDATE {
					peers := make([]string, 0)
					if pl, err := c.hub.GetAllPeer(c.Key); err == nil {
						delete(pl, c.Id)
						for peerId := range pl {
							peers = append(peers, peerId)
						}
					}
					if len(peers) > 0 {
						c.outMessages <- NewMessage("", c.Id, MESSAGE_PEERS, peers)
					}
					log.Printf("[%s] peers:%+v", c.Id, peers)
				} else {
					c.outMessages <- &message
				}
			}
		}
	}()

	for {

		message, ok := <-c.outMessages
		if !ok {
			// The hub closed the channel.
			c.write(websocket.CloseMessage, []byte{})
			return
		}

		log.Printf("[peerClient][%s] Got message=%s", c.Id, message.Bytes())

		if message.Type == MESSAGE_LEAVE && message.Dst == c.Id {
			c.write(websocket.CloseMessage, []byte{})
			c.close <- nil
			return
		}

		c.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
		w, err := c.wsConn.NextWriter(websocket.TextMessage)
		if err != nil {
			//log.Printf("[peerClient][%s] Write error=%s", c.Id, err)

			c.close <- err
			return
		}
		if _, err := w.Write(message.Bytes()); err != nil {
			//log.Printf("[peerClient][%s] Write error=%s", c.Id, err)

			c.close <- err
			return
		}

		if err := w.Close(); err != nil {
			//log.Printf("[peerClient][%s] Write error=%s", c.Id, err)

			c.close <- err
			return
		}

		//log.Printf("[peerClient][%s] Write message=%s", c.Id, message.Bytes())
	}
}

func (c *PeerClient) WriteClose() {
	c.write(websocket.CloseMessage, []byte{})
	c.wsConn.Close()
}

func (c *PeerClient) Wait() error {
	defer c.hub.RemovePeer(c)

	err := <-c.close

	if c.outMessages != nil {
		close(c.outMessages)
	}

	return err
}

func NewClient(id string, key string, token string, ip net.Addr, hub *PeerHub, conn *websocket.Conn) *PeerClient {
	return &PeerClient{
		Id: id,
		Key: key,
		Token: token,
		IP: ip.String(),

		mutex: &sync.Mutex{},
		hub: hub,
		wsConn: conn,
		outMessages: make(chan *Message),
		close: make(chan error),
	}
}
