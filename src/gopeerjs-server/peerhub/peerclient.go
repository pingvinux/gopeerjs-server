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
	newline = []byte("\n")
	space = []byte(" ")
)

type PeerClient struct {
	sync.Mutex

	Id           string
	Key          string
	Token        string
	IP           net.Addr
	isOpen       bool

	hub          *PeerHub
	wsConn       *websocket.Conn
	outMessages  chan *Message
	close        chan bool
	closeQueue   chan bool
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
	log.Infof("[PeerClient][%s] Start QueuePump", c.Id)

	var ch *amqp.Channel
	var queue amqp.Queue
	var err error

	if ch, err = c.hub.amqpConn.Channel(); err != nil {
		log.Infof("[PeerClient][%s] QueuePump. Error=%s", c.Id, err)

		c.close <- true
		return
	}
	defer func() {
		ch.Close()

		log.Infof("[PeerClient][%s] Close QueuePump", c.Id)
	}()

	if err := ch.ExchangeDeclare(AMQP_MESSAGE_EXCHANGE, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		log.Infof("[PeerClient][%s] QueuePump. Error=%s", c.Id, err)
		c.close <- true
		return
	}

	if queue, err = ch.QueueDeclare("", false, true, true, false, nil); err != nil {
		log.Infof("[PeerClient][%s] QueuePump. Error=%s", c.Id, err)
		c.close <- true
		return
	}

	if err := ch.QueueBind(queue.Name, c.Id, AMQP_MESSAGE_EXCHANGE, false, nil); err != nil {
		log.Infof("[PeerClient][%s] QueuePump. Error=%s", c.Id, err)
		c.close <- true
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
		log.Infof("[PeerClient][%s] QueuePump. Error=%s", c.Id, err)
		c.close <- true
		return
	}

	go func() {
		for msg := range amqpMessages {
			var message Message
			if err := json.Unmarshal(msg.Body, &message); err != nil {
				log.Info("[PeerClient] QueuePump. unmarshal error=%s", err)
				continue
			}
			if message.Type != "LEAVE" {
				c.outMessages <- &message
			} else {
				c.close <- true
				return
			}
		}
	}()

	<- c.closeQueue
}

func (c *PeerClient) ReadPump() {
	defer func(){
		log.Infof("[PeerClient][%s] Close ReadPump", c.Id)
	}()

	log.Infof("[PeerClient][%s] Start ReadPump", c.Id)

	for {
		_, message, err := c.wsConn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}

			log.Infof("[PeerClient][%s] ReadPump. read error=%s", c.Id, err)

			// You should send the closed message to the close channel,
			// otherwise, the server will be crashed.
			c.close <- true
			return
		}

		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		log.Infof("[PeerClient][%s] ReadPump. raw_message=%s", c.Id, message)

		var clientMessage Message
		if err := json.Unmarshal(message, &clientMessage); err != nil {
			log.Infof("[PeerClient][%s] ReadPump. unmarshal error=%s", c.Id, err)
			continue
		}

		log.Infof("[PeerClient][%s] ReadPump. message=%+v", c.Id, clientMessage)

		switch clientMessage.Type {
		case "LEAVE":
			fallthrough
		case "CANDIDATE":
			fallthrough
		case "OFFER":
			fallthrough
		case "ANSWER":
			msg := &Message{
				Type: clientMessage.Type,
				Src: c.Id,
				Dst: clientMessage.Dst,
				Payload: clientMessage.Payload,
			}

			log.Infof("[PeerClient][%s] ReadPump. send to c.hub.transmitMessage", c.Id)

			c.hub.transmitMessage <- msg

			log.Infof("[PeerClient][%s] ReadPump. send to c.hub.transmitMessage end", c.Id)
		default:
		// Error Message unrecognized
		}
	}
}

func (c *PeerClient) write(mt int, payload []byte) error {
	c.wsConn.SetWriteDeadline(time.Now().Add(WS_WRITE_WAIT))
	return c.wsConn.WriteMessage(mt, payload)
}

func (c *PeerClient) Write(playload []byte) error {
	return c.write(websocket.TextMessage, playload)
}

func (c *PeerClient) WritePump() {
	defer func() {
		c.wsConn.Close()
		log.Infof("[PeerClient][%s] Close WritePump", c.Id)
	}()

	log.Infof("[PeerClient][%s] Start WritePump", c.Id)

	for {
		message, ok := <-c.outMessages
		if !ok {
			// The hub closed the channel.
			c.write(websocket.CloseMessage, []byte{})
			return
		}

		c.wsConn.SetWriteDeadline(time.Now().Add(WS_WRITE_WAIT))
		w, err := c.wsConn.NextWriter(websocket.TextMessage)
		if err != nil {
			log.Infof("[PeerClient][%s] WritePump. write error=%s", c.Id, err)

			return
		}

		w.Write(message.Bytes())

		log.Infof("[PeerClient][%s] WritePump. write message=%s", c.Id, message)

		if err := w.Close(); err != nil {
			log.Infof("[PeerClient][%s] WritePump. write error=%s", c.Id, err)
			return
		}
	}
}

func (c *PeerClient) WriteClose() {
	c.write(websocket.CloseMessage, []byte{})
	c.wsConn.Close()
}

func (c *PeerClient) Wait() {

	go c.QueuePump()
	go c.ReadPump()
	go c.WritePump()

	<-c.close

	c.hub.RemovePeer(c.Id)

	log.Infof("[PeerClient][%s] Wiat. Close peer", c.Id)
	if c.closeQueue != nil {
		close(c.closeQueue)
	}
	if c.outMessages != nil {
		close(c.outMessages)
	}
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
		close: make(chan bool),
		closeQueue: make(chan bool),
	}
}
