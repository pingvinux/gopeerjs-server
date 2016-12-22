package peers

import (
	"regexp"
	"gopeerjs-server/config"
	"github.com/valyala/fasthttp"
	"gopeerjs-server/peerhub"
	"encoding/json"
)

var (
	rePath, reErr = regexp.Compile("^" + config.AppBindPath + "/([^/]+)/([^/]+)/([^/]+)/peers$")
	logger = config.Logger
	debug = config.Debug
	handlerName = "message"
)

func init() {
	if reErr != nil {
		panic(reErr)
	}
}

func Match(ctx *fasthttp.RequestCtx) bool {
	if ctx.IsGet() {
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
	var key, _ = ctx.UserValue("key").(string)
	var id = ctx.UserValue("id").(string)
	var token = ctx.UserValue("token").(string)

	if debug {
		logger.Infof("[%s] id=%s, key=%s, token=%s", handlerName, id, key, token)
	}

	ctx.SetContentType("text/html")
	ctx.SetStatusCode(fasthttp.StatusOK)

	peerKey, err := peerhub.Token2Key(key)
	if err != nil {
		if debug {
			logger.Errorf("[%s][%s] Error=%s", handlerName, id, err)
		}

		var msg = peerhub.NewHttpErrorMessage()
		ctx.SetBody(msg.Bytes())
		return
	}

	peers := make([]string, 0)
	if pl, err := hub.GetAllPeer(peerKey.Key); err == nil {
		for peerId := range pl {
			if peerId != id {
				peers = append(peers, peerId)
			}
		}
	} else {
		logger.Errorf("[%s][%s] Error=%", handlerName, id, err)
	}

	data, _ := json.Marshal(peers)
	ctx.SetBody(data)
}
