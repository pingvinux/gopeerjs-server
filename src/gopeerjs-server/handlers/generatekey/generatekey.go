package generatekey

import (
	"github.com/valyala/fasthttp"
	"gopeerjs-server/config"
	"regexp"
	"gopeerjs-server/peerhub"
)

var (
	rePath, _ = regexp.Compile("^" + config.AppBindPath + "/([^/]+)/key$")
	logger = config.Logger
	debug = config.Debug
	handlerName = "generatekey"
)

func Match(ctx *fasthttp.RequestCtx) bool {
	if ctx.IsGet() {
		if m := rePath.FindSubmatch(ctx.URI().Path()); len(m) > 0 {
			ctx.SetUserValue("appkey", string(m[1]))

			if debug {
				logger.Infof("[%s] Match. %s %s", handlerName, ctx.Method(), ctx.URI())
			}
			return true
		}
	}

	return false
}

func Handle(ctx *fasthttp.RequestCtx) {
	appKey, _ := ctx.UserValue("appkey").(string)
	fileKey := string(ctx.FormValue("fkey"))

	if debug {
		logger.Infof("[%s] appkey=%s fileKey=%s", handlerName, appKey, fileKey)
	}

	if !peerhub.CheckAppkey(appKey) {
		if debug {
			logger.Errorf("[%s] Invalid app key. %s", handlerName, appKey)
		}

		ctx.SetStatusCode(fasthttp.StatusUnauthorized)
		return
	}

	key := peerhub.NewKey(fileKey)
	token, err := peerhub.Key2Token(key)
	if err != nil {
		if debug {
			logger.Errorf("[%s] Generate token error=%s", handlerName, err)
		}

		ctx.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}

	if debug {
		logger.Infof("[%s] Generate key=%s", handlerName, token)
	}

	ctx.SetContentType("text/html")
	ctx.SetBodyString(token)
}