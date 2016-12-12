package peerhub

import (
	"time"
	"github.com/clevergo/websocket"
	"net"
	"bytes"
	"encoding/json"
	"sync"
)

var (
	newline = []byte("\n")
	space = []byte(" ")
)

type PeerClient struct {
	sync.Mutex

	Id    string
	Key   string
	Token string
	IP    net.Addr

	isOpen bool
	hub   *PeerHub
	conn  *websocket.Conn
	send  chan []byte
	close chan bool
}

func (c *PeerClient) SetConnection(conn *websocket.Conn) {
	c.Lock()
	defer c.Unlock()

	c.conn = conn
}

func (c *PeerClient) IsConnected() bool {
	c.Lock()
	defer c.Unlock()

	var r = false
	if c.conn != nil {
		r = true
	}
	return r
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

func (c *PeerClient) ReadPump() {
	defer func() {
		c.hub.RemovePeer(c.Id)
		c.conn.Close()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				log.Printf("error: %v", err)
			}

			log.Infof("[PeerClient] ReadPump. peer_id=%s. read error=%s", c.Id, err)

			// You should send the closed message to the close channel,
			// otherwise, the server will be crashed.
			c.close <- true
			return
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		log.Infof("[PeerClient] ReadPump. peer_id=%s. raw_message=%s", c.Id, message)

		var clientMessage Message
		if err := json.Unmarshal(message, &clientMessage); err != nil {
			log.Infof("[PeerClient] ReadPump. peer_id=%s. unmarshal error=%s", c.Id, err)

			// Invalid message
			continue
		}

		log.Infof("[PeerClient] ReadPump. peer_id=%s. message=%+v", c.Id, clientMessage)

		switch clientMessage.Type {
		case "LEAVE":
			fallthrough
		case "CANDIDATE":
			fallthrough
		case "OFFER":
			fallthrough
		case "ANSWER":
			mess := &Message{
				Type: clientMessage.Type,
				Src: c.Id,
				Dst: clientMessage.Dst,
				Payload: clientMessage.Payload,
			}
			c.hub.TransmitMessage(mess)
		default:
			// Error Message unrecognized
		}

	}
}

func (c *PeerClient) write(mt int, payload []byte) error {
	c.conn.SetWriteDeadline(time.Now().Add(WS_WRITE_WAIT))
	return c.conn.WriteMessage(mt, payload)
}

func (c *PeerClient) Write(playload []byte) error {
	return c.write(websocket.TextMessage, playload)
}

func (c *PeerClient) WritePump() {
	for {
		select {
		case close := <-c.close:
			log.Infof("[PeerClient] WritePump. peer_id=%s. close", c.Id)
			if(close) {
				return
			}
		case message, ok := <-c.send:
			if !ok {
				// The hub closed the channel.
				c.write(websocket.CloseMessage, []byte{})
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(WS_WRITE_WAIT))
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Infof("[PeerClient] WritePump. peer_id=%s. write error=%s", c.Id, err)

				return
			}
			w.Write(message)

			log.Infof("[PeerClient] WritePump. peer_id=%s. write message=%s", c.Id, message)

			messages, _ := c.hub.GetLostMessage(c.Id)
			if len(messages) > 0 {
				for _, mess := range messages {
					w.Write(newline)
					w.Write(mess.Bytes())

					log.Infof("[PeerClient] WritePump. peer_id=%s. write message=%s", c.Id, mess.Bytes())
				}
			}

			if err := w.Close(); err != nil {
				log.Infof("[PeerClient] WritePump. peer_id=%s. write error=%s", c.Id, err)

				return
			}
		}
	}
}

func (c *PeerClient) WriteClose() {
	c.write(websocket.CloseMessage, []byte{})
	c.conn.Close()
}

func NewClient(id string, key string, token string, ip net.Addr, hub *PeerHub, conn *websocket.Conn) *PeerClient {
	return &PeerClient{
		Id: id,
		Key: key,
		Token: token,
		IP: ip,
		hub: hub,
		conn: conn,
		send: make(chan []byte, 0),
		close: make(chan bool, 0),
	}
}
