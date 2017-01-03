package open

import (
	"gopeerjs-server/peerhub"
	"strings"
	"time"
	"github.com/valyala/fasthttp"
	"gopeerjs-server/config"
	"regexp"
)

var (
	rePath, _ = regexp.Compile("^" + config.AppBindPath + "/([^/]+)/([^/]+)/([^/]+)/id$")
	logger = config.Logger
	debug = config.Debug
	handlerName = "open"
)

func Match(ctx *fasthttp.RequestCtx) bool {
	if ctx.IsPost() {
		if m := rePath.FindSubmatch(ctx.URI().Path()); len(m) > 0 {
			ctx.SetUserValue("key", string(m[1]))
			ctx.SetUserValue("id", string(m[2]))
			ctx.SetUserValue("token", string(m[3]))

			if debug {
				logger.Infof("[%s] Match. %s %s", handlerName, ctx.Method(), ctx.URI())
			}
			return true
		}
	}
	return false
}

func Handle(ctx * fasthttp.RequestCtx, hub *peerhub.PeerHub) {
	var id, _ = ctx.UserValue("id").(string)
	var key, _ = ctx.UserValue("key").(string)
	var token, _ = ctx.UserValue("token").(string)
	var ip = ctx.RemoteAddr()

	if debug {
		logger.Infof("[%s] id=%s, key=%s, token=%s", handlerName, id, key, token)
	}

	ctx.Response.Header.Set("Content-type", "application/octet-stream")
	ctx.Response.SetStatusCode(fasthttp.StatusOK)

	peerKey, err := peerhub.Token2Key(key)
	if err != nil {
		if debug {
			logger.Errorf("[%s] %s", handlerName, err)
		}

		ctx.SetBodyString(peerhub.NewHttpErrorMessage().String())
		return
	}

	var client *peerhub.PeerClient

	if cl := hub.GetPeer(id); cl != nil {
		if debug {
			logger.Infof("[%s][%s] Load existed peer", handlerName, cl.Id)
		}

		client = cl
	} else {
		if debug {
			logger.Infof("[%s][%s] Create new peer", handlerName, id)
		}

		client = peerhub.NewClient(id, peerKey.Key, token, ip, hub, nil)
		hub.AddPeer(client)
	}

	var body = strings.Repeat("00", 9) + "\n"
	if client.Open() {
		body += peerhub.NewOpenMessage().String() + "\n"
	}

	if client.Token == token {
		defer func(){
			time.Sleep(25*time.Millisecond)
			if !client.IsConnected() {
				if debug {
					logger.Infof("[%s][%s] Remove peer. Not connected to WebSocket", handlerName, client.Id)
				}

				hub.RemovePeer(client)
			}
		}()
	} else {
		body += peerhub.NewHttpErrorMessage().String()
	}

	if debug {
		logger.Infof("[%s][%s] Send message. message=%s", handlerName, client.Id, body)
	}

	ctx.SetBodyString(body)
}
