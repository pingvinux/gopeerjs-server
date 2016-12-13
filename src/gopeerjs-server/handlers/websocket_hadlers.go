package handlers

import (
	"github.com/valyala/fasthttp"
	"github.com/Sirupsen/logrus"
	"github.com/clevergo/websocket"
	"gopeerjs-server/peerhub"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
		return true
	},
}



func WebsocketHandler(ctx *fasthttp.RequestCtx, hub *peerhub.PeerHub, logger *logrus.Logger, debug bool) {
	var handlerName = "WebsocketHandler"

	var id = string(ctx.FormValue("id"))
	var key = string(ctx.FormValue("key"))
	var token = string(ctx.FormValue("token"))
	var ip = ctx.RemoteAddr()

	if _, err := peerhub.CheckKey(key); err != nil {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}

	logger.Infof("[%s]", handlerName)

	upgrader.Upgrade(ctx, func(conn *websocket.Conn){
		var client *peerhub.PeerClient

		logger.Infof("[%s] Get peer. peer_id=%s", handlerName, id)
		if cl := hub.GetPeer(id); cl != nil {
			logger.Infof("[%s] load existed peer", handlerName)

			client = cl
			client.SetConnection(conn)
		} else {
			logger.Infof("[%s] ceate new peer", handlerName)

			client = peerhub.NewClient(id, key, token, ip, hub, conn)
		}

		if(id == "" || key == "" || token == "") {
			var msg = peerhub.NewErrorMessage(peerhub.ERROR_WS)

			logger.Infof("[%s] Write OPEN. error=%s", handlerName, msg.String())

			client.Write(msg.Bytes())
			client.WriteClose()
			return
		}

		if client.Token != token {
			var msg = peerhub.NewErrorMessage(peerhub.ERROR_ID_TAKEN)

			logger.Infof("[%s] Write OPEN. error=%s", handlerName, msg.String())

			client.Write(msg.Bytes())
			client.WriteClose()
			return
		}

		if client.Open() {
			logger.Infof("[%s] Write OPEN", handlerName)
			if err := client.Write(peerhub.NewOpenMessage().Bytes()); err != nil {
				logger.Infof("[%s] Write OPEN. error=%s", handlerName, err)
				return
			}
		}

		hub.AddPeer(client)

		client.Wait()
	})
}
