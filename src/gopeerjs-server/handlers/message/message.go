package message

import (
	"regexp"
	"gopeerjs-server/config"
	"github.com/valyala/fasthttp"
	"bytes"
	"gopeerjs-server/peerhub"
	"encoding/json"
)

var (
	rePath, reErr = regexp.Compile("^" + config.AppBindPath + "/([^/]+)/([^/]+)/([^/]+)/(offer|answer|candidate|leave)$")
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
	if ctx.IsPost() {
		if m := rePath.FindSubmatch(ctx.URI().Path()); len(m) > 0 {
			ctx.SetUserValue("key", string(m[1]))
			ctx.SetUserValue("id", string(m[2]))
			ctx.SetUserValue("token", string(m[3]))
			ctx.SetUserValue("type", string(bytes.ToUpper(m[4])))

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
	var body = ctx.PostBody()

	if debug {
		logger.Infof("[%s] id=%s, key=%s, token=%s", handlerName, id, key, token)
	}

	ctx.SetStatusCode(fasthttp.StatusOK)

	if _, err := peerhub.CheckKey(key); err != nil {
		if debug {
			logger.Errorf("[%s][%s] Error=%s", handlerName, id, err)
		}

		var msg = peerhub.NewHttpErrorMessage()
		ctx.SetBody(msg.Bytes())
		return
	}

	var message peerhub.Message
	if err := json.Unmarshal(body, &message); err != nil {
		if debug {
			logger.Errorf("[%s][%s] Error=%s", handlerName, id, err)
		}

		var msg = peerhub.NewErrorMessage(peerhub.ERROR_MESSAGE_TAKEN)
		ctx.SetBody(msg.Bytes())
		return
	}

	if (message.Type != peerhub.MESSAGE_OFFER &&
		message.Type != peerhub.MESSAGE_ANSWER &&
		message.Type != peerhub.MESSAGE_CANDIDATE &&
		message.Type != peerhub.MESSAGE_LEAVE) {

		if debug {
			logger.Errorf("[%s][%s] Error=%s", handlerName, id, "Invalid message type")
		}

		var msg = peerhub.NewErrorMessage(peerhub.ERROR_MESSAGE_TAKEN)
		ctx.SetBody(msg.Bytes())
		return
	}


	hub.TransmitMessage(&message)
}
