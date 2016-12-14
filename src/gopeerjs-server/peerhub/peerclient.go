package peerhub

import (
	"time"
	"github.com/clevergo/websocket"
	"net"
	"bytes"
	"encoding/json"
	"sync"
	"github.com/streadway/amqp"
)

var (
	writeWait = 5 * time.Second // Websocket write timeout
	newline = []byte("\n")
	space = []byte(" ")
)

type PeerClient struct {
	sync.Mutex

	Id          string
	Key         string
	Token       string
	IP          net.Addr

	isOpen      bool
	hub         *PeerHub
	wsConn      *websocket.Conn
	outMessages chan *Message
	closeOk     chan bool
	closeError  chan error
	closeQueue  chan bool
}

func (c *PeerClient) SetConnection(conn *websocket.Conn) {
	c.Lock()
	defer c.Unlock()

	c.wsConn = conn
}

func (c *PeerClient) IsConnected() bool {
	c.Lock()
	defer c.Unlock()

	if c.wsConn != nil {
		return true
	}
	return false
}

func (c *PeerClient) Open() bool {
	c.Lock()
	defer c.Unlock()

	if c.isOpen == false {
		c.isOpen = true
		return true
	}
	return false
}

func (c *PeerClient) QueuePump() {
	var ch *amqp.Channel
	var queue amqp.Queue
	var err error

	if ch, err = c.hub.amqpConn.Channel(); err != nil {
		c.closeError <- err
		return
	}
	defer func() {
		ch.Close()
	}()

	if err := ch.ExchangeDeclare(AMQP_MESSAGE_EXCHANGE, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		c.closeError <- err
		return
	}

	if queue, err = ch.QueueDeclare("", false, true, true, false, nil); err != nil {
		c.closeError <- err
		return
	}

	if err := ch.QueueBind(queue.Name, c.Id, AMQP_MESSAGE_EXCHANGE, false, nil); err != nil {
		c.closeError <- err
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
		c.closeError <- err
		return
	}

	go func() {
		for msg := range amqpMessages {
			var message Message
			if err := json.Unmarshal(msg.Body, &message); err == nil {
				c.outMessages <- &message
			}
		}
	}()

	<- c.closeQueue
}

func (c *PeerClient) ReadPump() {
	for {
		_, message, err := c.wsConn.ReadMessage()
		if err != nil {
			//if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
			//	log.Printf("error: %v", err)
			//}

			c.closeError <- err
			return
		}

		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		//log.Printf("[peerClient][%s] Read message=%s", c.Id, message)

		var clientMessage Message
		if err := json.Unmarshal(message, &clientMessage); err != nil {
			//log.Printf("[peerClient][%s] Unmarshal error=%s", c.Id, err)
			continue
		}

		clientMessage.Src = c.Id

		c.hub.transmitMessage <- &clientMessage

		//log.Printf("[peerClient][%s] Send to hub message=%s", c.Id, message)
	}
}

func (c *PeerClient) write(mt int, payload []byte) error {
	c.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.wsConn.WriteMessage(mt, payload)
}

func (c *PeerClient) WritePump() {
	defer c.wsConn.Close()

	for {
		message, ok := <-c.outMessages
		if !ok {
			// The hub closed the channel.
			c.write(websocket.CloseMessage, []byte{})
			return
		}

		//log.Printf("[peerClient][%s] Got message=%s", c.Id, message.Bytes())

		if message.Type == MESSAGE_LEAVE && message.Dst == c.Id {
			c.write(websocket.CloseMessage, []byte{})
			c.closeOk <- true
			return
		}


		c.wsConn.SetWriteDeadline(time.Now().Add(writeWait))
		w, err := c.wsConn.NextWriter(websocket.TextMessage)
		if err != nil {
			//log.Printf("[peerClient][%s] Write error=%s", c.Id, err)

			c.closeError <- err
			return
		}
		if _, err := w.Write(message.Bytes()); err != nil {
			//log.Printf("[peerClient][%s] Write error=%s", c.Id, err)

			c.closeError <- err
			return
		}

		if err := w.Close(); err != nil {
			//log.Printf("[peerClient][%s] Write error=%s", c.Id, err)

			c.closeError <- err
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
	go c.QueuePump()
	go c.ReadPump()
	go c.WritePump()

	var err error

	waitLoop:
	for {
		select {
		case <- c.closeOk:
			break waitLoop
		case err = <-c.closeError:
			break waitLoop
		}
	}

	c.hub.RemovePeer(c.Id)

	if c.closeQueue != nil {
		close(c.closeQueue)
	}
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
		IP: ip,
		hub: hub,
		wsConn: conn,
		outMessages: make(chan *Message),
		closeOk: make(chan bool),
		closeError: make(chan error),
		closeQueue: make(chan bool),
	}
}
