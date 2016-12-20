package main

import (
	"github.com/valyala/fasthttp"
	"github.com/streadway/amqp"

	"gopeerjs-server/handlers/generateid"
	"gopeerjs-server/handlers/open"
	"gopeerjs-server/handlers/openws"
	"gopeerjs-server/handlers/message"
	"gopeerjs-server/peerhub"
	"log"
	"github.com/garyburd/redigo/redis"
	"time"
	"gopeerjs-server/config"
	"gopeerjs-server/handlers/generatekey"
)


func main() {
	redisPool := &redis.Pool{
		MaxIdle: 3,
		IdleTimeout: 240 * time.Second,
		Dial: func () (redis.Conn, error) {
			c, err := redis.DialURL(config.AppRedisUrl)
			if err != nil {
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}

	amqpConn, err := amqp.Dial(config.AppAmqpUrl)
	if err != nil {
		log.Fatal(err)
	}

	peerHub := peerhub.NewHub(redisPool, amqpConn)
	go peerHub.Run()

	controllerHandler := func(ctx *fasthttp.RequestCtx) {
		if len(ctx.Request.Header.Peek("Origin")) > 0 {
			ctx.Response.Header.Add("Access-Control-Allow-Origin", "*")
		}
		if string(ctx.Method()) == "OPTIONS" {
			ctx.Response.Header.Add("Access-Control-Allow-Method", "GET, POST")
			return
		}

		switch {
		case generateid.Match(ctx):
			generateid.Handle(ctx)
		case generatekey.Match(ctx):
			generatekey.Handle(ctx)
		case open.Match(ctx):
			open.Handle(ctx, peerHub)
		case openws.Match(ctx):
			openws.Handle(ctx, peerHub)
		case message.Match(ctx):
			message.Handle(ctx, peerHub)
		default:
			if config.Debug {
				config.Logger.Infof("[NotFound] %s %s", ctx.Method(), ctx.URI().Path())
			}

			ctx.SetStatusCode(fasthttp.StatusNotFound)
		}
	}

	log.Fatal(fasthttp.ListenAndServe(config.AppBindAddress, controllerHandler))


}
