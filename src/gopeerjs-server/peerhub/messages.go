package peerhub

import "encoding/json"

const (
	ERROR_WS = "No id, token, or key supplied to websocket server"
	ERROR_ID_TAKEN = "ID is taken"
	ERROR_KEY_TAKEN = "Invalid key provided"
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

func NewOpenMessage() *Message {
	return &Message{Type: "OPEN"}
}

func NewHttpErrorMessage() *Message {
	return &Message{Type: "HTTP-ERROR"}
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