package handlers

import (
	"github.com/valyala/fasthttp"
	"github.com/Sirupsen/logrus"
	"fmt"
	"gopeerjs-server/peerhub"
	"strings"
	"encoding/json"
	"time"
)

func IdHandler(ctx *fasthttp.RequestCtx, hub *peerhub.PeerHub, log *logrus.Logger, debug bool) {
	var handlerName = "IdHandler"

	uuid := peerhub.GeneratePeerId()
	if(debug) {
		log.Infof("[%s] generate %s", handlerName, uuid)
	}

	ctx.SetContentType("text/html")
	ctx.SetBodyString(fmt.Sprintf("%s", uuid))
}

func StartHandler(ctx * fasthttp.RequestCtx, hub *peerhub.PeerHub, logger *logrus.Logger, debug bool) {
	var handlerName = "StartHandler"

	var id, _ = ctx.UserValue("id").(string)
	var key, _ = ctx.UserValue("key").(string)
	var token, _ = ctx.UserValue("token").(string)
	var ip = ctx.RemoteAddr()

	logger.Infof("[%s] id=%s, key=%s, token=%s", handlerName, id, key, token)

	ctx.Response.Header.Set("Content-type", "application/octet-stream")
	ctx.Response.SetStatusCode(fasthttp.StatusOK)

	_, err := peerhub.CheckKey(key)
	if err != nil {
		ctx.SetBodyString(peerhub.NewHttpErrorMessage().String())
		return
	}

	var client *peerhub.PeerClient

	if cl := hub.GetPeer(id); cl != nil {
		logger.Infof("[%s] load existed peer", handlerName)

		client = cl
	} else {
		logger.Infof("[%s] create new peer", handlerName)

		client = peerhub.NewClient(id, key, token, ip, hub, nil)
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
				logger.Infof("[%s] remove peer. peer=%+v", client)
				hub.RemovePeer(client.Id)
			}
		}()
	} else {
		body += peerhub.NewHttpErrorMessage().String()
	}

	logger.Infof("[%s] send message. message=%s", handlerName, body)

	ctx.SetBodyString(body)
}

func CommonHandle(ctx * fasthttp.RequestCtx, hub *peerhub.PeerHub, logger *logrus.Logger, debug bool, retry bool) {
	var handlerName = "CommonHandle"

	var key, _ = ctx.UserValue("key").(string)
	var id = ctx.UserValue("id").(string)
	var token = ctx.UserValue("token").(string)
	var body = ctx.PostBody()

	logger.Infof("[%s] id=%s, key=%s, token=%s", handlerName, id, key, token)

	var message peerhub.Message
	if err := json.Unmarshal(body, &message); err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
	}

	if _, err := peerhub.CheckKey(key); err != nil {
		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}

	hub.TransmitMessage(&message)

	ctx.SetStatusCode(fasthttp.StatusOK)
}
