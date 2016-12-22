package peerhub

import "encoding/json"

const (
	ERROR_WS = "No id, token, or key supplied to websocket server"
	ERROR_ID_TAKEN = "ID is taken"
	ERROR_KEY_TAKEN = "Invalid key provided"
	ERROR_MESSAGE_TAKEN = "Invalid message taken"

	MESSAGE_OFFER = "OFFER"
	MESSAGE_ANSWER = "ANSWER"
	MESSAGE_CANDIDATE = "CANDIDATE"
	MESSAGE_LEAVE = "LEAVE"
	MESSAGE_PEERS = "PEERS"
	MESSAGE_UPDATE = "UPDATE-PEER"
)


type Message struct {
	Type string `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Src string `json:"src,omitempty"`
	Dst string `json:"dst,omitempty"`
}

func (m *Message) String() string {
	bytes, _ := json.Marshal(m)
	return string(bytes)
}

func (m *Message) Bytes() []byte {
	bytes, _ := json.Marshal(m)
	return bytes
}

type MessageList []Message

func (ml *MessageList) String() string {
	bytes, _ := json.Marshal(ml)
	return string(bytes)
}

func NewMessage(src, dst, msgType string, playload interface{}) *Message {
	data, _ := json.Marshal(playload)
	return &Message{
		Type: msgType,
		Src: src,
		Dst: dst,
		Payload: json.RawMessage(data),
	}
}

func NewExpireMessage(src, dst string) *Message {
	return &Message{Type: "EXPIRE", Src: src, Dst: dst}
}

func NewErrorMessage(msg string) *Message {
	tmp := struct {
		Msg string `json:msg`
	} {
		Msg: msg,
	}
	tmpStr, _ := json.Marshal(tmp)

	return &Message{Type: "ERROR", Payload: json.RawMessage(tmpStr)}
}

func NewOpenMessage() *Message {
	return &Message{Type: "OPEN"}
}

func NewHttpErrorMessage() *Message {
	return &Message{Type: "HTTP-ERROR"}
}

func NewUpdateMessage() *Message {
	return &Message{Type: "UPDATE-PEER"}
}
