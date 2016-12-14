package openws

import (
	"gopeerjs-server/config"
	"github.com/valyala/fasthttp"
	"github.com/clevergo/websocket"
	"gopeerjs-server/peerhub"
)

var (
	rePath = config.AppBindPath + "/peerjs"
	debug = config.Debug
	logger = config.Logger
	handlerName = "openws"

	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(ctx *fasthttp.RequestCtx) bool {
			return true
		},
	}
)

func write(conn *websocket.Conn, msg []byte) error {
	return conn.WriteMessage(websocket.TextMessage, msg)
}

func wrireClose(conn *websocket.Conn, msg []byte) {
	conn.WriteMessage(websocket.TextMessage, msg)
	conn.WriteMessage(websocket.CloseMessage, []byte{})
	conn.Close()
}

func Match(ctx *fasthttp.RequestCtx) bool {
	if ctx.IsGet() && string(ctx.URI().Path()) == rePath {
		if debug {
			logger.Infof("[%s] Match. %s %s", handlerName, ctx.Method(), ctx.URI())
		}
		return true
	}
	return false
}

func Handle(ctx *fasthttp.RequestCtx, hub *peerhub.PeerHub) {
	var key = string(ctx.FormValue("key"))
	var id = string(ctx.FormValue("id"))
	var token = string(ctx.FormValue("token"))
	var ip = ctx.RemoteAddr()

	if debug {
		logger.Infof("[%s] key=%s, id=%s, token=%s", handlerName, key, id, token)
	}

	upgrader.Upgrade(ctx, func(conn *websocket.Conn){
		if(id == "" || key == "" || token == "") {
			var msg = peerhub.NewErrorMessage(peerhub.ERROR_WS)

			if debug {
				logger.Errorf("[%s] Error=%s", handlerName, id, msg.String())
			}

			wrireClose(conn, msg.Bytes())
			return
		}
		if _, err := peerhub.CheckKey(key); err != nil {
			var msg = peerhub.NewErrorMessage(peerhub.ERROR_KEY_TAKEN)

			if debug {
				logger.Errorf("[%s] Error=%s", handlerName, msg.String())
			}

			wrireClose(conn, msg.Bytes())
			return
		}

		var client *peerhub.PeerClient

		if cl := hub.GetPeer(id); cl != nil {
			if debug {
				logger.Infof("[%s][%s] Load existed peer", handlerName, id)
			}

			client = cl
			client.SetConnection(conn)
		} else {
			if debug {
				logger.Infof("[%s][%s] Ceate new peer", handlerName, id)
			}

			client = peerhub.NewClient(id, key, token, ip, hub, conn)
		}



		if client.Token != token {
			var msg = peerhub.NewErrorMessage(peerhub.ERROR_ID_TAKEN)

			if debug {
				logger.Errorf("[%s][%s] Error=%s", handlerName, client.Id, msg.String())
			}

			wrireClose(conn, msg.Bytes())
			return
		}

		if client.Open() {
			if debug {
				logger.Infof("[%s][%s] Write OPEN", handlerName, client.Id)
			}

			var msg = peerhub.NewOpenMessage()
			if err := write(conn, msg.Bytes()); err != nil {
				if debug {
					logger.Errorf("[%s][%s] Error=%s", handlerName, client.Id, err)
				}
				conn.Close()
				return
			}
		}

		hub.AddPeer(client)
		err := client.Wait()
		if err != nil {
			logger.Errorf("[%s][%s] Error=%s", handlerName, client.Id, err)
		}
		if debug {
			logger.Info("[%s][%s] Close WS connection", handlerName, client.Id)
		}
	})
}