package main

import (
	"github.com/valyala/fasthttp"
	"gopeerjs-server/handlers"
	"gopeerjs-server/libs/logger"
	"gopeerjs-server/peerhub"
	"gopeerjs-server/libs/redispool"
	"regexp"
)

var (
	redisAddress = "0.0.0.0:6379"
	redisDB = 0
	redisPassword = ""

	address = ":8888"
	path = ""
	debug = true
)


func init() {
	//redis://user:secret@localhost:6379/0
}

func main() {
	log := logger.New()
	redis := redispool.NewPool(redisAddress, redisDB, redisPassword)

	peerHub := peerhub.NewHub(redis)

	log.Info("Start")


	reWs, _ := regexp.Compile("^" + path + "/peerjs" + "$")
	reId, _ := regexp.Compile("^" + path + "/([^/]+)/id" + "$");
	reAl, _ := regexp.Compile("^" + path + "/([^/]+)/([^/]+)/([^/]+)/([^/]+)" + "$");

	controllerHandler := func(ctx *fasthttp.RequestCtx) {
		var method = string(ctx.Method())
		var path = ctx.URI().Path()

		log.Infof("[%s] %s", method, path)

		if len(ctx.Request.Header.Peek("Origin")) > 0 {
			ctx.Response.Header.Add("Access-Control-Allow-Origin", "*")
		}

		if method == "OPTIONS" {
			ctx.Response.Header.Add("Access-Control-Allow-Method", "GET, POST")
			return
		}

		if method == "GET" && reWs.Match(path) {
			handlers.WebsocketHandler(ctx, peerHub, log, debug)
			return
		}
		if method == "GET" && reId.Match(path) {
			handlers.IdHandler(ctx, peerHub, log, debug)
			return
		}

		if method == "POST" {
			if m := reAl.FindSubmatch(path);  len(m) > 0 {
				f := string(m[4])
				ctx.SetUserValue("key", string(m[1]))
				ctx.SetUserValue("id", string(m[2]))
				ctx.SetUserValue("token", string(m[3]))

				switch f {
				case "id":
					handlers.StartHandler(ctx, peerHub, log, debug)
					return
				case "offer":
					handlers.CommonHandle(ctx, peerHub, log, debug, false)
					return
				case "candidate":
					handlers.CommonHandle(ctx, peerHub, log, debug, false)
					return
				case "answer":
					handlers.CommonHandle(ctx, peerHub, log, debug, false)
					return
				case "leave":
					handlers.CommonHandle(ctx, peerHub, log, debug, false)
					return
				}
			}
		}

		ctx.SetStatusCode(fasthttp.StatusNotFound);
	}

	log.Fatal(fasthttp.ListenAndServe(address, controllerHandler))
}
